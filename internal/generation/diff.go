package generation

import (
	"os/exec"

	"github.com/nix-community/nixos-cli/internal/settings"
	"github.com/nix-community/nixos-cli/internal/system"
)

type DiffCommandOptions struct {
	DiffTool    settings.DiffTool
	DiffToolCmd []string
}

func RunDiffCommand(s system.System, before string, after string, opts *DiffCommandOptions) error {
	log := s.Logger()

	tool := opts.DiffTool

	switch tool {
	case settings.DifferInternal:
		if s.IsRemote() {
			log.Warn("the internal differ does not work with remote systems")
			tool = settings.DifferNix
		}
	case settings.DifferCommand:
		cmd := opts.DiffToolCmd[0]
		if cmdPath, _ := exec.LookPath(cmd); cmdPath == "" {
			log.Warnf("differ.command uses '%s', but `%s` is not executable", cmd, cmd)
			tool = settings.DifferNix
		}
	case settings.DifferNvd:
		if nvdPath, _ := exec.LookPath("nvd"); nvdPath == "" {
			log.Warn("differ.tool is set to 'nvd', but `nvd` is not executable")
			tool = settings.DifferNix
		}
	}

	if opts.DiffTool != tool && tool == settings.DifferNix {
		log.Warn("falling back to `nix store diff-closures`")
	}

	var argv []string

	switch tool {
	case settings.DifferCommand:
		argv = append(opts.DiffToolCmd, before, after)
	case settings.DifferInternal:
		// TODO : implement diff
		log.Info("internal differ currently not implemented")
		return nil
	case settings.DifferNix:
		argv = []string{"nix", "store", "diff-closures", before, after}
	case settings.DifferNvd:
		argv = []string{"nvd", "diff", before, after}
	}

	s.Logger().CmdArray(argv)

	cmd := system.NewCommand(argv[0], argv[1:]...)
	_, err := s.Run(cmd)
	return err
}
