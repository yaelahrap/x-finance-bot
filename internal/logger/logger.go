// Package logger provides a structured logging factory built on log/slog.
//
// Use New to create a configured *slog.Logger, then pass it explicitly through
// your application. Context helpers allow attaching/retrieving loggers from
// context.Context for middleware and handler use.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Options configures the logger factory.
type Options struct {
	// Level is the minimum log level: "debug", "info", "warn", "error".
	// Defaults to "info" if empty or unrecognized.
	Level string

	// Format selects the output handler: "text" or "json".
	// Defaults to "text" unless Env is "production", in which case "json".
	Format string

	// Env is the deployment environment, added as a default attribute.
	Env string

	// Service is the service name, added as a default attribute.
	// Defaults to "x-finance-bot".
	Service string

	// Output is the writer for log output. Defaults to os.Stdout.
	Output io.Writer
}

// New creates a *slog.Logger from the given options.
func New(opts Options) *slog.Logger {
	if opts.Output == nil {
		opts.Output = os.Stdout
	}
	if opts.Service == "" {
		opts.Service = "x-finance-bot"
	}

	level := ParseLevel(opts.Level)

	format := strings.ToLower(opts.Format)
	if format == "" {
		if opts.Env == "production" {
			format = "json"
		} else {
			format = "text"
		}
	}

	handlerOpts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(opts.Output, handlerOpts)
	default:
		handler = slog.NewTextHandler(opts.Output, handlerOpts)
	}

	// Inject default attributes.
	var attrs []slog.Attr
	attrs = append(attrs, slog.String("service", opts.Service))
	if opts.Env != "" {
		attrs = append(attrs, slog.String("env", opts.Env))
	}

	if len(attrs) > 0 {
		handler = handler.WithAttrs(attrs)
	}

	return slog.New(handler)
}

// ParseLevel converts a level string to slog.Level.
// Recognized values (case-insensitive): "debug", "info", "warn", "error".
// Returns slog.LevelInfo for unrecognized input.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type ctxKey struct{}

// ContextWithLogger returns a new context with the given logger attached.
func ContextWithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext retrieves the logger from context. If none is set, it returns
// slog.Default().
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}
