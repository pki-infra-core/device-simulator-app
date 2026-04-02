package output

import (
	"bytes"
	"testing"
	"time"
)

func Test섹션_출력을_정렬한다(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	console := NewConsole(&out)

	err := console.PrintSection("identity status", map[string]string{
		"status":      "READY",
		"device_id":   "device-001",
		"fingerprint": "fp-001",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got := out.String()
	if !bytes.Contains([]byte(got), []byte("identity status")) {
		t.Fatalf("unexpected output %q", got)
	}
}

func Test유효기간_문자열을_만든다(t *testing.T) {
	t.Parallel()

	got := FormatValidityWindow(
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	)
	if got == "" {
		t.Fatal("expected formatted validity window")
	}
}
