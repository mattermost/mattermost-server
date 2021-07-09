// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package upgrader

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/crypto/openpgp"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

const mattermostBuildPublicKey = `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQENBFjZQxwBCAC6kNn3zDlq/aY83M9V7MHVPoK2jnZ3BfH7sA+ibQXsijCkPSR4
5bCUJ9qVA4XKGK+cpO9vkolSNs10igCaaemaUZNB6ksu3gT737/SZcCAfRO+cLX7
Q2la+jwTvu1YeT/M5xDZ1KHTFxsGskeIenz2rZHeuZwBl9qep34QszWtRX40eRts
fl6WltLrepiExTp6NMZ50k+Em4JGM6CWBMo22ucy0jYjZXO5hEGb3o6NGiG+Dx2z
b2J78LksCKGsSrn0F1rLJeA933bFL4g9ozv9asBlzmpgG77ESg6YE1N/Rh7WDzVA
prIR0MuB5JjElASw5LDVxDV6RZsxEVQr7ETLABEBAAG0KU1hdHRlcm1vc3QgQnVp
bGQgPGRldi1vcHNAbWF0dGVybW9zdC5jb20+iQFUBBMBCAA+AhsDBQsJCAcCBhUI
CQoLAgQWAgMBAh4BAheAFiEEobMdRvDzoQsCzy1E+PLDF0R3SygFAl6HYr0FCQlw
hqEACgkQ+PLDF0R3SyheNQgAnkiT2vFMCtU5FmC16HVYXzDpYMtdCQPh/gmeEkiI
80rFRg/cn6f0BNnaTfDu6r6cepmhLNpDAowjQ7uBnv8fL2dzCydIGFv2r7FfmcOJ
zhEQ3zXPwP6mYlxPCCgxAozsLv9Yv41KGCHIlzYwkAazc0BhpAW/h8L3VGkE+b+g
x6lKVoufm4rKnT49Dgly6fVOxuR/BqZo87B5jksV3izLTHt5hiY8Pc5GW8WwO/tr
pNAw+6HRXq1Dr/JRz5PIOr5KP5tVLBed4IteZ1xaTRd4++07ZbiZjhXY8WKpVp3y
iN7Om24jQpxbJI9+KKJ3+yhcwhr8/PJ8ZVuhJo3BNv1PcQ==
=9Qk8
-----END PGP PUBLIC KEY BLOCK-----`

var upgradePercentage int64
var upgradeError error
var upgrading int32

type writeCounter struct {
	total  int64
	readed int64
}

func (wc *writeCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.readed += int64(n)
	percentage := (wc.readed * 100) / wc.total
	if percentage == 0 {
		upgradePercentage = 1
	} else if percentage == 100 {
		upgradePercentage = 99
	} else {
		upgradePercentage = percentage
	}
	return n, nil
}

func getCurrentVersionTgzURL() string {
	version := model.CurrentVersion
	if strings.HasPrefix(model.BuildNumber, version+"-rc") {
		version = model.BuildNumber
	}

	return "https://releases.mattermost.com/" + version + "/mattermost-" + version + "-linux-amd64.tar.gz"
}

func verifySignature(filename string, sigfilename string, publicKey string) error {
	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader([]byte(publicKey)))
	if err != nil {
		mlog.Debug("Unable to load the public key to verify the file signature", mlog.Err(err))
		return NewInvalidSignature()
	}

	mattermost_tar, err := os.Open(filename)
	if err != nil {
		mlog.Debug("Unable to open the Mattermost .tar file to verify the file signature", mlog.Err(err))
		return NewInvalidSignature()
	}

	signature, err := os.Open(sigfilename)
	if err != nil {
		mlog.Debug("Unable to open the Mattermost .sig file verify the file signature", mlog.Err(err))
		return NewInvalidSignature()
	}

	_, err = openpgp.CheckDetachedSignature(keyring, mattermost_tar, signature)
	if err != nil {
		mlog.Debug("Unable to verify the Mattermost file signature", mlog.Err(err))
		return NewInvalidSignature()
	}
	return nil
}

