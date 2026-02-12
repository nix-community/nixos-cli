package nixopts

import (
	"reflect"
	"slices"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func addNixOptionBool(cmd *cobra.Command, dest *bool, name string, shorthand string, desc string) {
	flag := string(name)
	if shorthand != "" {
		cmd.Flags().BoolVarP(dest, flag, shorthand, false, desc)
	} else {
		cmd.Flags().BoolVar(dest, flag, false, desc)
	}
	cmd.Flags().Lookup(flag).Hidden = true
}

// func addNixOptionInt(cmd *cobra.Command, dest *int, name string, shorthand string, desc string) {
// 	flag := string(name)
// 	if shorthand != "" {
// 		cmd.Flags().IntVarP(dest, flag, shorthand, 0, desc)
// 	} else {
// 		cmd.Flags().IntVar(dest, flag, 0, desc)
// 	}
// 	cmd.Flags().Lookup(flag).Hidden = true
// }

func addNixOptionString(cmd *cobra.Command, dest *string, name string, shorthand string, desc string) {
	flag := string(name)
	if shorthand != "" {
		cmd.Flags().StringVarP(dest, flag, shorthand, "", desc)
	} else {
		cmd.Flags().StringVar(dest, flag, "", desc)
	}
	cmd.Flags().Lookup(flag).Hidden = true
}

func addNixOptionStringSlice(cmd *cobra.Command, dest *[]string, name string, shorthand string, desc string) {
	flag := string(name)
	if shorthand != "" {
		cmd.Flags().StringSliceVarP(dest, flag, shorthand, nil, desc)
	} else {
		cmd.Flags().StringSliceVar(dest, flag, nil, desc)
	}
	cmd.Flags().Lookup(flag).Hidden = true
}

func addNixOptionStringMap(cmd *cobra.Command, dest *map[string]string, name string, shorthand string, desc string) {
	flag := string(name)
	if shorthand != "" {
		cmd.Flags().StringToStringVarP(dest, flag, shorthand, nil, desc)
	} else {
		cmd.Flags().StringToStringVar(dest, flag, nil, desc)
	}
	cmd.Flags().Lookup(flag).Hidden = true
}

func addNixOptionVar(cmd *cobra.Command, dest pflag.Value, name string, shorthand string, desc string) {
	flag := string(name)
	if shorthand != "" {
		cmd.Flags().VarP(dest, flag, shorthand, desc)
	} else {
		cmd.Flags().Var(dest, flag, desc)
	}
	cmd.Flags().Lookup(flag).Hidden = true
}

type NixCommand string

const (
	CmdBuild        = "nix build"
	CmdLegacyBuild  = "nix-build"
	CmdCopyClosure  = "nix-copy-closure"
	CmdEval         = "nix eval"
	CmdInstantiate  = "nix-instantiate"
	CmdStoreRealise = "nix-store-realise"
)

type NixOption interface {
	Args() []string
	Supports(cmd NixCommand) bool
	Bind(cmd *cobra.Command)
}

type Quiet bool

func (q Quiet) Args() []string {
	if !q {
		return nil
	}

	return []string{"--quiet"}
}

func (q *Quiet) Bind(cmd *cobra.Command) {
	addNixOptionBool(cmd, (*bool)(q), "quiet", "", "Decrease logging verbosity level")
}

func (Quiet) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdLegacyBuild, CmdCopyClosure, CmdEval, CmdInstantiate, CmdStoreRealise:
		return true
	default:
		return false
	}
}

type PrintBuildLogs bool

func (p PrintBuildLogs) Args() []string {
	if !p {
		return nil
	}

	return []string{"--print-build-logs"}
}

func (p *PrintBuildLogs) Bind(cmd *cobra.Command) {
	addNixOptionBool(cmd, (*bool)(p), "print-build-logs", "L", "Print full build logs on stderr")
}

func (PrintBuildLogs) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdEval:
		return true
	default:
		return false
	}
}

type NoBuildOutput bool

func (n NoBuildOutput) Args() []string {
	if !n {
		return nil
	}

	return []string{"--no-build-output"}
}

func (n *NoBuildOutput) Bind(cmd *cobra.Command) {
	addNixOptionBool(cmd, (*bool)(n), "no-build-output", "Q", "Silence build output on stdout and stderr")
}

func (NoBuildOutput) Supports(c NixCommand) bool {
	switch c {
	case CmdLegacyBuild, CmdCopyClosure, CmdInstantiate, CmdStoreRealise:
		return true
	default:
		return false
	}
}

type ShowTrace bool

func (s ShowTrace) Args() []string {
	if !s {
		return nil
	}

	return []string{"--show-trace"}
}

func (s *ShowTrace) Bind(cmd *cobra.Command) {
	addNixOptionBool(cmd, (*bool)(s), "show-trace", "", "Print stack trace of evaluation errors")
}

