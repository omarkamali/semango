package util

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

var Logger *slog.Logger
var LogLevelVar = &slog.LevelVar{}

// SetLogLevel updates the global logger level
func SetLogLevel(level slog.Level) {
	LogLevelVar.Set(level)
}

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

type PrettyHandler struct {
	opts slog.HandlerOptions
	out  io.Writer
}

func (h *PrettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *PrettyHandler) Handle(_ context.Context, r slog.Record) error {
	level := r.Level.String()
	levelColor := colorReset

	switch r.Level {
	case slog.LevelDebug:
		levelColor = colorGray
	case slog.LevelInfo:
		levelColor = colorCyan
	case slog.LevelWarn:
		levelColor = colorYellow
	case slog.LevelError:
		levelColor = colorRed
	}

	timeStr := r.Time.Format("15:04:05")
	msg := r.Message

	fmt.Fprintf(h.out, "%s %s%s%s %s",
		colorGray+timeStr+colorReset,
		levelColor, strings.ToUpper(level), colorReset,
		msg,
	)

	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(h.out, " %s%s=%v%s", colorGray, a.Key, a.Value, colorReset)
		return true
	})

	fmt.Fprintf(h.out, "\n")
	return nil
}

func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Simple implementation: ignore for now or implement properly if needed
	return h
}

func (h *PrettyHandler) WithGroup(name string) slog.Handler {
	// Simple implementation
	return h
}

func init() {
	// Default to Pretty handler, info level, writing to stderr to keep stdout clean for data output
	level := slog.LevelInfo
	if os.Getenv("DEBUG") != "" || os.Getenv("VERBOSE") != "" {
		level = slog.LevelDebug
	}
	LogLevelVar.Set(level)

	Logger = slog.New(&PrettyHandler{
		out: os.Stderr,
		opts: slog.HandlerOptions{
			Level: LogLevelVar,
		},
	})

	slog.SetDefault(Logger)
}

// Example of how to use it from other packages:
// import "github.com/omarkamali/semango/internal/util"
// ...
// util.Logger.Info("Something happened", "key", "value")
// or if SetDefault was called:
// slog.Info("Something happened", "key", "value")
