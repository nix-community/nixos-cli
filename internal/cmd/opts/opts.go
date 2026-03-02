package cmdOpts

import (
	"github.com/nix-community/nixos-cli/internal/activation"
	"github.com/nix-community/nixos-cli/internal/cmd/nixopts"
	"github.com/nix-community/nixos-cli/internal/configuration"
	systemdUtils "github.com/nix-community/nixos-cli/internal/systemd"
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
	AlwaysConfirm         bool
	Attr                  string
	BuildHost             string
	BuildImage            string
	BuildVM               bool
	BuildVMWithBootloader bool
	Dry                   bool
	File                  string
	FlakeRef              string
	GenerationTag         string
	InstallBootloader     bool
	LocalRoot             bool
	NoActivate            bool
	NoBoot                bool
	NoRollback            bool
	OutputPath            string
	ProfileName           string
	RemoteRoot            bool
	RollbackTimeout       systemdUtils.SystemdDuration
	Specialisation        string
	StorePath             string
	TargetHost            string
	UpgradeAllChannels    bool
	UpgradeChannels       bool
	EvalOnly              bool
	UseNom                bool
	Verbose               bool

	NixOptions ApplyNixOpts
}

type ApplyNixOpts struct {
	nixopts.Quiet
	nixopts.PrintBuildLogs
	nixopts.NoBuildOutput
	nixopts.ShowTrace
	nixopts.KeepGoing
	nixopts.KeepFailed
	nixopts.Fallback
	nixopts.Refresh
	nixopts.Repair
	nixopts.Impure
	nixopts.Offline
	nixopts.NoNet
	nixopts.SubstituteOnDestination
	nixopts.MaxJobs
	nixopts.Cores
	nixopts.Builders
	nixopts.LogFormat
	nixopts.Option
	nixopts.Include

	nixopts.RecreateLockFile
	nixopts.NoUpdateLockFile
	nixopts.NoWriteLockFile
	nixopts.NoUseRegistries
	nixopts.CommitLockFile
	nixopts.UpdateInput
	nixopts.OverrideInput
}

func (o *ApplyNixOpts) Flags() []nixopts.NixOption {
	return nixopts.CollectFlags(o)
}

func (o *ApplyNixOpts) ArgsForCommand(cmd nixopts.NixCommand) []string {
	return nixopts.ArgsForOptionsSet(o.Flags(), cmd)
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
	OlderThan     systemdUtils.SystemdDuration
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
	nixopts.Quiet
	nixopts.PrintBuildLogs
	nixopts.NoBuildOutput
	nixopts.ShowTrace
	nixopts.KeepGoing
	nixopts.KeepFailed
	nixopts.Fallback
	nixopts.Refresh
	nixopts.Repair
	nixopts.Impure
	nixopts.Offline
	nixopts.NoNet
	nixopts.MaxJobs
	nixopts.Cores
	nixopts.LogFormat
	nixopts.Builders
	nixopts.Include
	nixopts.Option

	nixopts.RecreateLockFile
	nixopts.NoUpdateLockFile
	nixopts.NoWriteLockFile
	nixopts.NoUseRegistries
	nixopts.CommitLockFile
	nixopts.UpdateInput
	nixopts.OverrideInput
}

func (o *InstallNixOpts) Flags() []nixopts.NixOption {
	return nixopts.CollectFlags(o)
}

func (o *InstallNixOpts) ArgsForCommand(cmd nixopts.NixCommand) []string {
	return nixopts.ArgsForOptionsSet(o.Flags(), cmd)
}

type OptionOpts struct {
	NonInteractive   bool
	DisplayJson      bool
	NoUseCache       bool
	DisplayValueOnly bool
	MinScore         int64
	OptionInput      string
	FlakeRef         string
	File             string
	Attr             string

	Include nixopts.Include
}

type ReplOpts struct {
	FlakeRef string
	File     string
	Attr     string

	Include nixopts.Include
}
