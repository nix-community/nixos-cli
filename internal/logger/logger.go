package logger

type LogLevel int

const (
	LogLevelInfo   LogLevel = 0
	LogLevelWarn   LogLevel = 1
	LogLevelError  LogLevel = 2
	LogLevelSilent LogLevel = 3
)

type Logger interface {
	SetLogLevel(level LogLevel)
	Info(v ...any)
	Infof(format string, v ...any)
	Warn(v ...any)
	Warnf(format string, v ...any)
	Error(v ...any)
	Errorf(format string, v ...any)
	Print(v ...any)
	Printf(format string, v ...any)
	CmdArray(argv []string)
	Step(message string)
}
