package cmdOpts

import (
	"github.com/nix-community/nixos-cli/internal/activation"
	"github.com/nix-community/nixos-cli/internal/configuration"
)

type MainOpts struct {
	ColorAlways  bool
	ConfigValues map[string]string
}

type ActivateOpts struct {
	Action         activation.SwitchToConfigurationAction
	Specialisation string
	Verbose        bool
}

type AliasesOpts struct {
	DisplayJson bool
}

type ApplyOpts struct {
	File                  string
	Attr                  string
	AlwaysConfirm         bool
	BuildHost             string
	BuildImage            string
	BuildVM               bool
	BuildVMWithBootloader bool
	Dry                   bool
	FlakeRef              string
	GenerationTag         string
	InstallBootloader     bool
	NoActivate            bool
	NoBoot                bool
	OutputPath            string
	ProfileName           string
	LocalRoot             bool
	RemoteRoot            bool
	Specialisation        string
	StorePath             string
	TargetHost            string
	UpgradeAllChannels    bool
	UpgradeChannels       bool
	UseNom                bool
	Verbose               bool

	NixOptions ApplyNixOpts
}

type ApplyNixOpts struct {
	Quiet                   bool              `nixCategory:"build"`
	PrintBuildLogs          bool              `nixCategory:"build"`
	NoBuildOutput           bool              `nixCategory:"build"`
	ShowTrace               bool              `nixCategory:"build"`
	KeepGoing               bool              `nixCategory:"build,copy"`
	KeepFailed              bool              `nixCategory:"build,copy"`
	Fallback                bool              `nixCategory:"build,copy"`
	Refresh                 bool              `nixCategory:"build,copy"`
	Repair                  bool              `nixCategory:"build,copy"`
	Impure                  bool              `nixCategory:"build"`
	Offline                 bool              `nixCategory:"build"`
	NoNet                   bool              `nixCategory:"build"`
	SubstituteOnDestination bool              `nixCategory:"build,copy"`
	MaxJobs                 int               `nixCategory:"build,copy"`
	Cores                   int               `nixCategory:"build,copy"`
	Builders                []string          `nixCategory:"build"`
	LogFormat               string            `nixCategory:"build,copy"`
	Includes                []string          `nixCategory:"build"`
	Options                 map[string]string `nixCategory:"build,copy"`

	RecreateLockFile bool              `nixCategory:"lock"`
	NoUpdateLockFile bool              `nixCategory:"lock"`
	NoWriteLockFile  bool              `nixCategory:"lock"`
	NoUseRegistries  bool              `nixCategory:"lock"`
	CommitLockFile   bool              `nixCategory:"lock"`
	UpdateInputs     []string          `nixCategory:"lock"`
	OverrideInputs   map[string]string `nixCategory:"lock"`
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
	File           string
	Attr           string

	NixOptions InstallNixOpts
}

type InstallNixOpts struct {
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

type OptionOpts struct {
	NonInteractive   bool
	NixPathIncludes  []string
	DisplayJson      bool
	NoUseCache       bool
	DisplayValueOnly bool
	MinScore         int64
	OptionInput      string
	FlakeRef         string
	File             string
	Attr             string
}

type ReplOpts struct {
	NixPathIncludes []string
	FlakeRef        string
	File            string
	Attr            string
}