func (ShowTrace) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdLegacyBuild, CmdCopyClosure, CmdEval, CmdInstantiate, CmdStoreRealise:
		return true
	default:
		return false
	}
}

type KeepGoing bool

func (k KeepGoing) Args() []string {
	if !k {
		return nil
	}

	return []string{"--keep-going"}
}

func (k *KeepGoing) Bind(cmd *cobra.Command) {
	addNixOptionBool(cmd, (*bool)(k), "keep-going", "k", "Keep going until all builds are finished despite failures")
}

func (KeepGoing) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdLegacyBuild, CmdCopyClosure, CmdEval, CmdInstantiate, CmdStoreRealise:
		return true
	default:
		return false
	}
}

type KeepFailed bool

func (k KeepFailed) Args() []string {
	if !k {
		return nil
	}

	return []string{"--keep-failed"}
}

func (k *KeepFailed) Bind(cmd *cobra.Command) {
	addNixOptionBool(cmd, (*bool)(k), "keep-failed", "K", "Keep failed builds (usually in /tmp)")
}

func (KeepFailed) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdLegacyBuild, CmdCopyClosure, CmdEval, CmdInstantiate, CmdStoreRealise:
		return true
	default:
		return false
	}
}

type Fallback bool

func (f Fallback) Args() []string {
	if !f {
		return nil
	}

	return []string{"--fallback"}
}

func (f *Fallback) Bind(cmd *cobra.Command) {
	addNixOptionBool(cmd, (*bool)(f), "fallback", "", "If binary download fails, fall back on building from source")
}

func (Fallback) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdLegacyBuild, CmdCopyClosure, CmdEval, CmdInstantiate, CmdStoreRealise:
		return true
	default:
		return false
	}
}

type Refresh bool

func (r Refresh) Args() []string {
	if !r {
		return nil
	}

	return []string{"--refresh"}
}

func (r *Refresh) Bind(cmd *cobra.Command) {
	addNixOptionBool(cmd, (*bool)(r), "refresh", "", "Consider all previously downloaded files out-of-date")
}

func (Refresh) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdEval:
		return true
	default:
		return false
	}
}

type Repair bool

func (r Repair) Args() []string {
	if !r {
		return nil
	}

	return []string{"--repair"}
}

func (r *Repair) Bind(cmd *cobra.Command) {
	addNixOptionBool(cmd, (*bool)(r), "repair", "", "Fix corrupted or missing store paths")
}

func (Repair) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdLegacyBuild, CmdEval, CmdInstantiate, CmdStoreRealise:
		return true
	default:
		return false
	}
}

type Impure bool

func (i Impure) Args() []string {
	if !i {
		return nil
	}

	return []string{"--impure"}
}

func (i *Impure) Bind(cmd *cobra.Command) {
	addNixOptionBool(cmd, (*bool)(i), "impure", "", "Allow access to mutable paths and repositories")
}

func (Impure) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdLegacyBuild, CmdEval, CmdInstantiate:
		return true
	default:
		return false
	}
}

type Offline bool

func (o Offline) Args() []string {
	if !o {
		return nil
	}

	return []string{"--offline"}
}

func (o *Offline) Bind(cmd *cobra.Command) {
	addNixOptionBool(cmd, (*bool)(o), "offline", "", "Disable substituters and consider all previously downloaded files up-to-date")
}

func (Offline) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdEval:
		return true
	default:
		return false
	}
}

type NoNet bool

func (n NoNet) Args() []string {
	if !n {
		return nil
	}

	return []string{"--no-net"}
}

func (n *NoNet) Bind(cmd *cobra.Command) {
	addNixOptionBool(cmd, (*bool)(n), "no-net", "", "Disable substituters and set all network timeout settings to minimum")
}

func (NoNet) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdEval:
		return true
	default:
		return false
	}
}

type MaxJobs struct {
	Value   int
	Changed bool
}

func (m MaxJobs) Args() []string {
	if !m.Changed {
		return nil
	}

	return []string{"--max-jobs", strconv.Itoa(m.Value)}
}

func (m *MaxJobs) Bind(cmd *cobra.Command) {
	addNixOptionVar(cmd, m, "max-jobs", "j", "Max number of build jobs in parallel")
}

func (MaxJobs) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdLegacyBuild, CmdCopyClosure, CmdEval, CmdInstantiate, CmdStoreRealise:
		return true
	default:
		return false
	}
}

func (m *MaxJobs) Type() string {
	return "int"
}

func (m *MaxJobs) String() string {
	return strconv.Itoa(m.Value)
}

func (m *MaxJobs) Set(s string) error {
	v, err := strconv.Atoi(s)
	if err != nil {
		return err
	}
	*m = MaxJobs{
		Value:   v,
		Changed: true,
	}
	return nil
}

