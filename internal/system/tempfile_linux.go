//go:build linux

package system

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

type TempFile struct {
	memFd int
}

func NewTempFile(pattern string, content []byte) (*TempFile, error) {
	memFd, err := unix.MemfdCreate(pattern, unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING|unix.MFD_NOEXEC_SEAL)
	if err != nil {
		return nil, fmt.Errorf("failed to create memfd: %v", err)
	}

	defer func() {
		if err != nil {
			_ = unix.Close(memFd)
		}
	}()

	if _, err = unix.Write(memFd, content); err != nil {
		return nil, fmt.Errorf("failed to write to memfd: %v", err)
	}

	_, err = unix.FcntlInt(uintptr(memFd), unix.F_ADD_SEALS, unix.F_SEAL_SEAL|unix.F_SEAL_SHRINK|unix.F_SEAL_GROW|unix.F_SEAL_WRITE)
	if err != nil {
		return nil, fmt.Errorf("failed to add seals to memfd: %v", err)
	}

	if err = unix.Fchmod(memFd, 0o400); err != nil {
		return nil, fmt.Errorf("failed to change permissions of memfd: %v", err)
	}

	t := TempFile{
		memFd: memFd,
	}

	return &t, nil
}

func (t *TempFile) Path() string {
	return fmt.Sprintf("/proc/%v/fd/%v", os.Getpid(), t.memFd)
}

func (t *TempFile) Remove() error {
	return unix.Close(t.memFd)
}
