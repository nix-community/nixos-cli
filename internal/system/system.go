package system

type System interface {
	CommandRunner
	IsNixOS() bool
	IsRemote() bool
}
