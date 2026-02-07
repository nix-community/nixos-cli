package system

import (
	"fmt"
	"os"

	shlex "github.com/carapace-sh/carapace-shlex"
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
func CopyClosures(src System, dest System, paths []string, extraArgs ...string) error {
	log := src.Logger()

	if len(paths) == 0 {
		log.Debugf("no store paths to copy")
		return nil
	}

	argv := []string{"nix-copy-closure"}

	var nixSSHOpts []string
	nixSSHOptsEnv := os.Getenv("NIX_SSHOPTS")
	if nixSSHOptsEnv != "" {
		nixSSHOptsTokens, err := shlex.Split(nixSSHOptsEnv)
		if err != nil {
			return fmt.Errorf("failed to parse NIX_SSHOPTS: %v", err)
		}
		nixSSHOpts = nixSSHOptsTokens.Strings()
	}

	srcIsRemote := src.IsRemote()
	destIsRemote := dest.IsRemote()

	var commandRunner CommandRunner

	// All type asserts must work here, otherwise the IsRemote() method is
	// implemented incorrectly for a given platform or the conditions are
	// put together incorrectly.
	//
	// There are/will be no other system types implemented, so casting
	// directly is fine here.
	if srcIsRemote && destIsRemote {
		// remote -> remote, so treat the source as a store and use the local
		// machine as the command runner.
		//
		// This should either be running as a trusted user or as root, so
		// remote store access should exist.
		commandRunner = NewLocalSystem(log)
		srcAddr := src.(*SSHSystem).Address()
		destAddr := dest.(*SSHSystem).Address()
		if srcAddr == destAddr {
			log.Debugf("remotes have the same address, skipping copy")
			return nil
		}
		srcArg := fmt.Sprintf("ssh://%s", srcAddr)
		argv = append(argv, "--store", srcArg, "--to", destAddr)
		nixSSHOpts = append(nixSSHOpts, src.(*SSHSystem).NixSSHOpts()...)
		nixSSHOpts = append(nixSSHOpts, dest.(*SSHSystem).NixSSHOpts()...)
	} else if srcIsRemote && !destIsRemote {
		// remote -> local, so use --from and run on the local host (dest), since there
		// is no reliable way to run this on the remote while determining how
		// the local address appears to it.
		commandRunner = dest
		srcAddr := src.(*SSHSystem).Address()
		argv = append(argv, "--from", srcAddr)
		nixSSHOpts = append(nixSSHOpts, src.(*SSHSystem).NixSSHOpts()...)
	} else if !srcIsRemote && destIsRemote {
		// local -> remote, so run this command on the local host.
		commandRunner = src
		destAddr := dest.(*SSHSystem).Address()
		argv = append(argv, "--to", destAddr)
		nixSSHOpts = append(nixSSHOpts, dest.(*SSHSystem).NixSSHOpts()...)
	} else {
		// local -> local, no-op
		log.Debugf("both systems are local, skipping copy")
		return nil
	}

	argv = append(argv, extraArgs...)
	if log.GetLogLevel() == logger.LogLevelDebug {
		argv = append(argv, "-v")
	}

	argv = append(argv, paths...)

	log.CmdArray(argv)

	cmd := NewCommand(argv[0], argv[1:]...)
	nixSSHOptsEnv = shlex.Join(nixSSHOpts)
	if nixSSHOptsEnv != "" {
		cmd.SetEnv("NIX_SSHOPTS", nixSSHOptsEnv)
	}
	_, err := commandRunner.Run(cmd)
	return err
}
