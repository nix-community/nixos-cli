package nixopts

import (
	"github.com/spf13/cobra"
)

func addNixOptionBool(cmd *cobra.Command, dest *bool, name string, shorthand string, desc string) {
	if shorthand != "" {
		cmd.Flags().BoolVarP(dest, name, shorthand, false, desc)
	} else {
		cmd.Flags().BoolVar(dest, name, false, desc)
	}
	cmd.Flags().Lookup(name).Hidden = true
}

func addNixOptionInt(cmd *cobra.Command, dest *int, name string, shorthand string, desc string) {
	if shorthand != "" {
		cmd.Flags().IntVarP(dest, name, shorthand, 0, desc)
	} else {
		cmd.Flags().IntVar(dest, name, 0, desc)
	}
	cmd.Flags().Lookup(name).Hidden = true
}

func addNixOptionString(cmd *cobra.Command, dest *string, name string, shorthand string, desc string) {
	if shorthand != "" {
		cmd.Flags().StringVarP(dest, name, shorthand, "", desc)
	} else {
		cmd.Flags().StringVar(dest, name, "", desc)
	}
	cmd.Flags().Lookup(name).Hidden = true
}

func addNixOptionStringArray(cmd *cobra.Command, dest *[]string, name string, shorthand string, desc string) {
	if shorthand != "" {
		cmd.Flags().StringSliceVarP(dest, name, shorthand, nil, desc)
	} else {
		cmd.Flags().StringSliceVar(dest, name, nil, desc)
	}
	cmd.Flags().Lookup(name).Hidden = true
}

func addNixOptionStringMap(cmd *cobra.Command, dest *map[string]string, name string, shorthand string, desc string) {
	if shorthand != "" {
		cmd.Flags().StringToStringVarP(dest, name, shorthand, nil, desc)
	} else {
		cmd.Flags().StringToStringVar(dest, name, nil, desc)
	}
	cmd.Flags().Lookup(name).Hidden = true
}

func AddQuietNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, "quiet", "", "Decrease logging verbosity level")
}

func AddPrintBuildLogsNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, "print-build-logs", "L", "Decrease logging verbosity level")
}

func AddNoBuildOutputNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, "no-build-output", "Q", "Silence build output on stdout and stderr")
}

func AddShowTraceNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, "show-trace", "", "Print stack trace of evaluation errors")
}

func AddKeepGoingNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, "keep-going", "k", "Keep going until all builds are finished despite failures")
}

func AddKeepFailedNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, "keep-failed", "K", "Keep failed builds (usually in /tmp)")
}

func AddFallbackNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, "fallback", "", "If binary download fails, fall back on building from source")
}

func AddRefreshNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, "refresh", "", "Consider all previously downloaded files out-of-date")
}

func AddRepairNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, "repair", "", "Fix corrupted or missing store paths")
}

func AddImpureNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, "impure", "", "Allow access to mutable paths and repositories")
}

func AddOfflineNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, "offline", "", "Disable substituters and consider all previously downloaded files up-to-date.")
}

func AddNoNetNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, "no-net", "", "Disable substituters and set all network timeout settings to minimum")
}

func AddIncludesNixOption(cmd *cobra.Command, dest *[]string) {
	addNixOptionStringArray(cmd, dest, "include", "I", "Add path to list of locations to look up <...> file names")
}

func AddMaxJobsNixOption(cmd *cobra.Command, dest *int) {
	addNixOptionInt(cmd, dest, "max-jobs", "j", "Max number of build jobs in parallel")
}

func AddCoresNixOption(cmd *cobra.Command, dest *int) {
	addNixOptionInt(cmd, dest, "cores", "", "Max number of CPU cores used (sets NIX_BUILD_CORES env variable)")
}

func AddBuildersNixOption(cmd *cobra.Command, dest *[]string) {
	addNixOptionStringArray(cmd, dest, "builders", "", "List of Nix remote builder addresses")
}

func AddLogFormatNixOption(cmd *cobra.Command, dest *string) {
	addNixOptionString(cmd, dest, "log-format", "", "Configure how output is formatted")
}

func AddOptionNixOption(cmd *cobra.Command, dest *map[string]string) {
	addNixOptionStringMap(cmd, dest, "option", "", "Set Nix config option (passed as 1 arg, requires = separator)")
}

func AddRecreateLockFileNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, "recreate-lock-file", "", "Recreate the flake's lock file from scratch")
}

func AddNoUpdateLockFileNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, "no-update-lock-file", "", "Do not allow any updates to the flake's lock file")
}

func AddNoWriteLockFileNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, "no-write-lock-file", "", "Do not write the flake's newly generated lock file")
}

func AddNoUseRegistriesNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, "no-use-registries", "", "Don't allow lookups in the flake registries")
	addNixOptionBool(cmd, dest, "no-registries", "", "Don't allow lookups in the flake registries")
	// TODO: add deprecation notice for --no-registries?
}

func AddCommitLockFileNixOption(cmd *cobra.Command, dest *bool) {
	addNixOptionBool(cmd, dest, "commit-lock-file", "", "Commit changes to the flake's lock file")
}

func AddUpdateInputNixOption(cmd *cobra.Command, dest *[]string) {
	addNixOptionStringArray(cmd, dest, "update-input", "", "Update a specific flake input")
}

func AddOverrideInputNixOption(cmd *cobra.Command, dest *map[string]string) {
	addNixOptionStringMap(cmd, dest, "override-input", "", "Override a specific flake input (passed as 1 arg, requires = separator)")
}
