package activate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/nix-community/nixos-cli/internal/logger"
)

func execUserSwitchProcess(uid uint32, gid uint32, runtimePath string) error {
	exe, err := filepath.EvalSymlinks("/proc/self/exe")
	if err != nil {
		return err
	}

	cmd := exec.Command(exe, os.Args[1:]...)

	cmd.Env = []string{
		fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimePath),
		fmt.Sprintf("%s=%s", NIXOS_STC_PARENT_EXE, exe),
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uid,
			Gid: gid,
		},
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func userSwitch(log *logger.Logger, parentExe string) error {
	childExe, err := filepath.EvalSymlinks("/proc/self/exe")
	if err != nil {
		log.Errorf("failed to get path of exe: %v", err)
		return err
	}

	if childExe != parentExe {
		err := fmt.Errorf("this program is not meant to be called from outside of `nixos activate`")
		log.Error(err)
		return err
	}

	ctx := context.Background()

	systemd, err := systemdDbus.NewUserConnectionContext(ctx)
	if err != nil {
		log.Errorf("failed to initialize systemd dbus connection: %v", err)
		return err
	}
	defer systemd.Close()

	// The systemd user session seems to not send a Reloaded signal,
	// so we don't have anything to wait on here.
	//
	// Also, ignore errors, since the dbus session bus will probably
	// return an error here due to it running in the user's context.
	_ = systemd.ReexecuteContext(ctx)

	nixosActivationStatus := make(chan string, 1)

	_, err = systemd.RestartUnitContext(ctx, "nixos-activation.service", "replace", nixosActivationStatus)
	if err != nil {
		log.Errorf("failed to restart nixos-activation.service: %v", err)
		return err
	}

	status := <-nixosActivationStatus
	if status == "timeout" || status == "failed" || status == "dependency" {
		err := fmt.Errorf("restarting nixos-activation.service failed with status %s", status)
		log.Error(err)
		return err
	}

	return nil
}
