package logger

// MultiLogger fans out log calls to multiple underlying loggers.
type MultiLogger struct {
	loggers []Logger
}

func NewMultiLogger(loggers ...Logger) *MultiLogger {
	return &MultiLogger{loggers: loggers}
}

func (m *MultiLogger) SetLogLevel(level LogLevel) {
	for _, l := range m.loggers {
		l.SetLogLevel(level)
	}
}

func (m *MultiLogger) GetLogLevel() LogLevel {
	if len(m.loggers) == 0 {
		return LogLevelInfo
	}

	minimumLevel := m.loggers[0].GetLogLevel()

	for _, l := range m.loggers[1:] {
		if lvl := l.GetLogLevel(); lvl < minimumLevel {
			minimumLevel = lvl
		}
	}

	return minimumLevel
}

func (m *MultiLogger) Print(v ...any) {
	for _, l := range m.loggers {
		l.Print(v...)
	}
}

func (m *MultiLogger) Printf(format string, v ...any) {
	for _, l := range m.loggers {
		l.Printf(format, v...)
	}
}

func (m *MultiLogger) Debug(v ...any) {
	for _, l := range m.loggers {
		l.Debug(v...)
	}
}

func (m *MultiLogger) Debugf(format string, v ...any) {
	for _, l := range m.loggers {
		l.Debugf(format, v...)
	}
}

func (m *MultiLogger) Info(v ...any) {
	for _, l := range m.loggers {
		l.Info(v...)
	}
}

func (m *MultiLogger) Infof(format string, v ...any) {
	for _, l := range m.loggers {
		l.Infof(format, v...)
	}
}

func (m *MultiLogger) Warn(v ...any) {
	for _, l := range m.loggers {
		l.Warn(v...)
	}
}

func (m *MultiLogger) Warnf(format string, v ...any) {
	for _, l := range m.loggers {
		l.Warnf(format, v...)
	}
}

func (m *MultiLogger) Error(v ...any) {
	for _, l := range m.loggers {
		l.Error(v...)
	}
}

func (m *MultiLogger) Errorf(format string, v ...any) {
	for _, l := range m.loggers {
		l.Errorf(format, v...)
	}
}

func (m *MultiLogger) CmdArray(argv []string) {
	for _, l := range m.loggers {
		l.CmdArray(argv)
	}
}

func (m *MultiLogger) Step(message string) {
	for _, l := range m.loggers {
		l.Step(message)
	}
}
