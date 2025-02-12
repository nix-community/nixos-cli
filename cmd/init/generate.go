package init

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"strings"

	buildOpts "github.com/water-sucks/nixos/internal/build"
	"github.com/water-sucks/nixos/internal/config"
	"github.com/water-sucks/nixos/internal/logger"
	"github.com/water-sucks/nixos/internal/system"
)

//go:embed hardware_configuration.nix.txt
var hardwareConfigurationNixTemplate string

//go:embed configuration.nix.txt
var configurationNixTemplate string

//go:embed flake.nix.txt
var flakeNixTemplate string

func generateHwConfigNix(s system.CommandRunner, log *logger.Logger, cfg *config.Config, virtType VirtualisationType, scanFilesystems bool) (string, error) {
	imports := []string{}
	initrdAvailableModules := []string{}
	initrdModules := []string{}
	kernelModules := []string{}
	modulePackages := []string{}
	extraAttrs := []KVPair{}

	for k, v := range cfg.Init.ExtraAttrs {
		extraAttrs = append(extraAttrs, KVPair{Key: k, Value: v})
	}

	hwConfigSettings := hardwareConfigSettings{
		Imports:                &imports,
		InitrdAvailableModules: &initrdAvailableModules,
		InitrdModules:          &initrdModules,
		KernelModules:          &kernelModules,
		ModulePackages:         &modulePackages,
		Attrs:                  &extraAttrs,
	}
	_ = hwConfigSettings

	if cfg.Init.ExtraAttrs != nil {
		for k, v := range cfg.Init.ExtraAttrs {
			extraAttrs = append(extraAttrs, KVPair{Key: k, Value: v})
		}
	}

	log.Infof("determining host platform")
	hostPlatform, err := determineHostPlatform(s)
	if err != nil {
		log.Warnf("failed to determine host platform: %v", err)
		log.Info("fill in the `nixpkgs.hostPlatform` attribute in your hardware-configuration.nix before continuing installation")
	} else {
		log.Infof("host platform: %v", hostPlatform)
		extraAttrs = append(extraAttrs, KVPair{Key: "nixpkgs.hostPlatform", Value: hostPlatform})
	}

	cpuInfo := getCPUInfo(log)

	log.Infof("detected CPU type: %v", cpuInfo.Manufacturer.CPUType())
	log.Infof("KVM virtualisation enabled: %v", cpuInfo.VirtualisationEnabled)
	log.Infof("virtualisation type of current host: %v", virtType)

	// Add KVM modules if need be.
	if cpuInfo.VirtualisationEnabled {
		switch cpuInfo.Manufacturer {
		case manufacturerIntel:
			kernelModules = append(kernelModules, "kvm-intel")
		case manufacturerAMD:
			kernelModules = append(kernelModules, "kvm-amd")
		}
	}

	switch virtType {
	case VirtualisationTypeOracle:
		extraAttrs = append(extraAttrs, KVPair{Key: "virtualisation.virtualbox.guest.enable", Value: "true"})
	case VirtualisationTypeParallels:
		extraAttrs = append(extraAttrs, KVPair{Key: "hardware.parallels.enable", Value: "true"})
		extraAttrs = append(extraAttrs, KVPair{Key: "nixpkgs.config.allowUnfreePredicate", Value: `pkg: builtins.elem (lib.getName pkg [ "prl-tools" ])`})
	case VirtualisationTypeQemu, VirtualisationTypeKVM, VirtualisationTypeBochs:
		imports = append(imports, `(modulesPath + "/profiles/qemu-guest.nix")`)
	case VirtualisationTypeHyperV:
		extraAttrs = append(extraAttrs, KVPair{Key: "virtualisation.hypervGuest.enable", Value: "true"})
	case VirtualisationTypeSystemdNspawn:
		extraAttrs = append(extraAttrs, KVPair{Key: "boot.isContainer", Value: "true"})
	case VirtualisationTypeNone:
		imports = append(imports, `(modulesPath + "/installer/scan/not-detected.nix")`)
		switch cpuInfo.Manufacturer {
		case manufacturerIntel:
			extraAttrs = append(extraAttrs, KVPair{Key: "hardware.cpu.intel.updateMicrocode", Value: "lib.mkDefault config.hardware.enableRedistributableFirmware"})
		case manufacturerAMD:
			extraAttrs = append(extraAttrs, KVPair{Key: "hardware.cpu.amd.updateMicrocode", Value: "lib.mkDefault config.hardware.enableRedistributableFirmware"})
		}
	}

	findPCIDevices(&hwConfigSettings, log)
	findUSBDevices(&hwConfigSettings, log)

	findGenericDevicesInDir(&hwConfigSettings, log, blockDeviceDirname)
	findGenericDevicesInDir(&hwConfigSettings, log, mmcDeviceDirname)

	networkInterfaces := detectNetworkInterfaces()
	networkInterfaceLines := []string{}
	for _, i := range networkInterfaces {
		networkInterfaceLines = append(networkInterfaceLines, fmt.Sprintf("  # networking.interfaces.%v.useDHCP = lib.mkDefault true;", i))
	}

	// TODO: detect bcachefs
	// TODO: detect LVM
	// TODO: find swap devices
	// TODO: find filesystems

	extraAttrLines := make([]string, len(extraAttrs))
	for i, attr := range extraAttrs {
		extraAttrLines[i] = fmt.Sprintf("  %v = %v;", attr.Key, attr.Value)
	}

	return fmt.Sprintf(
		hardwareConfigurationNixTemplate,
		strings.Join(imports, "\n    "),
		nixStringList(initrdAvailableModules),
		nixStringList(initrdModules),
		nixStringList(kernelModules),
		strings.Join(modulePackages, " "),
		"", // TODO: filesystems
		"", // TODO: swap devices
		strings.Join(networkInterfaceLines, "\n")+"\n",
		strings.Join(extraAttrLines, "\n"),
	), nil
}

