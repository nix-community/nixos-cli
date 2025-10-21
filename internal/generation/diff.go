package generation

import (
	"os/exec"

	"github.com/nix-community/nixos-cli/internal/system"
)

type DiffCommandOptions struct {
	UseNvd  bool
	Verbose bool
}

func RunDiffCommand(s system.CommandRunner, before string, after string, opts *DiffCommandOptions) error {
	log := s.Logger()

	useNvd := opts.UseNvd

	if opts.UseNvd {
		nvdPath, _ := exec.LookPath("nvd")
		nvdFound := nvdPath != ""
		if !nvdFound {
			log.Warn("use_nvd is specified in config, but `nvd` is not executable")
			log.Warn("falling back to `nix store diff-closures`")
			useNvd = false
		}
	}

	argv := []string{"nix", "store", "diff-closures", before, after}
	if useNvd {
		argv = []string{"nvd", "diff", before, after}
	}

	if opts.Verbose {
		s.Logger().CmdArray(argv)
	}

	cmd := system.NewCommand(argv[0], argv[1:]...)
	_, err := s.Run(cmd)
	return err
}
