//go:build linux

package activate

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"

	"github.com/nix-community/nixos-cli/internal/activation"
	cmdOpts "github.com/nix-community/nixos-cli/internal/cmd/opts"
	"github.com/nix-community/nixos-cli/internal/constants"
	"github.com/nix-community/nixos-cli/internal/generation"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/system"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

const (
	ACTIVATION_LOCKFILE = "/run/nixos/switch-to-configuration.lock"

	DRY_RESTART_BY_ACTIVATION_LIST_FILE = "/run/nixos/dry-activation-restart-list"
	DRY_RELOAD_BY_ACTIVATION_LIST_FILE  = "/run/nixos/dry-activation-reload-list"
	RELOAD_BY_ACTIVATION_LIST_FILE      = "/run/nixos/activation-reload-list"
	RESTART_BY_ACTIVATION_LIST_FILE     = "/run/nixos/activation-restart-list"
)

type RequiredVars struct {
	OutPath              string
	Toplevel             string
	PreSwitchCheckCmd    string
	InstallBootloaderCmd string
	LocaleArchive        string
	NewSystemd           string
}

type ErrorRequiredVarMissing struct {
	VarName string
}

func (e ErrorRequiredVarMissing) Error() string {
	return fmt.Sprintf("missing required environment variable $%s, this is a bug", e.VarName)
}

func getRequiredVars() (*RequiredVars, error) {
	outPath := os.Getenv("OUT")
	if outPath == "" {
		return nil, ErrorRequiredVarMissing{VarName: "OUT"}
	}

	toplevel := os.Getenv("TOPLEVEL")
	if toplevel == "" {
		return nil, ErrorRequiredVarMissing{VarName: "TOPLEVEL"}
	}

	preSwitchCheck := os.Getenv("PRE_SWITCH_CHECK")
	if preSwitchCheck == "" {
		return nil, ErrorRequiredVarMissing{VarName: "PRE_SWITCH_CHECK"}
	}

	installBootloaderCmd := os.Getenv("INSTALL_BOOTLOADER")
	if installBootloaderCmd == "" {
		return nil, ErrorRequiredVarMissing{VarName: "INSTALL_BOOTLOADER"}
	}

	localeArchive := os.Getenv("LOCALE_ARCHIVE")
	if localeArchive == "" {
		return nil, ErrorRequiredVarMissing{VarName: "LOCALE_ARCHIVE"}
	}

	newSystemd := os.Getenv("SYSTEMD")
	if newSystemd == "" {
		return nil, ErrorRequiredVarMissing{VarName: "SYSTEMD"}
	}

	return &RequiredVars{
		OutPath:              outPath,
		Toplevel:             toplevel,
		PreSwitchCheckCmd:    preSwitchCheck,
		InstallBootloaderCmd: installBootloaderCmd,
		LocaleArchive:        localeArchive,
		NewSystemd:           newSystemd,
	}, nil
}

func execInSwitchContext(
	s system.CommandRunner,
	log *logger.Logger,
	action activation.SwitchToConfigurationAction,
	specialisation string,
) error {
	if specialisation != "" {
		specialisations, err := generation.CollectSpecialisations(constants.CurrentSystem)
		if err != nil {
			log.Warnf("unable to access specialisations: %v", err)
		}

		if !slices.Contains(specialisations, specialisation) {
			err = fmt.Errorf("specialisation '%v' does not exist", specialisations)
			log.Error(err)
			return err
		}
	}

	err := activation.SwitchToConfiguration(s, constants.CurrentSystem, action, &activation.SwitchToConfigurationOptions{
		Specialisation: specialisation,
	})

	return err
}

func runPreSwitchCheck(
	s system.CommandRunner,
	cmdStr string,
	toplevel string,
	action activation.SwitchToConfigurationAction,
) error {
	// TODO: would it be more appropriate to use shlex.Split() here?
	args := strings.Split(cmdStr, " ")
	args = append(args, toplevel)
	args = append(args, action.String())

	cmd := system.NewCommand(args[0], args[1:]...)
	_, err := s.Run(cmd)
	return err
}

