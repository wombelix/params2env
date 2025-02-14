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
		{
			name:      "debug level",
			level:     "debug",
			wantLevel: slog.LevelDebug,
		},
		{
			name:      "info level",
			level:     "info",
			wantLevel: slog.LevelInfo,
		},
		{
			name:      "warn level",
			level:     "warn",
			wantLevel: slog.LevelWarn,
		},
		{
			name:      "error level",
			level:     "error",
			wantLevel: slog.LevelError,
		},
		{
			name:      "invalid level defaults to info",
			level:     "invalid",
			wantLevel: slog.LevelInfo,
		},
		{
			name:      "empty level defaults to info",
			level:     "",
			wantLevel: slog.LevelInfo,
		},
		{
			name:      "case insensitive level",
			level:     "DEBUG",
			wantLevel: slog.LevelDebug,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := InitLogger(tt.level)
			if logger == nil {
				t.Error("InitLogger() returned nil")
				return
			}

			// Get the handler from the logger
			handler, ok := logger.Handler().(*slog.TextHandler)
			if !ok {
				t.Error("Logger handler is not TextHandler")
				return
			}

			// Check if the level is set correctly
			// Note: slog doesn't provide a direct way to get the level from the handler
			// We can only verify that the logger is created successfully
			// and that it responds to the appropriate level
			if !handler.Enabled(context.Background(), tt.wantLevel) {
				t.Errorf("Logger level %v not enabled for wanted level %v", tt.level, tt.wantLevel)
			}
		})
	}
}
