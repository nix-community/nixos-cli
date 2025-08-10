//go:build linux

package init

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/system"
)

const (
	swapDeviceListFilename        = "/proc/swaps"
	mountedFilesystemListFilename = "/proc/self/mountinfo"
)

type Filesystem struct {
	Mountpoint      string
	DevicePath      string
	FSType          string
	Options         []string
	LUKSInformation *LUKSInformation
}

type LUKSInformation struct {
	Name       string
	DevicePath string
}

func findSwapDevices(log *logger.Logger) []string {
	swapDevices := []string{}

	swapDeviceList, err := os.Open(swapDeviceListFilename)
	if err != nil {
		log.Warnf("failed to open swap device list %v: %v", swapDeviceListFilename, err)
		return swapDevices
	}
	defer func() { _ = swapDeviceList.Close() }()

	s := bufio.NewScanner(swapDeviceList)
	s.Split(bufio.ScanLines)

	_ = s.Scan() // Skip header line

	for s.Scan() {
		fields := strings.Fields(s.Text())
		swapFilename := fields[0]
		swapType := fields[1]

		switch swapType {
		case "partition":
			swapDevices = append(swapDevices, findStableDevPath(swapFilename))
		case "file":
			log.Infof("skipping swap file %v, specify in configuration manually if needed", swapFilename)
		default:
			log.Warnf("unsupported swap type %v for %v; do not specify in configuration", swapType, swapFilename)
		}
	}

	return swapDevices
}

func findFilesystems(log *logger.Logger, rootDir string) []Filesystem {
	filesystems := []Filesystem{}

	foundFileystems := make(map[string]string, 0)
	foundLuksDevices := make(map[string]struct{}, 0)

	mountList, err := os.Open(mountedFilesystemListFilename)
	if err != nil {
		log.Warnf("failed to open swap device list %v: %v", mountedFilesystemListFilename, err)
		return filesystems
	}
	defer func() { _ = mountList.Close() }()

	s := bufio.NewScanner(mountList)
	s.Split(bufio.ScanLines)

	for s.Scan() {
		fields := strings.Fields(s.Text())

		mountID := fields[2]
		path := fields[3]
		if path == "/" {
			path = ""
		}

		absoluteMountpoint := strings.ReplaceAll(fields[4], "\\040", "")

		if stat, err := os.Stat(absoluteMountpoint); err != nil || !stat.IsDir() {
			continue
		}

		if !isSubdir(absoluteMountpoint, rootDir) {
			continue
		}

		var mountpoint string
		if absoluteMountpoint == rootDir {
			mountpoint = "/"
		} else {
			mountpoint = absoluteMountpoint[len(rootDir):]
		}

		mountOptions := strings.Split(fields[5], ",")

		if isSubdir(mountpoint, "/proc") || isSubdir(mountpoint, "/sys") || isSubdir(mountpoint, "/dev") || isSubdir(mountpoint, "/run") {
			continue
		} else if mountpoint == "/var/lib/nfs/rpc_pipefs" {
			continue
		}

		// Skip irrelevant fields
		n := 6
		for ; n < len(fields); n++ {
			if fields[n] == "-" {
				n++
				break
			}
		}

		// Sanity check. If the mount entry is malformed, we should not attempt
		// to access the rest of the fields, lest we risk an OOB.
		if n > len(fields)-3 {
			log.Warnf("malformed mount entry: %v", s.Text())
			continue
		}

		fsType := fields[n]

		devicePath := fields[n+1]
		devicePath = strings.ReplaceAll(devicePath, "\\040", "")
		devicePath = strings.ReplaceAll(devicePath, "\\011", "\t")

		superblockOptions := strings.Split(fields[n+2], ",")

		// Skip read-only Nix store bind mount
		if mountpoint == "/nix/store" && slices.Contains(superblockOptions, "rw") && slices.Contains(mountOptions, "ro") {
			continue
		}

		if fsType == "fuse" || fsType == "fuseblk" {
			log.Warnf("don't know how to emit `fileSystem` option for FUSE filesystem '%v'", mountpoint)
			continue
		}

		if mountpoint == "/tmp" && fsType == "tmpfs" {
			continue
		}

		if existingFsPath, ok := foundFileystems[mountID]; ok {
			// TODO: check if filesystem is a btrfs subvolume

			filesystems = append(filesystems, Filesystem{
				Mountpoint: mountpoint,
				DevicePath: filepath.Join(existingFsPath, path),
				FSType:     fsType,
				Options:    []string{"bind"},
			})

			continue
		}

		foundFileystems[mountID] = path

		extraOptions := []string{}

		if strings.HasPrefix(devicePath, "/dev/loop") {
			startIndex := len("/dev/loop")
			endIndex := strings.Index(devicePath[startIndex:], "/")
			if endIndex == -1 {
				endIndex = len(devicePath)
			}
			loopNumber := devicePath[startIndex:endIndex]

			backerFilename := fmt.Sprintf("/sys/block/loop%s/loop/backing_file", loopNumber)

			if backer, err := os.ReadFile(backerFilename); err == nil {
				devicePath = string(backer)
				extraOptions = append(extraOptions, "loop")
			}
		}

		// Preserve umask for FAT filesystems in order to preserve
		// EFI system partition security.
		if fsType == "vfat" {
			for _, o := range superblockOptions {
				if o == "fmask" || o == "dmask" {
					extraOptions = append(extraOptions, o)
				}
			}
		}

		// TODO: check if filesystem is a btrfs subvolume

		// TODO: check if Stratis pool

		filesystemToAdd := Filesystem{
			Mountpoint: mountpoint,
			DevicePath: findStableDevPath(devicePath),
			FSType:     fsType,
			Options:    extraOptions,
		}

		deviceName := filepath.Base(devicePath)
		filesystemToAdd.LUKSInformation = queryLUKSInformation(deviceName, foundLuksDevices)

		filesystems = append(filesystems, filesystemToAdd)

	}

	return filesystems
}

