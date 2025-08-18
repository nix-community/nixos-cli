package cmdOpts

import "github.com/nix-community/nixos-cli/internal/configuration"

type MainOpts struct {
	ColorAlways  bool
	ConfigValues map[string]string
}

type AliasesOpts struct {
	DisplayJson bool
}

type ApplyOpts struct {
	Dry                   bool
	InstallBootloader     bool
	NoActivate            bool
	NoBoot                bool
	OutputPath            string
	ProfileName           string
	Specialisation        string
	GenerationTag         string
	UpgradeChannels       bool
	UpgradeAllChannels    bool
	UseNom                bool
	Verbose               bool
	BuildVM               bool
	BuildVMWithBootloader bool
	AlwaysConfirm         bool
	FlakeRef              string

	NixOptions ApplyNixOptions
}

type ApplyNixOptions struct {
	Quiet          bool
	PrintBuildLogs bool
	NoBuildOutput  bool
	ShowTrace      bool
	KeepGoing      bool
	KeepFailed     bool
	Fallback       bool
	Refresh        bool
	Repair         bool
	Impure         bool
	Offline        bool
	NoNet          bool
	MaxJobs        int
	Cores          int
	Builders       []string
	LogFormat      string
	Includes       []string
	Options        map[string]string

	RecreateLockFile bool
	NoUpdateLockFile bool
	NoWriteLockFile  bool
	NoUseRegistries  bool
	CommitLockFile   bool
	UpdateInputs     []string
	OverrideInputs   map[string]string
}

type EnterOpts struct {
	Command      string
	CommandArray []string
	RootLocation string
	System       string
	Silent       bool
	Verbose      bool
}

type FeaturesOpts struct {
	DisplayJson bool
}

type GenerationOpts struct {
	ProfileName string
}

type GenerationDiffOpts struct {
	Before  uint
	After   uint
	Verbose bool
}

type GenerationDeleteOpts struct {
	All        bool
	LowerBound uint64
	// This ideally should be a uint64 to match types,
	// but Cobra's pflags does not support this type yet.
	Keep          []uint
	MinimumToKeep uint64
	OlderThan     string
	UpperBound    uint64
	AlwaysConfirm bool
	Pattern       string
	// This ideally should be a uint64 to match types,
	// but Cobra's pflags does not support this type yet.
	Remove  []uint
	Verbose bool
}

type GenerationListOpts struct {
	DisplayJson  bool
	DisplayTable bool
}

type GenerationSwitchOpts struct {
	Dry            bool
	Specialisation string
	Verbose        bool
	AlwaysConfirm  bool
	Generation     uint
}

type GenerationRollbackOpts struct {
	Dry            bool
	Specialisation string
	Verbose        bool
	AlwaysConfirm  bool
}

type InfoOpts struct {
	DisplayJson     bool
	DisplayMarkdown bool
}

type InitOpts struct {
	Directory          string
	ForceWrite         bool
	NoFSGeneration     bool
	Root               string
	ShowHardwareConfig bool
}

type InstallOpts struct {
	Channel        string
	NoBootloader   bool
	NoChannelCopy  bool
	NoRootPassword bool
	Root           string
	SystemClosure  string
	Verbose        bool
	FlakeRef       *configuration.FlakeRef

	NixOptions struct {
		Quiet          bool
		PrintBuildLogs bool
		NoBuildOutput  bool
		ShowTrace      bool
		KeepGoing      bool
		KeepFailed     bool
		Fallback       bool
		Refresh        bool
		Repair         bool
		Impure         bool
		Offline        bool
		NoNet          bool
		MaxJobs        int
		Cores          int
		LogFormat      string
		Includes       []string
		Options        map[string]string

		RecreateLockFile bool
		NoUpdateLockFile bool
		NoWriteLockFile  bool
		NoUseRegistries  bool
		CommitLockFile   bool
		UpdateInputs     []string
		OverrideInputs   map[string]string
	}
}

type OptionOpts struct {
	NonInteractive   bool
	NixPathIncludes  []string
	DisplayJson      bool
	NoUseCache       bool
	DisplayValueOnly bool
	MinScore         int64
	OptionInput      string
	FlakeRef         string
}

type ReplOpts struct {
	NixPathIncludes []string
	FlakeRef        string
}
