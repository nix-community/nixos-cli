package logger

import (
	"log"
	"os"

	"github.com/fatih/color"
	"github.com/nix-community/nixos-cli/internal/utils"
)

var (
	green      = color.New(color.FgGreen)
	boldYellow = color.New(color.FgYellow).Add(color.Bold)
	boldRed    = color.New(color.FgRed).Add(color.Bold)
	blue       = color.New(color.FgBlue)
)

type ConsoleLogger struct {
	print *log.Logger
	cmd   *log.Logger
	debug *log.Logger
	info  *log.Logger
	warn  *log.Logger
	error *log.Logger

	level        LogLevel
	stepNumber   uint
	stepsEnabled bool
}

func NewConsoleLogger() *ConsoleLogger {
	return &ConsoleLogger{
		print: log.New(os.Stderr, "", 0),
		debug: log.New(os.Stderr, blue.Sprint("debug: "), 0),
		cmd:   log.New(os.Stderr, blue.Sprint("$ "), 0),
		info:  log.New(os.Stderr, green.Sprint("info: "), 0),
		warn:  log.New(os.Stderr, boldYellow.Sprint("warning: "), 0),
		error: log.New(os.Stderr, boldRed.Sprint("error: "), 0),

		level:        LogLevelInfo,
		stepNumber:   0,
		stepsEnabled: os.Getenv("NIXOS_CLI_DISABLE_STEPS") == "",
	}
}

func (l *ConsoleLogger) SetLogLevel(level LogLevel) {
	l.level = level
}

func (l *ConsoleLogger) GetLogLevel() LogLevel {
	return l.level
}

func (l *ConsoleLogger) Print(v ...any) {
	l.print.Print(v...)
}

func (l *ConsoleLogger) Printf(format string, v ...any) {
	l.print.Printf(format, v...)
}

func (l *ConsoleLogger) Debug(v ...any) {
	if l.level > LogLevelDebug {
		return
	}
	l.debug.Println(v...)
}

func (l *ConsoleLogger) Debugf(format string, v ...any) {
	if l.level > LogLevelDebug {
		return
	}

	l.debug.Printf(format+"\n", v...)
}

func (l *ConsoleLogger) Info(v ...any) {
	if l.level > LogLevelInfo {
		return
	}
	l.info.Println(v...)
}

func (l *ConsoleLogger) Infof(format string, v ...any) {
	if l.level > LogLevelInfo {
		return
	}

	l.info.Printf(format+"\n", v...)
}

func (l *ConsoleLogger) Warn(v ...any) {
	if l.level > LogLevelWarn {
		return
	}

	l.warn.Println(v...)
}

func (l *ConsoleLogger) Warnf(format string, v ...any) {
	if l.level > LogLevelWarn {
		return
	}

	l.warn.Printf(format+"\n", v...)
}

func (l *ConsoleLogger) Error(v ...any) {
	if l.level > LogLevelError {
		return
	}

	l.error.Println(v...)
}

func (l *ConsoleLogger) Errorf(format string, v ...any) {
	if l.level > LogLevelError {
		return
	}

	l.error.Printf(format+"\n", v...)
}

func (l *ConsoleLogger) CmdArray(argv []string) {
	if l.level > LogLevelDebug {
		return
	}

	msg := blue.Sprintf("%v", utils.EscapeAndJoinArgs(argv))
	l.cmd.Printf("%v\n", msg)
}

func (l *ConsoleLogger) Step(message string) {
	if !l.stepsEnabled {
		l.Info(message)
		return
	}

	if l.level > LogLevelInfo {
		return
	}

	l.stepNumber++
	if l.stepNumber > 1 {
		l.print.Println()
	}

	msg := color.New(color.FgMagenta).Add(color.Bold).Sprintf("%v. %v", l.stepNumber, message)
	l.print.Println(msg)
}

// Call this when the colors have been enabled or disabled.
func (l *ConsoleLogger) RefreshColorPrefixes() {
	l.debug.SetPrefix(blue.Sprint("debug: "))
	l.cmd.SetPrefix(blue.Sprint("$ "))
	l.info.SetPrefix(green.Sprint("info: "))
	l.warn.SetPrefix(boldYellow.Sprint("warning: "))
	l.error.SetPrefix(boldRed.Sprint("error: "))
}