func lvmDevicesExist(s system.CommandRunner, log *logger.Logger) bool {
	cmd := system.NewCommand("lsblk", "-o", "TYPE")

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	_, err := s.Run(cmd)
	if err != nil {
		log.Warnf("failed to run lsblk: %v", err)
		return false
	}

	// There should probably be a better metric than just checking if the
	// string "lvm" exists inside `lsblk` output, but meh. This is a rough
	// heuristic that somewhat works unless device labels end up having "lvm"
	// in them.
	return strings.Contains(stdout.String(), "lvm")
}

func bcachefsFilesystemsExist(log *logger.Logger) bool {
	entries, err := os.ReadDir("/dev")
	if err != nil {
		log.Warnf("failed to read /dev: %v", err)
		return false
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "bcache") {
			return true
		}
	}

	return false
}

func findStableDevPath(devicePath string) string {
	if !filepath.IsAbs(devicePath) {
		return devicePath
	}

	rdev, err := getRdev(devicePath)
	if err != nil {
		return devicePath
	}

	directoriesToCheck := []string{
		"/dev/disk/by-uuid",
		"/dev/mapper",
		"/dev/disk/by-label",
	}

	for _, directory := range directoriesToCheck {
		if stablePath, ok := checkDirForEqualDevice(rdev, directory); ok {
			return stablePath
		}
	}

	return devicePath
}

func getRdev(devicePath string) (uint64, error) {
	stat, err := os.Stat(devicePath)
	if err != nil {
		return 0, err
	}
	sysStat := stat.Sys()
	if sysStat == nil {
		return 0, err
	}
	devStat, ok := sysStat.(*syscall.Stat_t)
	if !ok {
		return 0, err
	}

	return devStat.Rdev, nil
}

func checkDirForEqualDevice(deviceRdev uint64, dirname string) (string, bool) {
	entries, err := os.ReadDir(dirname)
	if err != nil {
		return "", false
	}

	for _, entry := range entries {
		devicePath := filepath.Join(dirname, entry.Name())
		rdev, err := getRdev(devicePath)
		if err != nil {
			continue
		}

		if rdev == deviceRdev {
			return devicePath, true
		}
	}

	return "", false
}

func isSubdir(subdir string, dir string) bool {
	if len(dir) == 0 || dir == "/" {
		return true
	}

	if dir == subdir {
		return true
	}

	if len(subdir) <= len(dir)+1 {
		return false
	}

	return strings.Index(subdir, dir) == 0 && subdir[len(dir)] == '/'
}

func queryLUKSInformation(deviceName string, foundLuksDevices map[string]struct{}) *LUKSInformation {
	// Check if the device in question is a LUKS device.
	uuidFilename := fmt.Sprintf("/sys/class/block/%s/dm/uuid", deviceName)
	uuidFileContents, err := os.ReadFile(uuidFilename)
	if err != nil {
		return nil
	}
	if !strings.HasPrefix(string(uuidFileContents), "CRYPT_LUKS") {
		return nil
	}

	// Then, make sure it has a single slave device. These are the only types of
	// supported LUKS devices for filesystem generation.
	slaveDeviceDirname := fmt.Sprintf("/sys/class/block/%s/slaves", deviceName)
	slaveDeviceEntries, err := os.ReadDir(slaveDeviceDirname)
	if err != nil {
		return nil
	}

	if len(slaveDeviceEntries) != 1 {
		return nil
	}

	// Get the real name of the device that LUKS is using, and attempt to find
	// a stable device path for it.
	slaveName := slaveDeviceEntries[0].Name()
	slaveDeviceName := filepath.Join("/dev", slaveName)
	dmNameFilename := fmt.Sprintf("/sys/class/block/%s/dm/name", slaveDeviceName)

	dmNameFileContents, err := os.ReadFile(dmNameFilename)
	if err != nil {
		return nil
	}
	dmName := strings.TrimSpace(string(dmNameFileContents))

	realDevicePath := findStableDevPath(dmName)

	// Check if the device has already been found.
	if _, ok := foundLuksDevices[dmName]; ok {
		return nil
	}
	foundLuksDevices[dmName] = struct{}{}

	return &LUKSInformation{
		Name:       dmName,
		DevicePath: realDevicePath,
	}
}
