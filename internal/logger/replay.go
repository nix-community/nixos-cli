package logger

import "slices"

type ReplayLogger struct {
	entries []logEntry
	out     Logger
}

type logKind int

const (
	kindDebug logKind = iota
	kindDebugf
	kindInfo
	kindInfof
	kindWarn
	kindWarnf
	kindError
	kindErrorf
	kindPrint
	kindPrintf
	kindCmdArray
	kindStep
)

type logEntry struct {
	kind   logKind
	format string
	args   []any
	argv   []string
	msg    string
}

func NewReplayLogger(out Logger) *ReplayLogger {
	return &ReplayLogger{
		entries: make([]logEntry, 0, 64),
		out:     out,
	}
}

func (l *ReplayLogger) Flush() {
	for _, e := range l.entries {
		switch e.kind {
		case kindDebug:
			l.out.Debug(e.args...)
		case kindDebugf:
			l.out.Debugf(e.format, e.args...)
		case kindInfo:
			l.out.Info(e.args...)
		case kindInfof:
			l.out.Infof(e.format, e.args...)
		case kindWarn:
			l.out.Warn(e.args...)
		case kindWarnf:
			l.out.Warnf(e.format, e.args...)
		case kindError:
			l.out.Error(e.args...)
		case kindErrorf:
			l.out.Errorf(e.format, e.args...)
		case kindPrint:
			l.out.Print(e.args...)
		case kindPrintf:
			l.out.Printf(e.format, e.args...)
		case kindCmdArray:
			l.out.CmdArray(e.argv)
		case kindStep:
			l.out.Step(e.msg)
		}
	}
	l.entries = l.entries[:0]
}

func (l *ReplayLogger) HasEntries() bool {
	return len(l.entries) > 0
}

func (l *ReplayLogger) Print(v ...any) {
	l.entries = append(l.entries, logEntry{
		kind: kindPrint,
		args: v,
	})
}

func (l *ReplayLogger) Printf(format string, v ...any) {
	l.entries = append(l.entries, logEntry{
		kind:   kindPrintf,
		format: format,
		args:   v,
	})
}

func (l *ReplayLogger) Info(v ...any) {
	l.entries = append(l.entries, logEntry{
		kind: kindInfo,
		args: v,
	})
}

func (l *ReplayLogger) Infof(format string, v ...any) {
	l.entries = append(l.entries, logEntry{
		kind:   kindInfof,
		format: format,
		args:   v,
	})
}

func (l *ReplayLogger) Debug(v ...any) {
	l.entries = append(l.entries, logEntry{
		kind: kindDebug,
		args: v,
	})
}

func (l *ReplayLogger) Debugf(format string, v ...any) {
	l.entries = append(l.entries, logEntry{
		kind:   kindDebugf,
		format: format,
		args:   v,
	})
}

func (l *ReplayLogger) Warn(v ...any) {
	l.entries = append(l.entries, logEntry{
		kind: kindWarn,
		args: v,
	})
}

func (l *ReplayLogger) Warnf(format string, v ...any) {
	l.entries = append(l.entries, logEntry{
		kind:   kindWarnf,
		format: format,
		args:   v,
	})
}

func (l *ReplayLogger) Error(v ...any) {
	l.entries = append(l.entries, logEntry{
		kind: kindError,
		args: v,
	})
}

func (l *ReplayLogger) Errorf(format string, v ...any) {
	l.entries = append(l.entries, logEntry{
		kind:   kindErrorf,
		format: format,
		args:   v,
	})
}

func (l *ReplayLogger) Step(msg string) {
	l.entries = append(l.entries, logEntry{
		kind: kindStep,
		msg:  msg,
	})
}

func (l *ReplayLogger) CmdArray(args []string) {
	l.entries = append(l.entries, logEntry{
		kind: kindCmdArray,
		argv: slices.Clone(args),
	})
}

func (l *ReplayLogger) GetLogLevel() LogLevel {
	return l.out.GetLogLevel()
}

func (l *ReplayLogger) SetLogLevel(level LogLevel) {
	// no-op, let log sink control which level they want
}
