package util

import (
	"context"
	"log/slog"
)

// contextKey is used to store logger in context
type contextKey string

const loggerKey contextKey = "logger"

// FromContext retrieves a logger from the context.
// If no logger is stored in the context, it returns the default logger.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}
	return Logger // Fall back to global logger
}

// WithLogger adds a logger to the context.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// WithField adds a field to the logger in the context.
// If no logger exists in the context, it creates one with the field.
func WithField(ctx context.Context, key string, value interface{}) context.Context {
	logger := FromContext(ctx).With(key, value)
	return WithLogger(ctx, logger)
}

// WithFields adds multiple fields to the logger in the context.
func WithFields(ctx context.Context, fields map[string]interface{}) context.Context {
	logger := FromContext(ctx)
	for key, value := range fields {
		logger = logger.With(key, value)
	}
	return WithLogger(ctx, logger)
}