func installBootloader(
	s system.CommandRunner,
	cmdStr string,
	toplevel string,
) error {
	// TODO: would it be more appropriate to use shlex.Split() here?
	args := strings.Split(cmdStr, " ")
	args = append(args, toplevel)

	cmd := system.NewCommand(args[0], args[1:]...)
	_, err := s.Run(cmd)
	return err
}

var ErrMismatchedInterfaceVersion = errors.New("this NixOS configuration has an init that is incompatible with the current configuration")

func validateInterfaceVersion(toplevel string) error {
	currentInitInterfaceVersionFile := filepath.Join(constants.CurrentSystem, "init-interface-version")
	newInitInterfaceVersionFile := filepath.Join(toplevel, "init-interface-version")

	currentInitInterfaceVersion, err := os.ReadFile(currentInitInterfaceVersionFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %v", currentInitInterfaceVersionFile, err)
	}

	newInitInterfaceVersion, err := os.ReadFile(newInitInterfaceVersionFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %v", newInitInterfaceVersionFile, err)
	}

	if string(currentInitInterfaceVersion) != string(newInitInterfaceVersion) {
		return ErrMismatchedInterfaceVersion
	}

	return nil
}

var errActivateScriptNotExist = errors.New("activate script does not exist")

func runActivateScript(toplevel string, dry bool) error {
	var script string
	if dry {
		script = filepath.Join(toplevel, "dry-activate")
	} else {
		script = filepath.Join(toplevel, "activate")
	}

	if _, err := os.Stat(script); errors.Is(err, os.ErrNotExist) {
		return errActivateScriptNotExist
	}

	cmd := exec.Command(script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

type unitAction string

const (
	actionStart   unitAction = "start"
	actionStop    unitAction = "stop"
	actionRestart unitAction = "restart"
	actionReload  unitAction = "reload"
)

// Run a specified action on a unit list, of either
// "start", "stop", "restart", or "reload".
//
// Returns a map of systemd job statuses for each unit
// name, with the following possible values:
//
// - "done" - success
// - "canceled" - context canceled before execution finished
// - "timeout" - job timeout reached
// - "failed" - job failed
// - "dependency" - a dependency failed, so this job was removed
// - "skipped" - action did not apply for unit, so nothing done
//
// along with a list of errors encountered
func runUnitAction(
	ctx context.Context,
	systemd *systemdDbus.Conn,
	units UnitList,
	action unitAction,
) (map[string]string, []error) {
	// Instead of using an errgroup, collect all errors
	// that result from this operation using an error channel..
	//
	// Errors will be displayed by invoking the systemctl binary
	// later, after all unit actions are finished.
	var wg sync.WaitGroup
	errCh := make(chan error, len(units))

	var mu sync.Mutex
	statuses := make(map[string]string, len(units))

	for unit := range units {
		wg.Go(func() {
			unit := unit
			ch := make(chan string, 1)

			var err error
			switch action {
			case actionReload:
				_, err = systemd.ReloadUnitContext(ctx, unit, "replace", ch)
			case actionRestart:
				_, err = systemd.RestartUnitContext(ctx, unit, "replace", ch)
			case actionStart:
				_, err = systemd.StartUnitContext(ctx, unit, "replace", ch)
			case actionStop:
				_, err = systemd.StopUnitContext(ctx, unit, "replace", ch)
			}

			if err != nil {
				errCh <- fmt.Errorf("failed to %v %s: %w", action, unit, err)
			}

			result := <-ch

			mu.Lock()
			statuses[unit] = result
		})
	}

	wg.Wait()
	close(errCh)

	errs := make([]error, 0, len(units))
	for err := range errCh {
		if err != nil {
			errs = append(errs, err)
		}
	}

	return statuses, errs
}

func activateMain(cmd *cobra.Command, opts *cmdOpts.ActivateOpts) error {
	log := logger.FromContext(cmd.Context())
	s := system.NewLocalSystem(log)

	if opts.Action == activation.SwitchToConfigurationActionDryActivate {
		recordUnits = false
	}

	if os.Geteuid() != 0 {
		err := fmt.Errorf("this command must be ran as root")
		log.Errorf("%s", err)
		return err
	}

	if attemptingActivation := os.Getenv("NIXOS_CLI_ATTEMPTING_ACTIVATION"); attemptingActivation == "" {
		err := execInSwitchContext(s, log, opts.Action, opts.Specialisation)
		if err != nil {
			log.Errorf("failed to re-execute switch-to-configuration script: %v", err)
		}

		return err
	}

	if !s.IsNixOS() {
		err := fmt.Errorf("the activate command is unsupported on non-NixOS systems")
		log.Error(err)
		return err
	}

	vars, err := getRequiredVars()
	if err != nil {
		log.Errorf("%s", err)
		return err
	}

	err = os.Setenv("NIXOS_ACTION", opts.Action.String())
	if err != nil {
		log.Errorf("failed to set NIXOS_ACTION variable: %s", err)
		return err
	}

	err = os.Setenv("LOCALE_ARCHIVE", vars.LocaleArchive)
	if err != nil {
		log.Errorf("failed to set LOCALE_ARCHIVE variable: %s", err)
		return err
	}

	err = os.MkdirAll("/run/nixos", 0o755)
	if err != nil {
		log.Errorf("failed to create /run/nixos: %s", err)
		return err
	}

	lockfile, err := os.OpenFile(ACTIVATION_LOCKFILE, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		log.Errorf("failed to create activation lockfile %s: %s", ACTIVATION_LOCKFILE, err)
		return err
	}
	defer func() { _ = lockfile.Close() }()

	if err := unix.Flock(int(lockfile.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		log.Errorf("failed to lock %s", ACTIVATION_LOCKFILE)
		log.Info("is another activation process running?")
		return err
	}
	defer func() { _ = unix.Flock(int(lockfile.Fd()), unix.LOCK_UN) }()

	// TODO: syslog init?

	if skipCheck := os.Getenv("NIXOS_NO_CHECK"); skipCheck == "" {
		log.Info("running pre-switch checks")

		err = runPreSwitchCheck(s, vars.PreSwitchCheckCmd, vars.Toplevel, opts.Action)
		if err != nil {
			log.Errorf("failed to run pre-switch check commands: %s", err)
			return err
		}
	}

	if opts.Action == activation.SwitchToConfigurationActionChecksOnly {
		return nil
	}

	if opts.Action == activation.SwitchToConfigurationActionBoot || opts.Action == activation.SwitchToConfigurationActionSwitch {
		log.Info("installing bootloader")

		if err := installBootloader(s, vars.InstallBootloaderCmd, vars.Toplevel); err != nil {
			log.Errorf("failed to install bootloader: %s", err)
			return err
		}
	}

	if skipSync := os.Getenv("NIXOS_NO_SYNC"); skipSync == "" {
		log.Info("syncing /nix/store to disk")

		dir, err := os.Open("/nix/store")
		if err != nil {
			log.Errorf("failed to sync /nix/store: %v", err)
			log.Info("will not proceed with activation")
			return err
		}
		defer func() { _ = dir.Close() }()

		if err := unix.Syncfs(int(dir.Fd())); err != nil {
			log.Errorf("failed to sync /nix/store: %v", err)
			log.Info("will not proceed with activation")
			return err
		}
	}

	if opts.Action == activation.SwitchToConfigurationActionBoot {
		return nil
	}

	if err = validateInterfaceVersion(vars.Toplevel); err != nil {
		log.Errorf("%v", err)
		if errors.Is(err, ErrMismatchedInterfaceVersion) {
			log.Info("the new configuration won't take effect until you reboot the system")
		}
		return err
	}

	// Prevent this process from getting killed if running
	// in a TTY and tty* systemd unit(s) are restarted.
	signal.Ignore(syscall.SIGHUP)

	ctx := context.Background()

	systemd, err := systemdDbus.NewWithContext(ctx)
	if err != nil {
		log.Errorf("failed to initialize systemd dbus connection: %v", err)
		return err
	}
	defer systemd.Close()

	unitLists := makeUnitLists(vars.Toplevel)

	currentActiveUnits, err := getActiveUnits(ctx, systemd)
	if err != nil {
		return fmt.Errorf("failed to get active units: %s", err)
	}

	err = unitLists.ClassifyActiveUnits(ctx, currentActiveUnits)
	if err != nil {
		log.Errorf("%v", err)
		return err
	}

	currentFilesystems, currentSwapDevices, _ := parseFstab("/etc/fstab")
	newFilesystems, newSwapDevices, _ := parseFstab(filepath.Join(vars.Toplevel, "/etc/fstab"))

	unitLists.ClassifyFilesystemUnits(currentFilesystems, newFilesystems)

	for device := range currentSwapDevices {
		if _, ok := newSwapDevices[device]; !ok {
			// The swap entry has disappeared, so turn it off.
			//
			// Can't use "systemctl stop" here, since systemd has lots
			// of alias units that prevent a stop from actually calling
			// "swapoff", so we instead invoke the syscall ourselves.
			if opts.Action == activation.SwitchToConfigurationActionDryActivate {
				log.Infof("would stop swap device %s", device)
			} else {
				log.Infof("stopping swap device %s", device)
				err := swapoff(device)
				if err != nil {
					log.Warnf("failed to stop swapping to device %s, continuing activation anyway", device)
				}
			}
		}

		// FIXME: update swap options (i.e. its priority).
	}

	currentPID1Path, err := filepath.EvalSymlinks("/proc/1/exe")
	if err != nil {
		currentPID1Path = "/unknown"
	}

	newPID1Path, err := filepath.EvalSymlinks(filepath.Join(vars.NewSystemd, "lib/systemd/systemd"))
	if err != nil {
		err := fmt.Errorf("systemd binary in this system does not exist, cannot continue")
		log.Errorf("%s", err)
		return err
	}

	currentSystemdSystemConfig, err := filepath.EvalSymlinks("/etc/systemd/system.conf")
	if err != nil {
		currentSystemdSystemConfig = "/unknown"
	}

	newSystemdSystemConfig, err := filepath.EvalSymlinks(filepath.Join(vars.Toplevel, "etc/systemd/system.conf"))
	if err != nil {
		newSystemdSystemConfig = "/unknown"
	}

	restartSystemd := currentPID1Path != newPID1Path || currentSystemdSystemConfig != newSystemdSystemConfig

	if opts.Action == activation.SwitchToConfigurationActionDryActivate {
		unitsToStop := unitLists.Stop.Filter(unitLists.Filter)
		if len(unitsToStop) > 0 {
			log.Infof("would stop the following units: %s", strings.Join(unitsToStop.Sorted(), ", "))
		}

		if len(unitLists.Skip) > 0 {
			log.Infof("would NOT stop the following changed units: %s", strings.Join(unitLists.Skip.Sorted(), ", "))
		}

		log.Info("would run activate script...")

		// If the dry activate script fails, don't stop printing output
		// and just ignore the errors.
		err = runActivateScript(vars.Toplevel, true)
		if err != nil && !errors.Is(err, errActivateScriptNotExist) {
			log.Warnf("running activation script failed: %s", err)
		}

		dryRestartUnits := readUnitsListFile(DRY_RESTART_BY_ACTIVATION_LIST_FILE)
		for unit := range dryRestartUnits {
			resolvedUnit := resolveUnit(unit, vars.Toplevel)

			if _, ok := currentActiveUnits[unit]; !ok {
				unitLists.Start.Add(unit)
				continue
			}

			err = unitLists.ClassifyModifiedUnit(
				unit,
				resolvedUnit.BaseName,
				resolvedUnit.NewUnitFile,
				resolvedUnit.NewBaseFile,
				nil,
				currentActiveUnits,
			)
			if err != nil {
				log.Errorf("failed to classify unit %s: %s", unit, err)
				return err
			}
		}

		err := os.RemoveAll(DRY_RESTART_BY_ACTIVATION_LIST_FILE)
		if err != nil {
			log.Warnf("failed to remove %s, please remove manually to prevent problems with future activations", DRY_RESTART_BY_ACTIVATION_LIST_FILE)
		}

		dryReloadUnits := readUnitsListFile(DRY_RELOAD_BY_ACTIVATION_LIST_FILE)
		for unit := range dryReloadUnits {
			if _, ok := currentActiveUnits[unit]; !ok {
				if !unitLists.Restart.Has(unit) && !unitLists.Stop.Has(unit) {
					unitLists.Reload.Add(unit)
				}
			}
		}

		err = os.RemoveAll(DRY_RELOAD_BY_ACTIVATION_LIST_FILE)
		if err != nil {
			log.Warnf("failed to remove %s, please remove manually to prevent problems with future activations", DRY_RELOAD_BY_ACTIVATION_LIST_FILE)
		}

		if restartSystemd {
			log.Info("would restart systemd")
		}

		if len(unitLists.Reload) > 0 {
			log.Infof("would reload the following units: %s", strings.Join(unitLists.Reload.Sorted(), ", "))
		}

		if len(unitLists.Restart) > 0 {
			log.Infof("would restart the following units: %s", strings.Join(unitLists.Restart.Sorted(), ", "))
		}

		unitsToStart := unitLists.Start.Filter(unitLists.Filter)
		if len(unitsToStart) > 0 {
			log.Infof("would start the following units: %s", strings.Join(unitsToStart.Sorted(), ", "))
		}

		return nil
	}

	log.Info("switching to system configuration")

	serviceStatuses := make(map[string]string)

	if len(unitLists.Stop) > 0 {
		filteredUnits := unitLists.Stop.Filter(unitLists.Filter)
		if len(filteredUnits) > 0 {
			log.Infof("stopping the following units: %s", strings.Join(filteredUnits.Sorted(), ", "))
		}

		statuses, _ := runUnitAction(ctx, systemd, unitLists.Stop, actionStop)
		maps.Copy(serviceStatuses, statuses)
	}

	if len(unitLists.Skip) > 0 {
		log.Infof("NOT restarting the following changed units: %s", strings.Join(unitLists.Skip.Sorted(), ", "))
	}

	exitCode := 0

	log.Info("running activation script")
	err = runActivateScript(vars.Toplevel, false)
	if err != nil && !errors.Is(err, errActivateScriptNotExist) {
		log.Errorf("failed to run activate script: %v", err)
		exitCode = 2
	}

	activateRestartUnits := readUnitsListFile(RESTART_BY_ACTIVATION_LIST_FILE)
	for unit := range activateRestartUnits {
		resolvedUnit := resolveUnit(unit, vars.Toplevel)

		if _, ok := currentActiveUnits[unit]; !ok {
			unitLists.Start.Add(unit)
			continue
		}

		err = unitLists.ClassifyModifiedUnit(
			unit,
			resolvedUnit.BaseName,
			resolvedUnit.NewUnitFile,
			resolvedUnit.NewBaseFile,
			nil,
			currentActiveUnits,
		)
		if err != nil {
			log.Errorf("failed to classify unit %s: %s", unit, err)
			return err
		}
	}

	err = os.RemoveAll(RESTART_BY_ACTIVATION_LIST_FILE)
	if err != nil {
		log.Warnf("failed to remove %s, please remove manually to prevent problems with future activations", DRY_RELOAD_BY_ACTIVATION_LIST_FILE)
	}

	activateReloadUnits := readUnitsListFile(RELOAD_BY_ACTIVATION_LIST_FILE)
	for unit := range activateReloadUnits {
		if _, ok := currentActiveUnits[unit]; !ok {
			if !unitLists.Restart.Has(unit) && !unitLists.Stop.Has(unit) {
				unitLists.Reload.Add(unit)
			}
		}
	}

	err = os.RemoveAll(RELOAD_BY_ACTIVATION_LIST_FILE)
	if err != nil {
		log.Warnf("failed to remove %s, please remove manually to prevent problems with future activations", RELOAD_BY_ACTIVATION_LIST_FILE)
	}

	// Restart systemd if necessary. Note that this is done using the
	// current version of systemd, just in case the new one has trouble
	// communicating with the running pid 1.
	//
	// Basically the equivalent of `systemd daemon-reexec`.
	if restartSystemd {
		// A reply will not be received, so ignore errors.
		_ = systemd.ReexecuteContext(ctx)
	}

	// Forget about previously failed services and reload the
	// available systemd units; basically `systemctl daemon-reload`.
	_ = systemd.ReloadContext(ctx)

	// TODO: figure out way to exit gracefully with correct error code
	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}
