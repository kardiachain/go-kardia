// not work for non-*nix OS.
package sysutils

import "syscall"

// FDRaise tries to maximize the file descriptor allowance of this process
// to the maximum hard-limit allowed by the OS.
func FDRaise(max uint64) error {
	// Get the current limit
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return err
	}
	// Try to update the limit to the max allowance
	limit.Cur = limit.Max
	if limit.Cur > max {
		limit.Cur = max
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return err
	}
	return nil
}

// FDCurrent retrieves the number of file descriptors allowed to be opened by this
// process.
func FDCurrent() (int, error) {
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return 0, err
	}
	return int(limit.Cur), nil
}

// FDMaximum retrieves the maximum number of file descriptors this process is
// allowed to request for itself.
func FDMaximum() (int, error) {
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return 0, err
	}
	return int(limit.Max), nil
}