func generateConfigNix(log *logger.Logger, cfg *config.Config, virtType VirtualisationType) (string, error) {
	var bootloaderConfig string

	if _, err := os.Stat("/sys/firmware/efi/efivars"); err == nil {
		log.Info("EFI system detected, using systemd-boot for bootloader")

		bootloaderConfig = `  # Use the systemd-boot EFI bootloader.
  boot.loader.systemd-boot.enable = true;
  boot.loader.efi.canTouchEfiVariables = true;
`
	} else if _, err := os.Stat("/boot/extlinux"); err == nil {
		log.Info("extlinux bootloader detected, using generic-extlinux-compatible bootloader")

		bootloaderConfig = `  # Use the extlinux bootloader.
  boot.loader.generic-extlinux-compatible.enable = true;
  # Disable GRUB, because NixOS enables it by default.
  boot.loader.grub.enable = false
`
	} else if virtType != VirtualisationTypeSystemdNspawn {
		log.Info("using GRUB2 for bootloader")

		bootloaderConfig = `  # Use the GRUB 2 bootloader.
  boot.loader.grub.enable = true;
  # boot.loader.grub.efiSupport = true;
  # boot.loader.grub.efiInstallAsRemovable = true;
  # boot.loader.efi.efiSysMountPoint = "/boot/efi";
  # Define on which hard drive you want to install GRUB2.
  # boot.loader.grub.device = "/dev/sda"; # or "nodev" for EFI systems
`
	} else {
		log.Info("container system (systemd-nspawn) detected, no bootloader is required")
	}

	var xserverConfig string
	if cfg.Init.EnableXserver {
		xserverConfig = `  # Enable the X11 windowing system.
  services.xserver.enable = true;
`
	} else {
		xserverConfig = `  # Enable the X11 windowing system.
  # services.xserver.enable = true;
`
	}

	return fmt.Sprintf(
		configurationNixTemplate,
		bootloaderConfig,
		xserverConfig,
		cfg.Init.DesktopConfig,
		cfg.Init.ExtraConfig,
		buildOpts.NixpkgsVersion,
	), nil
}

func generateFlakeNix() string {
	nixpkgsInputLine := fmt.Sprintf(`nixpkgs.url = "github:NixOS/nixpkgs/release-%v";`, buildOpts.NixpkgsVersion)
	return fmt.Sprintf(flakeNixTemplate, nixpkgsInputLine)
}

type KVPair struct {
	Key   string
	Value string
}

type hardwareConfigSettings struct {
	Imports                *[]string
	InitrdAvailableModules *[]string
	InitrdModules          *[]string
	KernelModules          *[]string
	ModulePackages         *[]string
	Attrs                  *[]KVPair
}

func determineHostPlatform(s system.CommandRunner) (string, error) {
	cmd := system.NewCommand("nix-instantiate", "--eval", "--expr", "builtins.currentSystem")

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	_, err := s.Run(cmd)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(stdout.String()), nil
}

func nixString(s string) string {
	return fmt.Sprintf(`"%v"`, strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), `"`, `\"`))
}

// Serialize a slice of strings to a Nix string list, without duplicates.
// Caller must add the opening and closing brackets, or ensure they exist
// in the format string.
func nixStringList(s []string) string {
	itemSet := make(map[string]bool)
	quotedItems := make([]string, len(s))

	for i, item := range s {
		if itemSet[item] {
			continue
		}

		itemSet[item] = true
		quotedItems[i] = nixString(item)
	}

	return strings.Join(quotedItems, " ")
}
