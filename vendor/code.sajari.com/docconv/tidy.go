package docconv

import (
	"io"
	"io/ioutil"
	"os"
	"os/exec"
)

// Tidy attempts to tidy up XML.
// Errors & warnings are deliberately suppressed as underlying tools
// throw warnings very easily.
func Tidy(r io.Reader, xmlIn bool) ([]byte, error) {
	f, err := ioutil.TempFile(os.TempDir(), "docconv")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	io.Copy(f, r)

	var output []byte
	if xmlIn {
		output, err = exec.Command("tidy", "-xml", "-numeric", "-asxml", "-quiet", "-utf8", f.Name()).Output()
	} else {
		output, err = exec.Command("tidy", "-numeric", "-asxml", "-quiet", "-utf8", f.Name()).Output()
	}

	if err != nil && err.Error() != "exit status 1" {
		return nil, err
	}
	return output, nil
}
