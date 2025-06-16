package util

import (
	"log/slog"
	"os"
)

var Logger *slog.Logger

func init() {
	// Default to JSON handler, info level, writing to stdout
	// This can be configured further later (e.g., based on config file)
	Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	slog.SetDefault(Logger) // Optionally set as default for global slog functions like slog.Info()
}

// Example of how to use it from other packages:
// import "github.com/omneity-labs/semango/internal/util"
// ...
// util.Logger.Info("Something happened", "key", "value")
// or if SetDefault was called:
// slog.Info("Something happened", "key", "value")
