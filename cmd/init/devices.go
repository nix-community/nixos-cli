//go:build linux

package init

import (
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/nix-community/nixos-cli/internal/logger"
)

const (
	pciDeviceDirname     = "/sys/bus/pci/devices"
	usbDeviceDirname     = "/sys/bus/usb/devices"
	blockDeviceDirname   = "/sys/class/block"
	mmcDeviceDirname     = "/sys/class/mmc_host"
	networkDeviceDirname = "/sys/class/net"
)

var (
	// Device numbers that use the Broadcom STA driver (wl.ko)
	broadcomStaDevices = []string{
		"0x4311", "0x4312", "0x4313", "0x4315",
		"0x4327", "0x4328", "0x4329", "0x432a",
		"0x432b", "0x432c", "0x432d", "0x4353",
		"0x4357", "0x4358", "0x4359", "0x4331",
		"0x43a0", "0x43b1",
	}
	//
	broadcomFullmacDevices = []string{
		"0x43a3", "0x43df", "0x43ec", "0x43d3",
		"0x43d9", "0x43e9", "0x43ba", "0x43bb",
		"0x43bc", "0xaa52", "0x43ca", "0x43cb",
		"0x43cc", "0x43c3", "0x43c4", "0x43c5",
	}
	virtioScsiDevices  = []string{"0x1004", "0x1048"}
	intel2200bgDevices = []string{
		"0x1043", "0x104f", "0x4220",
		"0x4221", "0x4223", "0x4224",
	}
	intel3945abgDevices = []string{
		"0x4229", "0x4230", "0x4222", "0x4227",
	}
)

func findPCIDevices(h *hardwareConfigSettings, log logger.Logger) {
	entries, err := os.ReadDir(pciDeviceDirname)
	if err != nil {
		log.Warnf("failed to read %v: %v", pciDeviceDirname, err)
		return
	}

findDevices:
	for _, entry := range entries {
		devicePath := filepath.Join(pciDeviceDirname, entry.Name())

		vendorFilename := filepath.Join(devicePath, "vendor")
		deviceFilename := filepath.Join(devicePath, "device")
		classFilename := filepath.Join(devicePath, "class")

		vendorContents, _ := os.ReadFile(vendorFilename)
		deviceContents, _ := os.ReadFile(deviceFilename)
		classContents, _ := os.ReadFile(classFilename)

		vendor := strings.TrimSpace(string(vendorContents))
		device := strings.TrimSpace(string(deviceContents))
		class := strings.TrimSpace(string(classContents))

		requiredModuleName := findModuleName(devicePath)
		if requiredModuleName != "" {
			// Add mass storage controllers, Firewire controllers, or USB controllers
			// (respectively) to the initrd modules list.
			if strings.HasPrefix(class, "0x01") || strings.HasPrefix(class, "0x02") || strings.HasPrefix(class, "0x0c03") {
				*h.InitrdAvailableModules = append(*h.InitrdAvailableModules, requiredModuleName)
			}
		}

		if vendor == "0x14e4" {
			// Broadcom devices
			for _, d := range broadcomStaDevices {
				if d == device {
					*h.ModulePackages = append(*h.ModulePackages, "config.boot.kernelPackages.broadcom_sta")
					*h.KernelModules = append(*h.KernelModules, "wl")
					continue findDevices
				}
			}

			for _, d := range broadcomFullmacDevices {
				if d == device {
					*h.ModulePackages = append(*h.ModulePackages, `(modulesPath + "/hardware/network/broadcom-43xx.nix")`)
					continue findDevices
				}
			}
		} else if vendor == "0x1af4" {
			// VirtIO SCSI devices
			for _, d := range virtioScsiDevices {
				if d == device {
					*h.InitrdAvailableModules = append(*h.InitrdAvailableModules, "virtio_scsi")
					continue findDevices
				}
			}
		} else if vendor == "0x8086" {
			// Intel devices
			for _, d := range intel2200bgDevices {
				if d == device {
					*h.Attrs = append(*h.Attrs, KVPair{Key: "networking.enableIntel2200BGFirmware", Value: "true"})
					continue findDevices
				}
			}

			for _, d := range intel3945abgDevices {
				if d == device {
					*h.Attrs = append(*h.Attrs, KVPair{Key: "networking.enableIntel3945ABGFirmware", Value: "true"})
					continue findDevices
				}
			}
		}
	}
}

func findUSBDevices(h *hardwareConfigSettings, log logger.Logger) {
	entries, err := os.ReadDir(usbDeviceDirname)
	if err != nil {
		log.Warnf("failed to read %s: %v", usbDeviceDirname, err)
		return
	}

	for _, entry := range entries {
		devicePath := filepath.Join(usbDeviceDirname, entry.Name())

		classFilename := filepath.Join(devicePath, "bInterfaceClass")
		protocolFilename := filepath.Join(devicePath, "bInterfaceProtocol")

		classContents, _ := os.ReadFile(classFilename)
		protocolContents, _ := os.ReadFile(protocolFilename)

		class := strings.TrimSpace(string(classContents))
		protocol := strings.TrimSpace(string(protocolContents))

		moduleName := findModuleName(devicePath)

		// Add modules for USB mass storage controllers (first condition) or keyboards (second condition)
		if strings.HasPrefix(class, "08") || (strings.HasPrefix(class, "03") && strings.HasPrefix(protocol, "01")) {
			*h.InitrdAvailableModules = append(*h.InitrdAvailableModules, moduleName)
		}
	}
}

func findGenericDevicesInDir(h *hardwareConfigSettings, log logger.Logger, deviceDirname string) {
	entries, err := os.ReadDir(deviceDirname)
	if err != nil {
		log.Warnf("failed to read %v: %v", deviceDirname, err)
		return
	}

	for _, entry := range entries {
		devicePath := filepath.Join(blockDeviceDirname, entry.Name(), "device")

		moduleName := findModuleName(devicePath)
		if moduleName != "" {
			*h.InitrdAvailableModules = append(*h.InitrdAvailableModules, moduleName)
		}
	}
}

func detectNetworkInterfaces() []string {
	detectedInterfaces := []string{}

	interfaces, _ := net.Interfaces()
	for _, i := range interfaces {
		// Skip loopback interfaces
		if !strings.HasPrefix(i.Name, "lo") {
			detectedInterfaces = append(detectedInterfaces, i.Name)
		}
	}

	return detectedInterfaces
}

func findModuleName(devicePath string) string {
	moduleFilename := filepath.Join(devicePath, "driver", "module")
	if _, err := os.Stat(moduleFilename); err != nil {
		return ""
	}

	realFilename, err := os.Readlink(moduleFilename)
	if err != nil {
		return ""
	}

	return filepath.Base(realFilename)
}
