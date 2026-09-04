package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func debugJSONLogger(output *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

func decodeLogEntries(t *testing.T, data []byte) []map[string]any {
	t.Helper()

	decoder := json.NewDecoder(bytes.NewReader(data))
	entries := make([]map[string]any, 0)

	for {
		var entry map[string]any
		err := decoder.Decode(&entry)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode log entry: %v", err)
		}

		entries = append(entries, entry)
	}

	return entries
}

func assertLogField(
	t *testing.T,
	entry map[string]any,
	field string,
	want any,
) {
	t.Helper()

	if entry[field] != want {
		t.Fatalf(
			"log field %q = %v, want %v",
			field,
			entry[field],
			want,
		)
	}
}
