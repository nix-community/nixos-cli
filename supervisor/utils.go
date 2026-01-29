//go:build linux

package main

import (
	"os"

	"github.com/nix-community/nixos-cli/internal/logger"
	"golang.org/x/sys/unix"
)

func acquireProcessLock(log logger.Logger, lockfile string) (unlockFunc func(), err error) {
	file, err := os.OpenFile(lockfile, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		log.Errorf("failed to create activation lockfile %s: %s", lockfile, err)
		return
	}
	// God, I wish I had Zig's `errdefer`.
	defer func() {
		if err != nil {
			_ = file.Close()
		}
	}()

	if err = unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		log.Errorf("failed to lock %s; another process may be running", lockfile)
		return
	}

	unlockFunc = func() {
		_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
		_ = file.Close()
		_ = os.Remove(lockfile)
	}

	return
}
