//go:build linux

package activate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	shlex "github.com/carapace-sh/carapace-shlex"
	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/coreos/go-systemd/v22/login1"

	"github.com/nix-community/nixos-cli/internal/activation"
	cmdOpts "github.com/nix-community/nixos-cli/internal/cmd/opts"
	"github.com/nix-community/nixos-cli/internal/constants"
	"github.com/nix-community/nixos-cli/internal/generation"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/system"
	systemdUtils "github.com/nix-community/nixos-cli/internal/systemd"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

const (
	ACTIVATION_LOCKFILE = "/run/nixos/switch-to-configuration.lock"

	DRY_RESTART_BY_ACTIVATION_LIST_FILE = "/run/nixos/dry-activation-restart-list"
	DRY_RELOAD_BY_ACTIVATION_LIST_FILE  = "/run/nixos/dry-activation-reload-list"
	RELOAD_BY_ACTIVATION_LIST_FILE      = "/run/nixos/activation-reload-list"
	RESTART_BY_ACTIVATION_LIST_FILE     = "/run/nixos/activation-restart-list"

	NIXOS_STC_PARENT_EXE = "__NIXOS_SWITCH_TO_CONFIGURATION_PARENT_EXE"

	SYSINIT_REACTIVATION_TARGET = "sysinit-reactivation.target"
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
	log logger.Logger,
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
	args, err := shlex.Split(cmdStr)
	if err != nil {
		return err
	}

	argv := args.Strings()
	argv = append(argv, toplevel)
	argv = append(argv, action.String())

	cmd := system.NewCommand(argv[0], argv[1:]...)
	_, err = s.Run(cmd)
	return err
}

func installBootloader(
	s system.CommandRunner,
	cmdStr string,
	toplevel string,
) error {
	args, err := shlex.Split(cmdStr)
	if err != nil {
		return err
	}

	argv := args.Strings()
	argv = append(argv, toplevel)

	cmd := system.NewCommand(argv[0], argv[1:]...)
	_, err = s.Run(cmd)
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

type unitActionResult struct {
	Action unitAction
	Unit   string
	Result string
	Err    error
}

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
) []unitActionResult {
	var wg sync.WaitGroup

	results := make(chan unitActionResult, len(units))

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

			select {
			case result := <-ch:
				results <- unitActionResult{
					Action: action,
					Unit:   unit,
					Result: result,
					Err:    err,
				}
			case <-ctx.Done():
			}
		})
	}

	wg.Wait()
	close(results)

	collected := make([]unitActionResult, 0, len(units))
	for result := range results {
		collected = append(collected, result)
	}

	return collected
}

func waitForSystemdToSettle(systemd *systemdDbus.Conn, idleTimeout time.Duration, maxTimeout time.Duration) {
	changes, _ := systemd.SubscribeUnits(idleTimeout)

	idleTimer := time.NewTimer(idleTimeout)
	overallTimer := time.NewTimer(maxTimeout)

	for {
		select {
		case <-changes:
			if !idleTimer.Stop() {
				<-idleTimer.C
			}
			idleTimer.Reset(idleTimeout)

		case <-idleTimer.C:
			return

		case <-overallTimer.C:
			return
		}
	}
}

