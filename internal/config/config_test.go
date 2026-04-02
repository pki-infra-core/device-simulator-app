package config

import (
	"os"
	"testing"
	"time"
)

func Test환경변수와_커맨드를_읽는다(t *testing.T) {
	t.Setenv("DEVICE_ID", "device-001")
	t.Setenv("TENANT_ID", "tenant-a")
	t.Setenv("DEVICE_MODEL", "sensor-v1")
	t.Setenv("ISSUER_BASE_URL", "https://issuer.example")
	t.Setenv("BOOTSTRAP_TOKEN", "token-001")
	t.Setenv("IDENTITY_STORAGE_DIR", "./identity")
	t.Setenv("DEVICE_IDENTITY_PASSPHRASE", "secret")
	t.Setenv("HTTP_TIMEOUT", "12s")

	cfg, args, err := Load([]string{"bootstrap", "--force"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.DeviceID != "device-001" {
		t.Fatalf("unexpected device id %q", cfg.DeviceID)
	}
	if cfg.HTTPTimeout != 12*time.Second {
		t.Fatalf("unexpected timeout %s", cfg.HTTPTimeout)
	}
	if args.Command != "bootstrap" || !args.Force {
		t.Fatalf("unexpected args %+v", args)
	}
}

func Test필수값이_없으면_오류를_반환한다(t *testing.T) {
	originalEnv := os.Environ()
	for _, entry := range originalEnv {
		pair := splitEnv(entry)
		if pair[0] != "" {
			t.Setenv(pair[0], "")
		}
	}

	_, _, err := Load([]string{"status"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func splitEnv(entry string) [2]string {
	for i := 0; i < len(entry); i++ {
		if entry[i] == '=' {
			return [2]string{entry[:i], entry[i+1:]}
		}
	}
	return [2]string{}
}