type Cores struct {
	Value   int
	Changed bool
}

func (c Cores) Args() []string {
	if !c.Changed {
		return nil
	}

	return []string{"--cores", strconv.Itoa(c.Value)}
}

func (c *Cores) Bind(cmd *cobra.Command) {
	addNixOptionVar(cmd, c, "cores", "", "Max number of CPU cores used (sets NIX_BUILD_CORES env variable)")
}

func (Cores) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdLegacyBuild, CmdCopyClosure, CmdEval, CmdInstantiate, CmdStoreRealise:
		return true
	default:
		return false
	}
}

func (c *Cores) Type() string {
	return "int"
}

func (c *Cores) String() string {
	return strconv.Itoa(c.Value)
}

func (c *Cores) Set(s string) error {
	v, err := strconv.Atoi(s)
	if err != nil {
		return err
	}
	*c = Cores{
		Value:   v,
		Changed: true,
	}
	return nil
}

type Builders struct {
	Value   string
	Changed bool
}

func (b Builders) Args() []string {
	if !b.Changed {
		return nil
	}

	return []string{"--builders", string(b.Value)}
}

func (b *Builders) Bind(cmd *cobra.Command) {
	addNixOptionVar(cmd, b, "builders", "", "List of Nix remote builder addresses, passed as single string")
}

func (Builders) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdLegacyBuild, CmdCopyClosure, CmdEval, CmdInstantiate, CmdStoreRealise:
		return true
	default:
		return false
	}
}

func (b *Builders) Type() string {
	return "string"
}

func (b *Builders) String() string {
	return b.Value
}

func (b *Builders) Set(s string) error {
	*b = Builders{
		Value:   s,
		Changed: true,
	}
	return nil
}

type LogFormat string

func (l LogFormat) Args() []string {
	if l == "" {
		return nil
	}

	return []string{"--log-format", string(l)}
}

func (l *LogFormat) Bind(cmd *cobra.Command) {
	addNixOptionString(cmd, (*string)(l), "log-format", "", "Configure how output is formatted")
}

func (LogFormat) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdLegacyBuild, CmdCopyClosure, CmdEval, CmdInstantiate, CmdStoreRealise:
		return true
	default:
		return false
	}
}

type Include []string

func (i Include) Args() []string {
	args := []string{}
	for _, include := range i {
		args = append(args, "--include", include)
	}
	return args
}

func (i *Include) Bind(cmd *cobra.Command) {
	addNixOptionStringSlice(cmd, (*[]string)(i), "include", "I", "Add path to list of locations to look up <...> file names")
}

func (Include) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdLegacyBuild, CmdEval, CmdInstantiate:
		return true
	default:
		return false
	}
}

type Option map[string]string

func (o Option) Args() []string {
	sortedKeys := make([]string, 0, len(o))
	for key := range o {
		sortedKeys = append(sortedKeys, key)
	}
	slices.Sort(sortedKeys)

	args := []string{}
	for _, key := range sortedKeys {
		args = append(args, "--option", key, o[key])
	}

	return args
}

func (o *Option) Bind(cmd *cobra.Command) {
	addNixOptionStringMap(cmd, (*map[string]string)(o), "option", "", "Set Nix config option (passed as 1 arg, requires = separator)")
}

func (Option) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdLegacyBuild, CmdCopyClosure, CmdEval, CmdInstantiate, CmdStoreRealise:
		return true
	default:
		return false
	}
}

type SubstituteOnDestination bool

func (s SubstituteOnDestination) Args() []string {
	if !s {
		return nil
	}

	// This can be for both `--use-substitutes` and
	// `--substitute-on-destination`, so use the common
	// short flag alias between them for this.
	return []string{"-s"}
}

func (s *SubstituteOnDestination) Bind(cmd *cobra.Command) {
	addNixOptionBool(cmd, (*bool)(s), "substitute-on-destination", "", "Let remote machine substitute missing store paths")
	addNixOptionBool(cmd, (*bool)(s), "use-substitutes", "", "Let remote machine substitute missing store paths")
	cmd.Flags().Lookup(string("use-substitutes")).Deprecated = string("use --substitute-on-destination instead")
}

func (SubstituteOnDestination) Supports(c NixCommand) bool {
	switch c {
	case CmdCopyClosure:
		return true
	default:
		return false
	}
}

type RecreateLockFile bool

func (r RecreateLockFile) Args() []string {
	if !r {
		return nil
	}

	return []string{"--recreate-lock-file"}
}

func (r *RecreateLockFile) Bind(cmd *cobra.Command) {
	addNixOptionBool(cmd, (*bool)(r), "recreate-lock-file", "", "Recreate the flake's lock file from scratch")
}

func (RecreateLockFile) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdEval:
		return true
	default:
		return false
	}
}

