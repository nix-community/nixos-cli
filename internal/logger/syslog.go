package logger

import (
	"fmt"
	"log/syslog"
	"os"

	"github.com/nix-community/nixos-cli/internal/utils"
)

type SyslogLogger struct {
	writer *syslog.Writer

	level        LogLevel
	stepNumber   uint
	stepsEnabled bool
}

func NewSyslogLogger(tag string) (*SyslogLogger, error) {
	w, err := syslog.New(syslog.LOG_DEBUG|syslog.LOG_USER, tag)
	if err != nil {
		return nil, err
	}

	return &SyslogLogger{
		writer:       w,
		level:        LogLevelInfo,
		stepNumber:   0,
		stepsEnabled: os.Getenv("NIXOS_CLI_DISABLE_STEPS") == "",
	}, nil
}

func (l *SyslogLogger) SetLogLevel(level LogLevel) {
	l.level = level
}

func (l *SyslogLogger) GetLogLevel() LogLevel {
	return l.level
}

func (l *SyslogLogger) Debug(v ...any) {
	if l.level > LogLevelDebug {
		return
	}

	_ = l.writer.Debug(fmt.Sprint(v...))
}

func (l *SyslogLogger) Debugf(format string, v ...any) {
	if l.level > LogLevelDebug {
		return
	}

	_ = l.writer.Debug(fmt.Sprintf(format, v...))
}

func (l *SyslogLogger) Print(v ...any) {
	_ = l.writer.Info(fmt.Sprint(v...))
}

func (l *SyslogLogger) Printf(format string, v ...any) {
	_ = l.writer.Info(fmt.Sprintf(format, v...))
}

func (l *SyslogLogger) Info(v ...any) {
	if l.level > LogLevelInfo {
		return
	}

	_ = l.writer.Info(fmt.Sprint(v...))
}

func (l *SyslogLogger) Infof(format string, v ...any) {
	if l.level > LogLevelInfo {
		return
	}

	_ = l.writer.Info(fmt.Sprintf(format, v...))
}

func (l *SyslogLogger) Warn(v ...any) {
	if l.level > LogLevelWarn {
		return
	}

	_ = l.writer.Warning(fmt.Sprint(v...))
}

func (l *SyslogLogger) Warnf(format string, v ...any) {
	if l.level > LogLevelWarn {
		return
	}

	_ = l.writer.Warning(fmt.Sprintf(format, v...))
}

func (l *SyslogLogger) Error(v ...any) {
	if l.level > LogLevelError {
		return
	}

	_ = l.writer.Err(fmt.Sprint(v...))
}

func (l *SyslogLogger) Errorf(format string, v ...any) {
	if l.level > LogLevelError {
		return
	}

	_ = l.writer.Err(fmt.Sprintf(format, v...))
}

func (l *SyslogLogger) CmdArray(argv []string) {
	if l.level > LogLevelDebug {
		return
	}

	_ = l.writer.Debug(fmt.Sprintf("$ %v", utils.EscapeAndJoinArgs(argv)))
}

func (l *SyslogLogger) Step(message string) {
	if !l.stepsEnabled {
		l.Info(message)
		return
	}

	if l.level > LogLevelInfo {
		return
	}

	l.stepNumber++
	stepMsg := fmt.Sprintf("%v. %v", l.stepNumber, message)
	_ = l.writer.Info(stepMsg)
}
