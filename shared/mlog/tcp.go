// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mlog

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	_ "net/http/pprof" // Enable pprof metrics
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/mattermost/logr"
)

const (
	DialTimeoutSecs             = 30
	WriteTimeoutSecs            = 30
	RetryBackoffMillis    int64 = 100
	MaxRetryBackoffMillis int64 = 30 * 1000 // 30 seconds
)

// Tcp outputs log records to raw socket server.
type TCP struct {
	logr.Basic

	params *TCPParams
	addy   string

	mutex    sync.Mutex
	conn     net.Conn
	monitor  chan struct{}
	shutdown chan struct{}
}

// TcpParams provides parameters for dialing a socket server.
type TCPParams struct {
	IP       string `json:"IP"`
	Port     int    `json:"Port"`
	TLS      bool   `json:"TLS"`
	Cert     string `json:"Cert"`
	Insecure bool   `json:"Insecure"`
}

// NewTcpTarget creates a target capable of outputting log records to a raw socket, with or without TLS.
func NewTCPTarget(filter logr.Filter, formatter logr.Formatter, params *TCPParams, maxQueue int) (*TCP, error) {
	tcp := &TCP{
		params:   params,
		addy:     fmt.Sprintf("%s:%d", params.IP, params.Port),
		monitor:  make(chan struct{}),
		shutdown: make(chan struct{}),
	}
	tcp.Basic.Start(tcp, tcp, filter, formatter, maxQueue)

	return tcp, nil
}

// getConn provides a net.Conn.  If a connection already exists, it is returned immediately,
// otherwise this method blocks until a new connection is created, timeout or shutdown.
func (tcp *TCP) getConn() (net.Conn, error) {
	tcp.mutex.Lock()
	defer tcp.mutex.Unlock()

	Log(LvlTCPLogTarget, "getConn enter", String("addy", tcp.addy))
	defer Log(LvlTCPLogTarget, "getConn exit", String("addy", tcp.addy))

	if tcp.conn != nil {
		Log(LvlTCPLogTarget, "reusing existing conn", String("addy", tcp.addy)) // use "With" once Zap is removed
		return tcp.conn, nil
	}

	type result struct {
		conn net.Conn
		err  error
	}

	connChan := make(chan result)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*DialTimeoutSecs)
	defer cancel()

	go func(ctx context.Context, ch chan result) {
		Log(LvlTCPLogTarget, "dailing", String("addy", tcp.addy))
		conn, err := tcp.dial(ctx)
		if err == nil {
			tcp.conn = conn
			tcp.monitor = make(chan struct{})
			go monitor(tcp.conn, tcp.monitor, Log)
		}
		ch <- result{conn: conn, err: err}
	}(ctx, connChan)

	select {
	case <-tcp.shutdown:
		return nil, errors.New("shutdown")
	case res := <-connChan:
		return res.conn, res.err
	}
}

// dial connects to a TCP socket, and optionally performs a TLS handshake.
// A non-nil context must be provided which can cancel the dial.
func (tcp *TCP) dial(ctx context.Context) (net.Conn, error) {
	var dialer net.Dialer
	dialer.Timeout = time.Second * DialTimeoutSecs
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", tcp.params.IP, tcp.params.Port))
	if err != nil {
		return nil, err
	}

	if !tcp.params.TLS {
		return conn, nil
	}

	Log(LvlTCPLogTarget, "TLS handshake", String("addy", tcp.addy))

	tlsconfig := &tls.Config{
		ServerName:         tcp.params.IP,
		InsecureSkipVerify: tcp.params.Insecure,
	}
	if tcp.params.Cert != "" {
		pool, err := getCertPool(tcp.params.Cert)
		if err != nil {
			return nil, err
		}
		tlsconfig.RootCAs = pool
	}

	tlsConn := tls.Client(conn, tlsconfig)
	if err := tlsConn.Handshake(); err != nil {
		return nil, err
	}
	return tlsConn, nil
}

