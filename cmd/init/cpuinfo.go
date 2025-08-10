//go:build linux

package init

import (
	"bufio"
	"bytes"
	"os"
	"strings"

	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/system"
)

type CPUInfo struct {
	VirtualisationEnabled bool
	Manufacturer          CPUManufacturer
}

type CPUManufacturer int

const (
	manufacturerIntel CPUManufacturer = iota
	manufacturerAMD
	manufacturerUnknown
)

func (c CPUManufacturer) CPUType() string {
	switch c {
	case manufacturerIntel:
		return "Intel"
	case manufacturerAMD:
		return "AMD"
	default:
		return "unknown"
	}
}

func getCPUInfo(log *logger.Logger) *CPUInfo {
	result := &CPUInfo{
		VirtualisationEnabled: false,
		Manufacturer:          manufacturerUnknown,
	}

	cpuinfoFile, err := os.Open("/proc/cpuinfo")
	if err != nil {
		log.Warnf("failed to open /proc/cpuinfo: %v", err)
		return result
	}

	defer func() { _ = cpuinfoFile.Close() }()

	s := bufio.NewScanner(cpuinfoFile)
	s.Split(bufio.ScanLines)

	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, "flags") {
			if strings.Contains(line, "vmx") || strings.Contains(line, "svm") {
				result.VirtualisationEnabled = true
			}
		} else if strings.HasPrefix(line, "vendor_id") {
			if strings.Contains(line, "GenuineIntel") {
				result.Manufacturer = manufacturerIntel
			} else if strings.Contains(line, "AuthenticAMD") {
				result.Manufacturer = manufacturerAMD
			}
		}
	}

	return result
}

type VirtualisationType int

const (
	VirtualisationTypeNone VirtualisationType = iota
	VirtualisationTypeOracle
	VirtualisationTypeParallels
	VirtualisationTypeQemu
	VirtualisationTypeKVM
	VirtualisationTypeBochs
	VirtualisationTypeHyperV
	VirtualisationTypeSystemdNspawn
	VirtualisationTypeUnknown
)

func (v VirtualisationType) String() string {
	switch v {
	case VirtualisationTypeOracle:
		return "Oracle"
	case VirtualisationTypeParallels:
		return "Parallels"
	case VirtualisationTypeQemu:
		return "QEMU"
	case VirtualisationTypeKVM:
		return "KVM"
	case VirtualisationTypeBochs:
		return "Bochs"
	case VirtualisationTypeHyperV:
		return "Hyper-V"
	case VirtualisationTypeSystemdNspawn:
		return "systemd-nspawn"
	case VirtualisationTypeNone:
		return "none"
	default:
		return "unknown"
	}
}

func determineVirtualisationType(s system.CommandRunner, log *logger.Logger) VirtualisationType {
	cmd := system.NewCommand("systemd-detect-virt")

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	_, err := s.Run(cmd)
	virtType := strings.TrimSpace(stdout.String())

	if err != nil {
		// Because yes, this fails with exit status 1. Stupid.
		if virtType == "none" {
			return VirtualisationTypeNone
		}

		log.Warnf("failed to run systemd-detect-virt: %v", err)
		return VirtualisationTypeUnknown
	}

	switch virtType {
	case "oracle":
		return VirtualisationTypeOracle
	case "parallels":
		return VirtualisationTypeParallels
	case "qemu":
		return VirtualisationTypeQemu
	case "kvm":
		return VirtualisationTypeKVM
	case "bochs":
		return VirtualisationTypeBochs
	case "microsoft":
		return VirtualisationTypeHyperV
	case "systemd-nspawn":
		return VirtualisationTypeSystemdNspawn
	default:
		log.Warnf("unknown virtualisation type: %v", virtType)
		return VirtualisationTypeUnknown
	}
}
