//go:build linux

package activate

import (
	"bufio"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

type Filesystem struct {
	Device  string
	Type    string
	Options string
}

type Swap struct {
	Options string
}

// Parse the contents of an fstab(5)-formatted file.
//
// If the file is unable to be opened, an error is returned.
// Errors inside the file itself (i.e. missing fields)
// are ignored if possible.
func parseFstab(filename string) (map[string]Filesystem, map[string]Swap, error) {
	filesystems := make(map[string]Filesystem)
	swapDevices := make(map[string]Swap)

	file, err := os.Open(filename)
	if err != nil {
		return filesystems, swapDevices, err
	}
	defer func() { _ = file.Close() }()

	s := bufio.NewScanner(file)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())

		if strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)

		if len(fields) < 3 {
			continue
		}

		device, mountpoint, fsType := fields[0], fields[1], fields[2]

		var options string
		if len(fields) >= 4 {
			options = fields[3]
		}

		if fsType == "swap" {
			swapDevices[device] = Swap{
				Options: options,
			}
		} else {
			filesystems[mountpoint] = Filesystem{
				Device:  device,
				Type:    fsType,
				Options: options,
			}
		}
	}

	return filesystems, swapDevices, nil
}

// Invoke the swapoff(2) syscall on a device.
//
// This is not available in the x/sys/unix package, so
// we wrap this syscall in a Go-friendly manner ourselves.
func swapoff(device string) error {
	pathBytes, err := unix.BytePtrFromString(device)
	if err != nil {
		return err
	}

	_, _, errno := unix.Syscall(uintptr(unix.SYS_SWAPOFF), uintptr(unsafe.Pointer(pathBytes)), 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
