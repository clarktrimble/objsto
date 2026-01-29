package objsto

import "context"

// Logger specifies a contextual, structured logger.
type Logger interface {
	Info(ctx context.Context, msg string, kv ...any)
	Debug(ctx context.Context, msg string, kv ...any)
	Trace(ctx context.Context, msg string, kv ...any)
	Error(ctx context.Context, msg string, err error, kv ...any)
}