func activateMain(cmd *cobra.Command, opts *cmdOpts.ActivateOpts) error {
	log := logger.FromContext(cmd.Context())
	s := system.NewLocalSystem(log)

	if !s.IsNixOS() {
		err := fmt.Errorf("the activate command is unsupported on non-NixOS systems")
		log.Error(err)
		return err
	}

	if opts.Action == activation.SwitchToConfigurationActionDryActivate {
		recordUnits = false
	}

	if parentExe := os.Getenv(NIXOS_STC_PARENT_EXE); parentExe != "" {
		return userSwitch(log, parentExe)
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

	if syslogLogger, err := logger.NewSyslogLogger("nixos-cli-activate"); err == nil {
		log = logger.NewMultiLogger(log, syslogLogger)
	} else {
		log.Warnf("failed to initialize syslog logger: %v", err)
	}

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
		os.Exit(100)
	}

	// Prevent this process from getting killed if running
	// in a TTY and tty* systemd unit(s) are restarted.
	signal.Ignore(syscall.SIGHUP)

	// Also, make sure termination is done gracefully.
	//
	// This is a three-tier warning system: the first cancellation
	// results in a warning, while the second results in a graceful
	// shutdown, and any subsequent ones will abort the process
	// forcefully.
	ctx, cancel := context.WithCancel(context.Background())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	var signalCount uint64 = 0
	go func() {
		for range signals {
			count := atomic.AddUint64(&signalCount, 1)
			switch count {
			case 1:
				log.Warnf("received cancel signal; interruption of this program can result in an unexpected system state until reboot")
				log.Infof("signal again to gracefully shutdown")
			case 2:
				log.Warnf("received cancel signal again: cancelling gracefully...")
				cancel()
			default:
				log.Errorf("received cancel signal a third time: aborting immediately!")
				os.Exit(1)
			}
		}
	}()

	systemd, err := systemdDbus.NewWithContext(ctx)
	if err != nil {
		log.Errorf("failed to initialize systemd dbus connection: %v", err)
		return err
	}
	defer systemd.Close()

	logind, err := login1.New()
	if err != nil {
		log.Errorf("failed to initialize logind system dbus connection: %v", err)
	}
	defer logind.Close()

	unitLists := makeUnitLists(vars.Toplevel)

	currentActiveUnits, err := getActiveUnits(ctx, systemd)
	if err != nil {
		log.Errorf("failed to get active units: %s", err)
		return err
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

		_ = os.RemoveAll(DRY_RESTART_BY_ACTIVATION_LIST_FILE)

		dryReloadUnits := readUnitsListFile(DRY_RELOAD_BY_ACTIVATION_LIST_FILE)
		for unit := range dryReloadUnits {
			if _, ok := currentActiveUnits[unit]; !ok {
				if !unitLists.Restart.Has(unit) && !unitLists.Stop.Has(unit) {
					unitLists.Reload.Add(unit)
				}
			}
		}

		_ = os.RemoveAll(DRY_RELOAD_BY_ACTIVATION_LIST_FILE)

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

	unitStatuses := make([]unitActionResult, 0)

	if len(unitLists.Stop) > 0 {
		filteredUnits := unitLists.Stop.Filter(unitLists.Filter)
		if len(filteredUnits) > 0 {
			log.Infof("stopping the following units: %s", strings.Join(filteredUnits.Sorted(), ", "))
		}

		statuses := runUnitAction(ctx, systemd, unitLists.Stop, actionStop)
		unitStatuses = append(unitStatuses, statuses...)
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

	_ = os.RemoveAll(RESTART_BY_ACTIVATION_LIST_FILE)

	activateReloadUnits := readUnitsListFile(RELOAD_BY_ACTIVATION_LIST_FILE)
	for unit := range activateReloadUnits {
		if _, ok := currentActiveUnits[unit]; !ok {
			if !unitLists.Restart.Has(unit) && !unitLists.Stop.Has(unit) {
				unitLists.Reload.Add(unit)
			}
		}
	}

	_ = os.RemoveAll(RELOAD_BY_ACTIVATION_LIST_FILE)

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

	users, err := logind.ListUsersContext(ctx)
	if err != nil {
		log.Errorf("failed to list users using logind: %s", err)
		return err
	}
	for _, user := range users {
		userProps, _ := logind.GetUserPropertiesContext(ctx, user.Path)

		var gid uint32
		err := userProps["GID"].Store(&gid)
		if err != nil {
			log.Warnf("failed to get GID for user %s, skipping", user.Name)
			continue
		}

		runtimePath := userProps["RuntimePath"].String()

		log.Infof("reloading units for user %s...", user.Name)

		err = execUserSwitchProcess(user.UID, gid, runtimePath)
		if err != nil {
			log.Errorf("failed to run user switch for user %s: %v", user.Name, err)
			return err
		}
	}

	// Restart sysinit-reactivation.target. This target only exists to
	// restart services ordered before sysinit.target. We cannot use
	// X-StopOnReconfiguration to restart sysinit.target because then
	// ALL services of the system would be restarted since all normal
	// services have a default dependency on sysinit.target.
	//
	// sysinit-reactivation.target ensures that services ordered BEFORE
	// sysinit.target get re-started in the correct order. Ordering between
	// these services is respected.
	log.Infof("restarting %s", SYSINIT_REACTIVATION_TARGET)

	sysinitReactivationStatus := make(chan string, 1)
	_, err = systemd.RestartUnitContext(ctx, SYSINIT_REACTIVATION_TARGET, "replace", sysinitReactivationStatus)
	if err != nil {
		log.Errorf("failed to restart %s: %s", SYSINIT_REACTIVATION_TARGET, err)
		exitCode = 4
	}
	unitStatuses = append(unitStatuses, unitActionResult{
		Action: actionRestart,
		Unit:   SYSINIT_REACTIVATION_TARGET,
		Result: <-sysinitReactivationStatus,
		Err:    err,
	})

	// Before reloading we need to ensure that the units are still active.
	//
	// They may have been deactivated because one of their requirements got
	// stopped. If they are inactive but should have been reloaded, the user
	// probably expects them to be started.
	if len(unitLists.Reload) > 0 {
		for unit := range unitLists.Reload {
			active, err := unitIsActive(ctx, systemd, unit)
			if err != nil {
				log.Errorf("failed to get state of unit %s: %v", unit, err)
				continue
			}

			if active {
				continue
			}

			unitPath := filepath.Join(vars.Toplevel, "etc/systemd/system", unit)
			unitInfo, err := systemdUtils.ParseUnit(unitPath, unitPath)
			if err != nil {
				log.Errorf("failed to parse unit file %s: %s", unitPath, err)
				continue
			}

			if !unitInfo.GetBoolean("Unit", "RefuseManualStart", false) &&
				!unitInfo.GetBoolean("Unit", "X-OnlyManualStart", false) {
				unitLists.Start.Add(unit)
				recordUnit(START_LIST_FILE, unit)
			}

			// Don't reload the unit, reloading would fail in this case.
			unitLists.Reload.Remove(unit)
			unrecordUnit(RELOAD_LIST_FILE, unit)
		}
	}

	// Reload units that need it.
	// This includes remounting changed mount units.
	if len(unitLists.Reload) > 0 {
		filteredUnits := unitLists.Reload.Filter(unitLists.Filter)
		if len(filteredUnits) > 0 {
			log.Infof("reloading the following units: %s", strings.Join(filteredUnits.Sorted(), ", "))
		}

		statuses := runUnitAction(ctx, systemd, unitLists.Reload, actionReload)
		unitStatuses = append(unitStatuses, statuses...)
	}
	_ = os.RemoveAll(RELOAD_LIST_FILE)

	// Restart changed services (aka those that have to be restarted,
	// rather than stopped and started).
	if len(unitLists.Restart) > 0 {
		filteredUnits := unitLists.Restart.Filter(unitLists.Filter)
		if len(filteredUnits) > 0 {
			log.Infof("restarting the following units: %s", strings.Join(filteredUnits.Sorted(), ", "))
		}

		statuses := runUnitAction(ctx, systemd, unitLists.Restart, actionRestart)
		unitStatuses = append(unitStatuses, statuses...)
	}
	_ = os.RemoveAll(RESTART_LIST_FILE)

	// Start all active targets, as well as changed units we stopped above.
	//
	// The latter is necessary because some may not be dependencies of the
	// targets (i.e. they were manually started).
	//
	// FIXME: detect units that are symlinks to other units. We shouldn't
	// start both at the same time because we'll get a "Failed to add path
	// to set" error from systemd.
	if len(unitLists.Start) > 0 {
		filteredUnits := unitLists.Start.Filter(unitLists.Filter)
		if len(filteredUnits) > 0 {
			log.Infof("starting the following units: %s", strings.Join(filteredUnits.Sorted(), ", "))
		}

		statuses := runUnitAction(ctx, systemd, unitLists.Start, actionStart)
		unitStatuses = append(unitStatuses, statuses...)
	}
	_ = os.RemoveAll(START_LIST_FILE)

	for _, s := range unitStatuses {
		switch s.Result {
		case "timeout", "failed", "dependency":
			log.Warnf("failed to %s %s", s.Action, s.Unit)
			exitCode = 4
		}

		if s.Err != nil {
			log.Warnf("service error for %s: %v", s.Unit, s.Err)
			exitCode = 4
		}
	}

	log.Info("waiting for systemd events to settle")
	waitForSystemdToSettle(systemd, 250*time.Millisecond, 90*time.Second)

	newActiveUnits, err := getActiveUnits(ctx, systemd)
	if err != nil {
		log.Errorf("failed to get new active units: %s", err)
		return err
	}

	failedUnits := make(UnitList)
	newUnits := make(UnitList)

	for unit, state := range newActiveUnits {
		if state.State == "failed" {
			failedUnits.Add(unit)
			continue
		}

		if state.SubState == "auto-restart" && strings.HasSuffix(unit, ".service") {
			prop, err := systemd.GetUnitTypePropertyContext(ctx, unit, "Service", "ExecMainStatus")
			if err != nil {
				log.Errorf("failed to get ExecMainStatus property for %s: %v", unit, err)
				continue
			}

			var execMainStatus int32
			err = prop.Value.Store(&execMainStatus)
			if err != nil {
				log.Errorf("failed to convert ExecMainStatus prop value to int32: %v", err)
			}

			if execMainStatus != 0 {
				failedUnits.Add(unit)
				continue
			}
		}

		if state.State != "failed" && strings.HasSuffix(unit, ".scope") {
			if _, exists := currentActiveUnits[unit]; !exists {
				newUnits.Add(unit)
			}
		}
	}

	if len(newUnits) > 0 {
		log.Infof("the following new units were started: %s", strings.Join(newUnits.Sorted(), ", "))
	}

	if len(failedUnits) > 0 {
		units := failedUnits.Sorted()
		log.Warnf("the following units failed: %s", strings.Join(units, ", "))

		systemctl := filepath.Join(vars.NewSystemd, "bin/systemctl")

		argv := []string{systemctl, "status", "--no-pager", "--full"}
		argv = append(argv, units...)

		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		_ = cmd.Run()

		exitCode = 4
	}

	if exitCode != 0 {
		log.Errorf("switching to system configuration failed (status %d)", exitCode)
		os.Exit(exitCode)
	} else {
		log.Info("finished switching to system configuration")
	}

	return nil
}