func canIWriteTheExecutable() error {
	executablePath, err := os.Executable()
	if err != nil {
		return errors.New("error getting the path of the executable")
	}
	executableInfo, err := os.Stat(path.Dir(executablePath))
	if err != nil {
		return errors.New("error getting the executable info")
	}
	stat, ok := executableInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("error getting the executable info")
	}
	fileUID := int(stat.Uid)
	fileUser, err := user.LookupId(strconv.Itoa(fileUID))
	if err != nil {
		return errors.New("error getting the executable info")
	}

	mattermostUID := os.Getuid()
	mattermostUser, err := user.LookupId(strconv.Itoa(mattermostUID))
	if err != nil {
		return errors.New("error getting the executable info")
	}

	mode := executableInfo.Mode()
	if fileUID != mattermostUID && mode&(1<<1) == 0 && mode&(1<<7) == 0 {
		return NewInvalidPermissions("invalid-user-and-permission", path.Dir(executablePath), mattermostUser.Username, fileUser.Username)
	}

	if fileUID != mattermostUID && mode&(1<<1) == 0 && mode&(1<<7) != 0 {
		return NewInvalidPermissions("invalid-user", path.Dir(executablePath), mattermostUser.Username, fileUser.Username)
	}

	if fileUID == mattermostUID && mode&(1<<7) == 0 {
		return NewInvalidPermissions("invalid-permission", path.Dir(executablePath), mattermostUser.Username, fileUser.Username)
	}
	return nil
}

func canIUpgrade() error {
	if runtime.GOARCH != "amd64" {
		return NewInvalidArch()
	}
	if runtime.GOOS != "linux" {
		return NewInvalidArch()
	}
	return canIWriteTheExecutable()
}

func CanIUpgradeToE0() error {
	if err := canIUpgrade(); err != nil {
		return errors.Wrap(err, "unable to upgrade from TE to E0")
	}
	if model.BuildEnterpriseReady == "true" {
		mlog.Warn("Unable to upgrade from TE to E0. The server is already running E0.")
		return errors.New("you cannot upgrade your server from TE to E0 because you are already running Mattermost Enterprise Edition")
	}
	return nil
}

func UpgradeToE0() error {
	if !atomic.CompareAndSwapInt32(&upgrading, 0, 1) {
		mlog.Warn("Trying to upgrade while another upgrade is running")
		return errors.New("another upgrade is already running")
	}
	defer atomic.CompareAndSwapInt32(&upgrading, 1, 0)

	upgradePercentage = 1
	upgradeError = nil

	executablePath, err := os.Executable()
	if err != nil {
		upgradePercentage = 0
		upgradeError = errors.New("error getting the executable path")
		mlog.Error("Unable to get the path of the Mattermost executable", mlog.Err(err))
		return err
	}

	filename, err := download(getCurrentVersionTgzURL(), 1024*1024*300)
	if err != nil {
		if filename != "" {
			os.Remove(filename)
		}
		upgradeError = fmt.Errorf("error downloading the new Mattermost server binary file (percentage: %d)", upgradePercentage)
		mlog.Error("Unable to download the Mattermost server binary file", mlog.Int64("percentage", upgradePercentage), mlog.String("url", getCurrentVersionTgzURL()), mlog.Err(err))
		upgradePercentage = 0
		return err
	}
	defer os.Remove(filename)
	sigfilename, err := download(getCurrentVersionTgzURL()+".sig", 1024)
	if err != nil {
		if sigfilename != "" {
			os.Remove(sigfilename)
		}
		upgradeError = errors.New("error downloading the signature file of the new server")
		mlog.Error("Unable to download the signature file of the new Mattermost server", mlog.String("url", getCurrentVersionTgzURL()+".sig"), mlog.Err(err))
		upgradePercentage = 0
		return err
	}
	defer os.Remove(sigfilename)

	err = verifySignature(filename, sigfilename, mattermostBuildPublicKey)
	if err != nil {
		upgradePercentage = 0
		upgradeError = errors.New("unable to verify the signature of the downloaded file")
		mlog.Error("Unable to verify the signature of the downloaded file", mlog.Err(err))
		return err
	}

	err = extractBinary(executablePath, filename)
	if err != nil {
		upgradePercentage = 0
		upgradeError = err
		mlog.Error("Unable to extract the binary from the downloaded file", mlog.Err(err))
		return err
	}
	upgradePercentage = 100
	return nil
}

