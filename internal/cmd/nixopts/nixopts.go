package nixopts

import (
	"github.com/spf13/cobra"
)

type NixOption string

const (
	NixOptionQuiet                   NixOption = "quiet"
	NixOptionPrintBuildLogs          NixOption = "print-build-logs"
	NixOptionNoBuildOutput           NixOption = "no-build-output"
	NixOptionShowTrace               NixOption = "show-trace"
	NixOptionKeepGoing               NixOption = "keep-going"
	NixOptionKeepFailed              NixOption = "keep-failed"
	NixOptionFallback                NixOption = "fallback"
	NixOptionRefresh                 NixOption = "refresh"
	NixOptionRepair                  NixOption = "repair"
	NixOptionImpure                  NixOption = "impure"
	NixOptionOffline                 NixOption = "offline"
	NixOptionNoNet                   NixOption = "no-net"
	NixOptionInclude                 NixOption = "include"
	NixOptionMaxJobs                 NixOption = "max-jobs"
	NixOptionCores                   NixOption = "cores"
	NixOptionBuilders                NixOption = "builders"
	NixOptionLogFormat               NixOption = "log-format"
	NixOptionOption                  NixOption = "option"
	NixOptionUseSubstitutes          NixOption = "use-substitutes"
	NixOptionSubstituteOnDestination NixOption = "substitute-on-destination"
	NixOptionRecreateLockFile        NixOption = "recreate-lock-file"
	NixOptionNoUpdateLockFile        NixOption = "no-update-lock-file"
	NixOptionNoWriteLockFile         NixOption = "no-write-lock-file"
	NixOptionNoUseRegistries         NixOption = "no-use-registries"
	NixOptionNoRegistries            NixOption = "no-registries"
	NixOptionCommitLockFile          NixOption = "commit-lock-file"
	NixOptionUpdateInput             NixOption = "update-input"
	NixOptionOverrideInput           NixOption = "override-input"
)

func addNixOptionBool(cmd *cobra.Command, dest *bool, name NixOption, shorthand string, desc string) {
	flag := string(name)
	if shorthand != "" {
		cmd.Flags().BoolVarP(dest, flag, shorthand, false, desc)
	} else {
		cmd.Flags().BoolVar(dest, flag, false, desc)
	}
	cmd.Flags().Lookup(flag).Hidden = true
}

func addNixOptionInt(cmd *cobra.Command, dest *int, name NixOption, shorthand string, desc string) {
	flag := string(name)
	if shorthand != "" {
		cmd.Flags().IntVarP(dest, flag, shorthand, 0, desc)
	} else {
		cmd.Flags().IntVar(dest, flag, 0, desc)
	}
	cmd.Flags().Lookup(flag).Hidden = true
}

func addNixOptionString(cmd *cobra.Command, dest *string, name NixOption, shorthand string, desc string) {
	flag := string(name)
	if shorthand != "" {
		cmd.Flags().StringVarP(dest, flag, shorthand, "", desc)
	} else {
		cmd.Flags().StringVar(dest, flag, "", desc)
	}
	cmd.Flags().Lookup(flag).Hidden = true
}

func addNixOptionStringArray(cmd *cobra.Command, dest *[]string, name NixOption, shorthand string, desc string) {
	flag := string(name)
	if shorthand != "" {
		cmd.Flags().StringSliceVarP(dest, flag, shorthand, nil, desc)
	} else {
		cmd.Flags().StringSliceVar(dest, flag, nil, desc)
	}
	cmd.Flags().Lookup(flag).Hidden = true
}

func addNixOptionStringMap(cmd *cobra.Command, dest *map[string]string, name NixOption, shorthand string, desc string) {
	flag := string(name)
	if shorthand != "" {
		cmd.Flags().StringToStringVarP(dest, flag, shorthand, nil, desc)
	} else {
		cmd.Flags().StringToStringVar(dest, flag, nil, desc)
	}
	cmd.Flags().Lookup(flag).Hidden = true
}

func AddQuietNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, NixOptionQuiet, "", "Decrease logging verbosity level")
}

func AddPrintBuildLogsNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, NixOptionPrintBuildLogs, "L", "Decrease logging verbosity level")
}

func AddNoBuildOutputNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, NixOptionNoBuildOutput, "Q", "Silence build output on stdout and stderr")
}

func AddShowTraceNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, NixOptionShowTrace, "", "Print stack trace of evaluation errors")
}

func AddKeepGoingNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, NixOptionKeepGoing, "k", "Keep going until all builds are finished despite failures")
}

func AddKeepFailedNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, NixOptionKeepFailed, "K", "Keep failed builds (usually in /tmp)")
}

func AddFallbackNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, NixOptionFallback, "", "If binary download fails, fall back on building from source")
}

func AddRefreshNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, NixOptionRefresh, "", "Consider all previously downloaded files out-of-date")
}

func AddRepairNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, NixOptionRepair, "", "Fix corrupted or missing store paths")
}

func AddImpureNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, NixOptionImpure, "", "Allow access to mutable paths and repositories")
}

func AddOfflineNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, NixOptionOffline, "", "Disable substituters and consider all previously downloaded files up-to-date.")
}

func AddNoNetNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, NixOptionNoNet, "", "Disable substituters and set all network timeout settings to minimum")
}

func AddIncludesNixOption(cmd *cobra.Command, dest *[]string) {
	addNixOptionStringArray(cmd, dest, NixOptionInclude, "I", "Add path to list of locations to look up <...> file names")
}

func AddMaxJobsNixOption(cmd *cobra.Command, dest *int) {
	addNixOptionInt(cmd, dest, NixOptionMaxJobs, "j", "Max number of build jobs in parallel")
}

func AddCoresNixOption(cmd *cobra.Command, dest *int) {
	addNixOptionInt(cmd, dest, NixOptionCores, "", "Max number of CPU cores used (sets NIX_BUILD_CORES env variable)")
}

func AddBuildersNixOption(cmd *cobra.Command, dest *[]string) {
	addNixOptionStringArray(cmd, dest, NixOptionBuilders, "", "List of Nix remote builder addresses")
}

func AddLogFormatNixOption(cmd *cobra.Command, dest *string) {
	addNixOptionString(cmd, dest, NixOptionLogFormat, "", "Configure how output is formatted")
}

func AddOptionNixOption(cmd *cobra.Command, dest *map[string]string) {
	addNixOptionStringMap(cmd, dest, NixOptionOption, "", "Set Nix config option (passed as 1 arg, requires = separator)")
}

func AddSubstituteOnDestinationNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, NixOptionSubstituteOnDestination, "", "Let remote machine substitute missing store paths")
	addNixOptionBool(cmd, dest, NixOptionUseSubstitutes, "", "Let remote machine substitute missing store paths")
	cmd.Flags().Lookup(string(NixOptionUseSubstitutes)).Deprecated = string(NixOptionSubstituteOnDestination)
}

func AddRecreateLockFileNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, NixOptionRecreateLockFile, "", "Recreate the flake's lock file from scratch")
}

func AddNoUpdateLockFileNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, NixOptionNoUpdateLockFile, "", "Do not allow any updates to the flake's lock file")
}

func AddNoWriteLockFileNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, NixOptionNoWriteLockFile, "", "Do not write the flake's newly generated lock file")
}

func AddNoUseRegistriesNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, NixOptionNoUseRegistries, "", "Don't allow lookups in the flake registries")
	addNixOptionBool(cmd, dest, NixOptionNoRegistries, "", "Don't allow lookups in the flake registries")
	cmd.Flags().Lookup(string(NixOptionNoRegistries)).Deprecated = string(NixOptionNoUseRegistries)
}

func AddCommitLockFileNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, NixOptionCommitLockFile, "", "Commit changes to the flake's lock file")
}

func AddUpdateInputNixOption(cmd *cobra.Command, dest *[]string) {
	addNixOptionStringArray(cmd, dest, NixOptionUpdateInput, "", "Update a specific flake input")
}

func AddOverrideInputNixOption(cmd *cobra.Command, dest *map[string]string) {
	addNixOptionStringMap(cmd, dest, NixOptionOverrideInput, "", "Override a specific flake input (passed as 1 arg, requires = separator)")
}
