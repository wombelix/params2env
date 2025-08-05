// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

package logger

import (
	"context"
	"log/slog"
	"testing"
)

func TestInitLogger(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		wantLevel slog.Level
	}{
		{"debug level", "debug", slog.LevelDebug},
		{"info level", "info", slog.LevelInfo},
		{"warn level", "warn", slog.LevelWarn},
		{"error level", "error", slog.LevelError},
		{"invalid level defaults to info", "invalid", slog.LevelInfo},
		{"empty level defaults to info", "", slog.LevelInfo},
		{"case insensitive level", "DEBUG", slog.LevelDebug},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testLoggerLevel(t, tt.level, tt.wantLevel)
		})
	}
}

func testLoggerLevel(t *testing.T, level string, wantLevel slog.Level) {
	logger := InitLogger(level)
	if logger == nil {
		t.Error("InitLogger() returned nil")
		return
	}

	handler, ok := logger.Handler().(*slog.TextHandler)
	if !ok {
		t.Error("Logger handler is not TextHandler")
		return
	}

	if !handler.Enabled(context.Background(), wantLevel) {
		t.Errorf("Logger level %v not enabled for wanted level %v", level, wantLevel)
	}
}
