package init

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nix-community/nixos-cli/internal/build"
	"github.com/nix-community/nixos-cli/internal/cmd/opts"
	"github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/settings"
	"github.com/nix-community/nixos-cli/internal/system"
	"github.com/spf13/cobra"
)

func InitCommand() *cobra.Command {
	opts := cmdOpts.InitOpts{}

	cmd := cobra.Command{
		Use:   "init",
		Short: "Initialize a NixOS configuration",
		Long:  "Initialize a NixOS configuration template and/or hardware options.",
		Args: func(cmd *cobra.Command, args []string) error {
			if !filepath.IsAbs(opts.Root) {
				return fmt.Errorf("--root must be an absolute path")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdUtils.CommandErrorHandler(initMain(cmd, &opts))
		},
	}

	cmdUtils.SetHelpFlagText(&cmd)

	cmd.Flags().StringVarP(&opts.Directory, "dir", "d", "/etc/nixos", "Directory `path` in root to write to")
	cmd.Flags().BoolVarP(&opts.ForceWrite, "force", "f", false, "Force generation of all configuration files")
	cmd.Flags().BoolVarP(&opts.NoFSGeneration, "no-fs", "n", false, "Do not generate 'fileSystem' options configuration")
	cmd.Flags().StringVarP(&opts.Root, "root", "r", "/", "Treat `path` as the root directory")
	cmd.Flags().BoolVarP(&opts.ShowHardwareConfig, "show-hardware-config", "s", false, "Print hardware config to stdout and exit")

	return &cmd
}

func initMain(cmd *cobra.Command, opts *cmdOpts.InitOpts) error {
	log := logger.FromContext(cmd.Context())
	cfg := settings.FromContext(cmd.Context())
	s := system.NewLocalSystem(log)

	virtType := determineVirtualisationType(s, log)

	log.Step("Generating hardware-configuration.nix...")

	hwConfigNixText, err := generateHwConfigNix(s, log, cfg, virtType, opts)
	if err != nil {
		log.Errorf("failed to generate hardware-configuration.nix: %v", err)
		return err
	}

	if opts.ShowHardwareConfig {
		fmt.Println(hwConfigNixText)
		return nil
	}

	log.Step("Generating configuration.nix...")

	configNixText, err := generateConfigNix(log, cfg, virtType)
	if err != nil {
		log.Errorf("failed to generate configuration.nix: %v", err)
	}

	log.Step("Writing configuration...")

	configDir := filepath.Join(opts.Root, opts.Directory)
	err = os.MkdirAll(configDir, 0o755)
	if err != nil {
		log.Errorf("failed to create %v: %v", configDir, err)
		return err
	}

	if build.Flake() {
		flakeNixText := generateFlakeNix()
		flakeNixFilename := filepath.Join(configDir, "flake.nix")
		log.Infof("writing %v", flakeNixFilename)

		if _, err := os.Stat(flakeNixFilename); err == nil {
			if opts.ForceWrite {
				log.Warn("overwriting existing flake.nix")
			} else {
				log.Error("not overwriting existing flake.nix since --force was not specified, exiting")
				return nil
			}
		}

		err = os.WriteFile(flakeNixFilename, []byte(flakeNixText), 0o644)
		if err != nil {
			log.Errorf("failed to write %v: %v", flakeNixFilename, err)
			return err
		}
	}

	configNixFilename := filepath.Join(configDir, "configuration.nix")
	log.Infof("writing %v", configNixFilename)
	if _, err := os.Stat(configNixFilename); err == nil {
		if opts.ForceWrite {
			log.Warn("overwriting existing configuration.nix")
		} else {
			log.Error("not overwriting existing configuration.nix since --force was not specified, exiting")
			return nil
		}
	}
	err = os.WriteFile(configNixFilename, []byte(configNixText), 0o644)
	if err != nil {
		log.Errorf("failed to write %v: %v", configNixFilename, err)
		return err
	}

	hwConfigNixFilename := filepath.Join(configDir, "hardware-configuration.nix")
	log.Infof("writing %v", hwConfigNixFilename)
	if _, err := os.Stat(hwConfigNixFilename); err == nil {
		log.Warn("overwriting existing hardware-configuration.nix")
	}
	err = os.WriteFile(hwConfigNixFilename, []byte(hwConfigNixText), 0o644)
	if err != nil {
		log.Errorf("failed to write %v: %v", hwConfigNixFilename, err)
		return err
	}

	return nil
}
