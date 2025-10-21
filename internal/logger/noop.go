package logger

type NoOpLogger struct{}

func NewNoOpLogger() NoOpLogger {
	return NoOpLogger{}
}

func (n NoOpLogger) SetLogLevel(level LogLevel) {}

func (n NoOpLogger) GetLogLevel() LogLevel {
	return LogLevelDebug
}

func (n NoOpLogger) Print(v ...any) {}

func (n NoOpLogger) Printf(format string, v ...any) {}

func (n NoOpLogger) Debug(v ...any) {}

func (n NoOpLogger) Debugf(format string, v ...any) {}

func (n NoOpLogger) Info(v ...any) {}

func (n NoOpLogger) Infof(format string, v ...any) {}

func (n NoOpLogger) Warn(v ...any) {}

func (n NoOpLogger) Warnf(format string, v ...any) {}

func (n NoOpLogger) Error(v ...any) {}

func (n NoOpLogger) Errorf(format string, v ...any) {}

func (n NoOpLogger) CmdArray(argv []string) {}

func (n NoOpLogger) Step(message string) {}
