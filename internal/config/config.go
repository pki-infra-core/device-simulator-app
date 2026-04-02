package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultHTTPTimeout     = 10 * time.Second
	defaultIdentityDir     = "./identity"
	defaultProfile         = "default"
	defaultLogLevel        = "info"
	defaultRenewBefore     = 72 * time.Hour
	defaultTelemetryStatus = "nominal"
	defaultFirmwareVersion = "simulator-1.0.0"
	defaultGatewayPingPath = "/api/v1/ping"
	defaultTelemetryPath   = "/api/v1/telemetry"
	defaultTemperature     = 21.5
	defaultBatteryPct      = 87
)

type Config struct {
	DeviceID                 string
	TenantID                 string
	DeviceModel              string
	Profile                  string
	IssuerBaseURL            string
	GatewayBaseURL           string
	BootstrapToken           string
	IdentityStorageDir       string
	IdentityPassphrase       string
	HTTPTimeout              time.Duration
	LogLevel                 string
	ForceBootstrap           bool
	RenewBefore              time.Duration
	GatewayCAFile            string
	GatewayServerName        string
	TelemetryStatus          string
	TelemetryFirmwareVersion string
	TelemetryTemperature     float64
	TelemetryBatteryPct      int
	PingPath                 string
	TelemetryPath            string
}

type Args struct {
	Command   string
	Force     bool
	RotateKey bool
	Help      bool
}

func Load(argv []string) (Config, Args, error) {
	if len(argv) == 0 {
		return Config{}, Args{}, errors.New(usage())
	}

	cfg := Config{
		DeviceID:                 os.Getenv("DEVICE_ID"),
		TenantID:                 os.Getenv("TENANT_ID"),
		DeviceModel:              os.Getenv("DEVICE_MODEL"),
		Profile:                  envOrDefault("DEVICE_PROFILE", defaultProfile),
		IssuerBaseURL:            os.Getenv("ISSUER_BASE_URL"),
		GatewayBaseURL:           os.Getenv("GATEWAY_BASE_URL"),
		BootstrapToken:           os.Getenv("BOOTSTRAP_TOKEN"),
		IdentityStorageDir:       envOrDefault("IDENTITY_STORAGE_DIR", defaultIdentityDir),
		IdentityPassphrase:       os.Getenv("DEVICE_IDENTITY_PASSPHRASE"),
		HTTPTimeout:              durationOrDefault("HTTP_TIMEOUT", defaultHTTPTimeout),
		LogLevel:                 envOrDefault("LOG_LEVEL", defaultLogLevel),
		ForceBootstrap:           boolOrDefault("FORCE_BOOTSTRAP", false),
		RenewBefore:              durationOrDefault("RENEW_BEFORE", defaultRenewBefore),
		GatewayCAFile:            os.Getenv("GATEWAY_CA_FILE"),
		GatewayServerName:        os.Getenv("GATEWAY_SERVER_NAME"),
		TelemetryStatus:          envOrDefault("TELEMETRY_STATUS", defaultTelemetryStatus),
		TelemetryFirmwareVersion: envOrDefault("FIRMWARE_VERSION", defaultFirmwareVersion),
		TelemetryTemperature:     floatOrDefault("TELEMETRY_TEMPERATURE", defaultTemperature),
		TelemetryBatteryPct:      intOrDefault("TELEMETRY_BATTERY_PCT", defaultBatteryPct),
		PingPath:                 envOrDefault("PING_PATH", defaultGatewayPingPath),
		TelemetryPath:            envOrDefault("TELEMETRY_PATH", defaultTelemetryPath),
	}

	args, err := parseArgs(argv)
	if err != nil {
		return Config{}, Args{}, err
	}
	if args.Help {
		return Config{}, Args{}, errors.New(usage())
	}
	if err := validate(cfg, args); err != nil {
		return Config{}, Args{}, err
	}

	return cfg, args, nil
}

func parseArgs(argv []string) (Args, error) {
	command := argv[0]
	args := Args{Command: command}

	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.BoolVar(&args.Help, "help", false, "show help")

	switch command {
	case "bootstrap":
		fs.BoolVar(&args.Force, "force", false, "reset local identity before bootstrap")
	case "renew":
		fs.BoolVar(&args.RotateKey, "rotate-key", false, "rotate local key during renewal")
	case "status", "ping", "telemetry", "reset":
	default:
		return Args{}, fmt.Errorf("unsupported command %q\n\n%s", command, usage())
	}

	if err := fs.Parse(argv[1:]); err != nil {
		return Args{}, err
	}
	return args, nil
}

func validate(cfg Config, args Args) error {
	required := map[string]string{}

	switch args.Command {
	case "bootstrap", "renew":
		required["DEVICE_ID"] = cfg.DeviceID
		required["TENANT_ID"] = cfg.TenantID
		required["DEVICE_MODEL"] = cfg.DeviceModel
		required["IDENTITY_STORAGE_DIR"] = cfg.IdentityStorageDir
		required["DEVICE_IDENTITY_PASSPHRASE"] = cfg.IdentityPassphrase
		required["ISSUER_BASE_URL"] = cfg.IssuerBaseURL
		required["BOOTSTRAP_TOKEN"] = cfg.BootstrapToken
	case "status", "reset":
		required["IDENTITY_STORAGE_DIR"] = cfg.IdentityStorageDir
		required["DEVICE_IDENTITY_PASSPHRASE"] = cfg.IdentityPassphrase
	case "ping", "telemetry":
		required["IDENTITY_STORAGE_DIR"] = cfg.IdentityStorageDir
		required["DEVICE_IDENTITY_PASSPHRASE"] = cfg.IdentityPassphrase
		required["GATEWAY_BASE_URL"] = cfg.GatewayBaseURL
		if args.Command == "telemetry" {
			required["DEVICE_ID"] = cfg.DeviceID
		}
	}

	var missing []string
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	if cfg.HTTPTimeout <= 0 {
		return errors.New("HTTP_TIMEOUT must be greater than 0")
	}
	if cfg.RenewBefore < 0 {
		return errors.New("RENEW_BEFORE must not be negative")
	}
	return nil
}

func usage() string {
	return strings.TrimSpace(`device-simulator-app

Usage:
  device-simulator <command> [flags]

Commands:
  bootstrap   Enroll and persist a new device identity
  status      Inspect stored identity status
  ping        Call the protected mTLS ping endpoint
  telemetry   Send fake telemetry over mTLS
  renew       Renew the existing identity
  reset       Remove local identity state

Flags:
  bootstrap --force
  renew --rotate-key`)
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationOrDefault(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func boolOrDefault(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes"
}

func intOrDefault(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return fallback
	}
	return parsed
}

func floatOrDefault(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	var parsed float64
	if _, err := fmt.Sscanf(value, "%f", &parsed); err != nil {
		return fallback
	}
	return parsed
}
