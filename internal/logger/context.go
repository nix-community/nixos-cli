package logger

import "context"

type loggerCtxKeyType string

const loggerCtxKey loggerCtxKeyType = "logger"

func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey, logger)
}

func FromContext(ctx context.Context) Logger {
	logger, ok := ctx.Value(loggerCtxKey).(Logger)
	if !ok {
		panic("logger not present in context")
	}

	return logger
}