func UpgradeToE0Status() (int64, error) {
	return upgradePercentage, upgradeError
}

func download(url string, limit int64) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	out, err := ioutil.TempFile("", "*_mattermost.tar.gz")
	if err != nil {
		return "", err
	}
	defer out.Close()

	counter := &writeCounter{total: resp.ContentLength}
	_, err = io.Copy(out, io.TeeReader(&io.LimitedReader{R: resp.Body, N: limit}, counter))
	return out.Name(), err
}

func getFilePermissionsOrDefault(filename string, def os.FileMode) os.FileMode {
	file, err := os.Open(filename)
	if err != nil {
		mlog.Warn("Unable to get the file permissions", mlog.String("filename", filename), mlog.Err(err))
		return def
	}
	defer file.Close()

	fileStats, err := file.Stat()
	if err != nil {
		mlog.Warn("Unable to get the file permissions", mlog.String("filename", filename), mlog.Err(err))
		return def
	}
	return fileStats.Mode()
}

func extractBinary(executablePath string, filename string) error {
	gzipStream, err := os.Open(filename)
	if err != nil {
		return err
	}

	uncompressedStream, err := gzip.NewReader(gzipStream)
	if err != nil {
		return err
	}

	tarReader := tar.NewReader(uncompressedStream)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			return errors.New("unable to find the Mattermost binary in the downloaded version")
		}

		if err != nil {
			return err
		}

		if header.Typeflag == tar.TypeReg && header.Name == "mattermost/bin/mattermost" {
			permissions := getFilePermissionsOrDefault(executablePath, 0755)
			tmpFile, err := ioutil.TempFile(path.Dir(executablePath), "*")
			if err != nil {
				return err
			}
			tmpFileName := tmpFile.Name()
			os.Remove(tmpFileName)
			err = os.Rename(executablePath, tmpFileName)
			if err != nil {
				return err
			}
			outFile, err := os.Create(executablePath)
			if err != nil {
				err2 := os.Rename(tmpFileName, executablePath)
				if err2 != nil {
					mlog.Critical("Unable to restore the backup of the executable file. Restore the executable file manually.")
					return errors.Wrap(err2, "critical error: unable to upgrade the binary or restore the old binary version. Please restore it manually")
				}
				return err
			}
			defer outFile.Close()
			if _, err = io.Copy(outFile, tarReader); err != nil {
				err2 := os.Remove(executablePath)
				if err2 != nil {
					mlog.Critical("Unable to restore the backup of the executable file. Restore the executable file manually.")
					return errors.Wrap(err2, "critical error: unable to upgrade the binary or restore the old binary version. Please restore it manually")
				}

				err2 = os.Rename(tmpFileName, executablePath)
				if err2 != nil {
					mlog.Critical("Unable to restore the backup of the executable file. Restore the executable file manually.")
					return errors.Wrap(err2, "critical error: unable to upgrade the binary or restore the old binary version. Please restore it manually")
				}
				return err
			}
			err = os.Remove(tmpFileName)
			if err != nil {
				mlog.Warn("Unable to clean up the binary backup file.", mlog.Err(err))
			}
			err = os.Chmod(executablePath, permissions)
			if err != nil {
				mlog.Warn("Unable to set the correct permissions for the file.", mlog.Err(err))
			}
			break
		}
	}
	return nil
}
