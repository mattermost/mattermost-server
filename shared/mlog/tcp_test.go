// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package mlog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wiggin77/merror"
)

const (
	testPort = 18066
)

func TestNewTCPTarget(t *testing.T) {
	target := LogTarget{
		Type:         "tcp",
		Format:       "json",
		Levels:       []LogLevel{LvlInfo},
		Options:      []byte(`{"IP": "localhost", "Port": 18066}`),
		MaxQueueSize: 1000,
	}
	targets := map[string]*LogTarget{"tcp_test": &target}

	t.Run("logging", func(t *testing.T) {
		buf := &buffer{}
		server, err := newSocketServer(testPort, buf)
		require.NoError(t, err)

		data := []string{"I drink your milkshake!", "We don't need no badges!", "You can't fight in here! This is the war room!"}

		logger := newLogr()
		err = logrAddTargets(logger, targets)
		require.NoError(t, err)

		for _, s := range data {
			logger.Info(s)
		}
		err = logger.Logr().Flush()
		require.NoError(t, err)
		err = logger.Logr().Shutdown()
		require.NoError(t, err)

		err = server.waitForAnyConnection()
		require.NoError(t, err)

		err = server.stopServer(true)
		require.NoError(t, err)

		sdata := buf.String()
		for _, s := range data {
			require.Contains(t, sdata, s)
		}
	})
}

// socketServer is a simple socket server used for testing TCP log targets.
// Note: There is more synchronization here than normally needed to avoid flaky tests.
//       For example, it's possible for a unit test to create a socketServer, attempt
//       writing to it, and stop the socket server before "go ss.listen()" gets scheduled.
type socketServer struct {
	listener net.Listener
	anyConn  chan struct{}
	buf      *buffer
	conns    map[string]*socketServerConn
	mux      sync.Mutex
}

type socketServerConn struct {
	raddy string
	conn  net.Conn
	done  chan struct{}
}

func newSocketServer(port int, buf *buffer) (*socketServer, error) {
	ss := &socketServer{
		buf:     buf,
		conns:   make(map[string]*socketServerConn),
		anyConn: make(chan struct{}),
	}

	addy := fmt.Sprintf(":%d", port)
	l, err := net.Listen("tcp4", addy)
	if err != nil {
		return nil, err
	}
	ss.listener = l

	go ss.listen()
	return ss, nil
}

func (ss *socketServer) listen() {
	for {
		conn, err := ss.listener.Accept()
		if err != nil {
			return
		}
		sconn := &socketServerConn{raddy: conn.RemoteAddr().String(), conn: conn, done: make(chan struct{})}
		ss.registerConnection(sconn)
		go ss.handleConnection(sconn)
	}
}

func (ss *socketServer) waitForAnyConnection() error {
	var err error
	select {
	case <-ss.anyConn:
	case <-time.After(5 * time.Second):
		err = errors.New("wait for any connection timed out")
	}
	return err
}

func (ss *socketServer) handleConnection(sconn *socketServerConn) {
	close(ss.anyConn)
	defer ss.unregisterConnection(sconn)
	buf := make([]byte, 1024)

	for {
		n, err := sconn.conn.Read(buf)
		if n > 0 {
			ss.buf.Write(buf[:n])
		}
		if err == io.EOF {
			ss.signalDone(sconn)
			return
		}
	}
}

func (ss *socketServer) registerConnection(sconn *socketServerConn) {
	ss.mux.Lock()
	defer ss.mux.Unlock()
	ss.conns[sconn.raddy] = sconn
}

func (ss *socketServer) unregisterConnection(sconn *socketServerConn) {
	ss.mux.Lock()
	defer ss.mux.Unlock()
	delete(ss.conns, sconn.raddy)
}

func (ss *socketServer) signalDone(sconn *socketServerConn) {
	ss.mux.Lock()
	defer ss.mux.Unlock()
	close(sconn.done)
}

func (ss *socketServer) stopServer(wait bool) error {
	errs := merror.New()
	ss.listener.Close()

	ss.mux.Lock()
	// defensive copy; no more connections can be accepted so copy will stay current.
	conns := make(map[string]*socketServerConn, len(ss.conns))
	for k, v := range ss.conns {
		conns[k] = v
	}
	ss.mux.Unlock()

	for _, sconn := range conns {
		if wait {
			select {
			case <-sconn.done:
			case <-time.After(time.Second * 5):
				errs.Append(errors.New("timed out"))
			}
		}
	}
	return errs.ErrorOrNil()
}

type buffer struct {
	buf bytes.Buffer
	mux sync.Mutex
}

func (b *buffer) Write(p []byte) (n int, err error) {
	b.mux.Lock()
	defer b.mux.Unlock()
	return b.buf.Write(p)
}

func (b *buffer) String() string {
	b.mux.Lock()
	defer b.mux.Unlock()
	return b.buf.String()
}
