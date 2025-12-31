//go:build windows

package daemon

import (
	"errors"
	"os"
)

// sendSIGTERM is a stub on Windows - the test that uses this skips on Windows.
func sendSIGTERM(process *os.Process) error {
	return errors.New("not supported by windows")
}
