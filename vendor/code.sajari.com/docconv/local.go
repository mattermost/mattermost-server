package docconv

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

// LocalFile is a type which wraps an *os.File.  See NewLocalFile for more details.
type LocalFile struct {
	*os.File

	unlink bool
}

// NewLocalFile ensures that there is a file which contains the data provided by r.  If r is
// actually an instance of *os.File then this file is used, otherwise a temporary file is
// created and the data from r copied into it.  Callers must call Done() when
// the LocalFile is no longer needed to ensure all resources are cleaned up.
func NewLocalFile(r io.Reader) (*LocalFile, error) {
	if f, ok := r.(*os.File); ok {
		return &LocalFile{
			File: f,
		}, nil
	}

	f, err := ioutil.TempFile(os.TempDir(), "docconv")
	if err != nil {
		return nil, fmt.Errorf("error creating temporary file: %v", err)
	}
	_, err = io.Copy(f, r)
	if err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, fmt.Errorf("error copying data into temporary file: %v", err)
	}

	return &LocalFile{
		File:   f,
		unlink: true,
	}, nil
}

// Done cleans up all resources.
func (l *LocalFile) Done() {
	l.Close()
	if l.unlink {
		os.Remove(l.Name())
	}
}
