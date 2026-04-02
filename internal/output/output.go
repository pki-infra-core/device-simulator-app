package output

import (
	"fmt"
	"io"
	"strings"
	"time"
)

type Console struct {
	out io.Writer
}

func NewConsole(out io.Writer) *Console {
	return &Console{out: out}
}

func (c *Console) PrintSection(title string, fields map[string]string) error {
	if _, err := fmt.Fprintf(c.out, "%s\n", title); err != nil {
		return err
	}

	order := []string{
		"status",
		"device_id",
		"tenant_id",
		"model",
		"profile",
		"serial_number",
		"not_before",
		"not_after",
		"validity",
		"fingerprint",
		"storage_path",
		"http_status",
		"response",
	}

	seen := make(map[string]bool, len(fields))
	for _, key := range order {
		value, ok := fields[key]
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		seen[key] = true
		if _, err := fmt.Fprintf(c.out, "  %-14s %s\n", key+":", value); err != nil {
			return err
		}
	}
	for key, value := range fields {
		if seen[key] || strings.TrimSpace(value) == "" {
			continue
		}
		if _, err := fmt.Fprintf(c.out, "  %-14s %s\n", key+":", value); err != nil {
			return err
		}
	}
	return nil
}

func FormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func FormatValidityWindow(notBefore, notAfter time.Time) string {
	if notBefore.IsZero() || notAfter.IsZero() {
		return ""
	}
	return fmt.Sprintf("%s -> %s", FormatTime(notBefore), FormatTime(notAfter))
}