func (tcp *TCP) close() error {
	tcp.mutex.Lock()
	defer tcp.mutex.Unlock()

	var err error
	if tcp.conn != nil {
		Log(LvlTCPLogTarget, "closing connection", String("addy", tcp.addy))
		close(tcp.monitor)
		err = tcp.conn.Close()
		tcp.conn = nil
	}
	return err
}

// Shutdown stops processing log records after making best effort to flush queue.
func (tcp *TCP) Shutdown(ctx context.Context) error {
	errs := &multierror.Error{}

	Log(LvlTCPLogTarget, "shutting down", String("addy", tcp.addy))

	if err := tcp.Basic.Shutdown(ctx); err != nil {
		errs = multierror.Append(errs, err)
	}

	if err := tcp.close(); err != nil {
		errs = multierror.Append(errs, err)
	}

	close(tcp.shutdown)
	return errs.ErrorOrNil()
}

// Write converts the log record to bytes, via the Formatter, and outputs to the socket.
// Called by dedicated target goroutine and will block until success or shutdown.
func (tcp *TCP) Write(rec *logr.LogRec) error {
	_, stacktrace := tcp.IsLevelEnabled(rec.Level())

	buf := rec.Logger().Logr().BorrowBuffer()
	defer rec.Logger().Logr().ReleaseBuffer(buf)

	buf, err := tcp.Formatter().Format(rec, stacktrace, buf)
	if err != nil {
		return err
	}

	try := 1
	backoff := RetryBackoffMillis
	for {
		select {
		case <-tcp.shutdown:
			return err
		default:
		}

		conn, err := tcp.getConn()
		if err != nil {
			Log(LvlTCPLogTarget, "failed getting connection", String("addy", tcp.addy), Err(err))
			reporter := rec.Logger().Logr().ReportError
			reporter(fmt.Errorf("log target %s connection error: %w", tcp.String(), err))
			backoff = tcp.sleep(backoff)
			continue
		}

		conn.SetWriteDeadline(time.Now().Add(time.Second * WriteTimeoutSecs))
		_, err = buf.WriteTo(conn)
		if err == nil {
			return nil
		}

		Log(LvlTCPLogTarget, "write error", String("addy", tcp.addy), Err(err))
		reporter := rec.Logger().Logr().ReportError
		reporter(fmt.Errorf("log target %s write error: %w", tcp.String(), err))

		_ = tcp.close()

		backoff = tcp.sleep(backoff)
		try++
		Log(LvlTCPLogTarget, "retrying write", String("addy", tcp.addy), Int("try", try))
	}
}

// monitor continuously tries to read from the connection to detect socket close.
// This is needed because TCP target uses a write only socket and Linux systems
// take a long time to detect a loss of connectivity on a socket when only writing;
// the writes simply fail without an error returned.
func monitor(conn net.Conn, done <-chan struct{}, logFunc LogFuncCustom) {
	addy := conn.RemoteAddr().String()
	defer logFunc(LvlTCPLogTarget, "monitor exiting", String("addy", addy))

	buf := make([]byte, 1)
	for {
		logFunc(LvlTCPLogTarget, "monitor loop", String("addy", addy))

		select {
		case <-done:
			return
		case <-time.After(1 * time.Second):
		}

		err := conn.SetReadDeadline(time.Now().Add(time.Second * 30))
		if err != nil {
			continue
		}

		_, err = conn.Read(buf)

		if errt, ok := err.(net.Error); ok && errt.Timeout() {
			// read timeout is expected, keep looping.
			continue
		}

		// Any other error closes the connection, forcing a reconnect.
		logFunc(LvlTCPLogTarget, "monitor closing connection", Err(err))
		conn.Close()
		return
	}
}

// String returns a string representation of this target.
func (tcp *TCP) String() string {
	return fmt.Sprintf("TcpTarget[%s:%d]", tcp.params.IP, tcp.params.Port)
}

func (tcp *TCP) sleep(backoff int64) int64 {
	select {
	case <-tcp.shutdown:
	case <-time.After(time.Millisecond * time.Duration(backoff)):
	}

	nextBackoff := backoff + (backoff >> 1)
	if nextBackoff > MaxRetryBackoffMillis {
		nextBackoff = MaxRetryBackoffMillis
	}
	return nextBackoff
}
