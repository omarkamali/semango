package util

import (
	"fmt"
	"log/slog"
	"runtime"
	"strings"
)

// SemangoError is a custom error type for adding context and stack traces.
type SemangoError struct {
	OriginalErr error
	Message     string
	Stack       string
	Attrs       []slog.Attr
}

// Error returns the error message.
func (e *SemangoError) Error() string {
	if e.OriginalErr != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.OriginalErr)
	}
	return e.Message
}

// Unwrap returns the underlying error.
func (e *SemangoError) Unwrap() error {
	return e.OriginalErr
}

const maxStackLength = 8192 // Max length of stack trace to capture

// NewError creates a new SemangoError without an original error.
func NewError(message string, attrs ...slog.Attr) *SemangoError {
	return newSemangoError(nil, message, attrs...)
}

// WrapError creates a new SemangoError, wrapping an existing error.
func WrapError(err error, message string, attrs ...slog.Attr) *SemangoError {
	if err == nil {
		return newSemangoError(nil, message, attrs...)
	}
	return newSemangoError(err, message, attrs...)
}

func newSemangoError(originalErr error, message string, attrs ...slog.Attr) *SemangoError {
	buf := make([]byte, maxStackLength)
	n := runtime.Stack(buf, false)
	stack := string(buf[:n])

	// Optional: Clean up the stack trace a bit, e.g., remove GOROOT functions
	// stack = cleanStack(stack)


	// If the original error is already a SemangoError, append attrs and message, but keep original stack.
	if se, ok := originalErr.(*SemangoError); ok {
		// Prepend message, inherit stack and original error from `se`
		// Combine attributes
		combinedAttrs := append(se.Attrs, attrs...) // New attrs take precedence if keys conflict, slog handles this.
		
		newMessage := message
		if se.Message != "" {
			newMessage = fmt.Sprintf("%s: %s", message, se.Message)
		}

		return &SemangoError{
			OriginalErr: se.OriginalErr, // Keep the root cause
			Message:     newMessage,
			Stack:       se.Stack, // Keep the original stack where the error was first wrapped
			Attrs:       combinedAttrs,
		}
	}


	return &SemangoError{
		OriginalErr: originalErr,
		Message:     message,
		Stack:       stack,
		Attrs:       attrs,
	}
}

// LogError logs a SemangoError with its structured context and stack trace.
// If the error is not a SemangoError, it logs it as a standard error message.
func LogError(logger *slog.Logger, err error) {
	if err == nil {
		return
	}

	var se *SemangoError
	if asSe, ok := err.(*SemangoError); ok {
		se = asSe
	} else if asWrapper, ok := err.(interface{ Unwrap() error }); ok {
		// Check if it wraps a SemangoError
		unwrapped := asWrapper.Unwrap()
		if unwrapSe, okUnwrap := unwrapped.(*SemangoError); okUnwrap {
			se = unwrapSe
		}
	}


	if se != nil {
		logAttrs := []any{
			slog.String("error_message", se.Message),
		}
		if se.OriginalErr != nil {
			logAttrs = append(logAttrs, slog.String("original_error", se.OriginalErr.Error()))
		}
		logAttrs = append(logAttrs, slog.String("stack_trace", se.Stack))


		for _, attr := range se.Attrs {
			logAttrs = append(logAttrs, attr)
		}
		logger.Error("An error occurred", logAttrs...)
	} else {
		// Fallback for non-SemangoError types
		logger.Error("An error occurred", slog.String("error", err.Error()))
	}
}

// Helper function to potentially clean stack traces (example)
// Not strictly necessary but can make logs cleaner.
func cleanStack(stack string) string {
	lines := strings.Split(stack, "\n")
	var cleanedLines []string
	goroot := runtime.GOROOT()
	for _, line := range lines {
		if !strings.Contains(line, goroot) { // Remove lines originating from GOROOT
			cleanedLines = append(cleanedLines, line)
		}
	}
	return strings.Join(cleanedLines, "\n")
} 