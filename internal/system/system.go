package system

import (
	"bytes"
	"strings"

	"github.com/nix-community/nixos-cli/internal/cmd/nixopts"
	"github.com/nix-community/nixos-cli/internal/logger"
)

type System interface {
	CommandRunner
	IsNixOS() bool
	IsRemote() bool
	FS() Filesystem
}

// Invoke the `nix-copy-closure` command to copy between two types of
// systems.
func CopyClosures(src System, dest System, paths []string, extraNixOpts nixopts.NixOptionsSet) error {
	log := src.Logger()

	if len(paths) == 0 {
		log.Debugf("no store paths to copy")
		return nil
	}

	commandRunner := NewLocalSystem(log)

	nixCopySupported := checkNixCommandSupport(commandRunner)

	var argv []string

	// All type asserts must work here, otherwise the IsRemote() method is
	// implemented incorrectly for a given platform or the conditions are
	// put together incorrectly.
	//
	// There are/will be no other system types implemented, so casting
	// directly is fine here.
	if src.IsRemote() && dest.IsRemote() {
		// remote -> remote, so treat the source as a store and use the local
		// machine as the command runner.
		//
		// This should either be running as a trusted user or as root, so
		// remote store access should exist.
		src := src.(*SSHSystem)
		dest := dest.(*SSHSystem)

		if src.Address() == dest.Address() {
			log.Debugf("remotes have the same address, skipping copy")
			return nil
		}

		if nixCopySupported {
			argv = []string{"nix", "copy", "--from", src.AddressWithScheme(), "--to", dest.AddressWithScheme()}
		} else {
			argv = []string{"nix-copy-closure", "--store", src.AddressWithScheme(), "--to", dest.Address()}
			if src.cfg.StoreType == NixStoreTypeSSHNG || dest.cfg.StoreType == NixStoreTypeSSHNG {
				log.Warn("the ssh-ng store type is only partially supported in legacy mode")
			}
		}
	} else if src.IsRemote() {
		// remote -> local
		src := src.(*SSHSystem)
		if nixCopySupported {
			argv = []string{"nix", "copy", "--from", src.AddressWithScheme()}
		} else {
			argv = []string{"nix-copy-closure", "--from", src.Address()}
			if src.cfg.StoreType == NixStoreTypeSSHNG {
				log.Warn("the ssh-ng store type is only partially supported in legacy mode")
			}
		}
	} else if dest.IsRemote() {
		// local -> remote
		dest := dest.(*SSHSystem)
		if nixCopySupported {
			argv = []string{"nix", "copy", "--to", dest.AddressWithScheme()}
		} else {
			argv = []string{"nix-copy-closure", "--to", dest.Address()}
			if dest.cfg.StoreType == NixStoreTypeSSHNG {
				log.Warn("the ssh-ng store type is only partially supported in legacy mode")
			}
		}
	} else {
		// local -> local, no-op
		log.Debugf("both systems are local, skipping copy")
		return nil
	}

	if extraNixOpts != nil {
		cmdToUse := nixopts.CmdCopyClosure
		if nixCopySupported {
			cmdToUse = nixopts.CmdCopy
		}
		argv = append(argv, extraNixOpts.ArgsForCommand(cmdToUse)...)
	}

	if log.GetLogLevel() == logger.LogLevelDebug {
		argv = append(argv, "-v")
	}

	argv = append(argv, paths...)

	log.CmdArray(argv)

	cmd := NewCommand(argv[0], argv[1:]...)

	cmd.InheritEnv("NIX_SSHOPTS")

	_, err := commandRunner.Run(cmd)
	return err
}

func checkNixCommandSupport(s System) bool {
	checkSupportCmd := NewCommand("nix", "config", "show", "experimental-features")

	var stdout bytes.Buffer
	checkSupportCmd.Stdout = &stdout
	checkSupportCmd.Stderr = nil

	_, err := s.Run(checkSupportCmd)
	if err != nil {
		return false
	}

	return strings.Contains(stdout.String(), "nix-command")
}