type NoUpdateLockFile bool

func (n NoUpdateLockFile) Args() []string {
	if !n {
		return nil
	}

	return []string{"--no-update-lock-file"}
}

func (n *NoUpdateLockFile) Bind(cmd *cobra.Command) {
	addNixOptionBool(cmd, (*bool)(n), "no-update-lock-file", "", "Do not allow any updates to the flake's lock file")
}

func (NoUpdateLockFile) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdEval:
		return true
	default:
		return false
	}
}

type NoWriteLockFile bool

func (n NoWriteLockFile) Args() []string {
	if !n {
		return nil
	}

	return []string{"--no-write-lock-file"}
}

func (n *NoWriteLockFile) Bind(cmd *cobra.Command) {
	addNixOptionBool(cmd, (*bool)(n), "no-write-lock-file", "", "Do not write the flake's newly generated lock file")
}

func (NoWriteLockFile) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdEval:
		return true
	default:
		return false
	}
}

type NoUseRegistries bool

func (n NoUseRegistries) Args() []string {
	if !n {
		return nil
	}

	return []string{"--no-use-registries"}
}

func (n *NoUseRegistries) Bind(cmd *cobra.Command) {
	addNixOptionBool(cmd, (*bool)(n), "no-use-registries", "", "Don't allow lookups in the flake registries")
	addNixOptionBool(cmd, (*bool)(n), "no-registries", "", "Don't allow lookups in the flake registries")
	cmd.Flags().Lookup(string("no-registries")).Deprecated = string("use --no-use-registries instead")
}

func (NoUseRegistries) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdEval:
		return true
	default:
		return false
	}
}

type CommitLockFile bool

func (c CommitLockFile) Args() []string {
	if !c {
		return nil
	}

	return []string{"--commit-lock-file"}
}

func (c *CommitLockFile) Bind(cmd *cobra.Command) {
	addNixOptionBool(cmd, (*bool)(c), "commit-lock-file", "", "Commit changes to the flake's lock file")
}

func (CommitLockFile) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdEval:
		return true
	default:
		return false
	}
}

type UpdateInput []string

func (u UpdateInput) Args() []string {
	args := []string{}
	for _, input := range u {
		args = append(args, "--update-input", input)
	}
	return args
}

func (u *UpdateInput) Bind(cmd *cobra.Command) {
	addNixOptionStringSlice(cmd, (*[]string)(u), "update-input", "", "Update a specific flake input")
}

func (UpdateInput) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdEval:
		return true
	default:
		return false
	}
}

type OverrideInput map[string]string

func (o OverrideInput) Args() []string {
	sortedKeys := make([]string, 0, len(o))
	for key := range o {
		sortedKeys = append(sortedKeys, key)
	}
	slices.Sort(sortedKeys)

	args := []string{}
	for _, key := range sortedKeys {
		args = append(args, "--override-input", key, o[key])
	}

	return args
}

func (o *OverrideInput) Bind(cmd *cobra.Command) {
	addNixOptionStringMap(cmd, (*map[string]string)(o), "override-input", "", "Override a specific flake input (passed as 1 arg, requires = separator)")
}

func (OverrideInput) Supports(c NixCommand) bool {
	switch c {
	case CmdBuild, CmdEval:
		return true
	default:
		return false
	}
}

type NixOptionsSet interface {
	Flags() []NixOption
	ArgsForCommand(cmd NixCommand) []string
}

// Collect all flags in a NixOption struct and convert it into a
// slice of options, sorted by string order of the field names.
//
// Fields with the same type will overwrite each other; as such,
// if fields share types, some fields will end up missing in the
// result. Make sure that any struct passed to this function will
// have unique field types, all that adhere to the NixOption interface.
func CollectFlags(v any) []NixOption {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		panic("CollectFlags expects non-nil pointer to struct")
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		panic("CollectFlags expects pointer to struct")
	}

	fieldMap := make(map[string]NixOption)

	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)

		if flag, ok := f.Addr().Interface().(NixOption); ok {
			name := f.Type().Name()
			fieldMap[name] = flag
		}
	}

	sortedKeys := make([]string, 0, len(fieldMap))
	for key := range fieldMap {
		sortedKeys = append(sortedKeys, key)
	}
	slices.Sort(sortedKeys)

	flags := make([]NixOption, 0, len(sortedKeys))
	for _, key := range sortedKeys {
		flags = append(flags, fieldMap[key])
	}

	return flags
}

// Serialize a set of NixOptions into an argument slice, suitable
// for adding to a string slice passed to exec.Command.
func ArgsForOptionsSet(options []NixOption, cmd NixCommand) []string {
	var out []string
	for _, opt := range options {
		if opt.Supports(cmd) {
			out = append(out, opt.Args()...)
		}
	}
	return out
}
