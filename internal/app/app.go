package app

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"device-identity-sdk/sdk"
	"device-simulator-app/internal/config"
	"device-simulator-app/internal/gateway"
	"device-simulator-app/internal/output"
)

type runtimeFactory interface {
	Build(cfg config.Config) (*Dependencies, error)
}

type Dependencies struct {
	SDK     SDKClient
	Gateway GatewayClient
}

type SDKClient interface {
	Bootstrap(ctx context.Context, req sdk.BootstrapRequest) (*sdk.BootstrapResult, error)
	LoadIdentity(ctx context.Context) (*sdk.Identity, error)
	ValidateIdentity(ctx context.Context) error
	TLSConfig(ctx context.Context, opts sdk.TLSConfigOptions) (*tls.Config, error)
	Renew(ctx context.Context, req sdk.RenewRequest) (*sdk.RenewResult, error)
}

type GatewayClient interface {
	Ping(ctx context.Context, provider gateway.TLSConfigProvider, path string) (*gateway.PingResult, error)
	SendTelemetry(ctx context.Context, provider gateway.TLSConfigProvider, path string, payload gateway.TelemetryPayload) (*gateway.PingResult, error)
}

type Runner struct {
	factory          runtimeFactory
	console          *output.Console
	logger           *slog.Logger
	now              func() time.Time
	issuerReadyCheck func(ctx context.Context, cfg config.Config) error
}

func NewRunner(factory runtimeFactory, console *output.Console, logger *slog.Logger) *Runner {
	return &Runner{
		factory:          factory,
		console:          console,
		logger:           logger,
		now:              time.Now,
		issuerReadyCheck: ensureIssuerReady,
	}
}

func (r *Runner) Run(ctx context.Context, cfg config.Config, args config.Args) error {
	switch args.Command {
	case "reset":
		return r.runReset(cfg)
	}

	deps, err := r.factory.Build(cfg)
	if err != nil {
		return err
	}

	switch args.Command {
	case "bootstrap":
		return r.runBootstrap(ctx, cfg, deps.SDK, args)
	case "status":
		return r.runStatus(ctx, cfg, deps.SDK)
	case "ping":
		return r.runPing(ctx, cfg, deps.SDK, deps.Gateway)
	case "telemetry":
		return r.runTelemetry(ctx, cfg, deps.SDK, deps.Gateway)
	case "renew":
		return r.runRenew(ctx, cfg, deps.SDK, args)
	default:
		return fmt.Errorf("unsupported command %q", args.Command)
	}
}

func (r *Runner) runBootstrap(ctx context.Context, cfg config.Config, client SDKClient, args config.Args) error {
	if err := r.issuerReadyCheck(ctx, cfg); err != nil {
		return err
	}

	if cfg.ForceBootstrap || args.Force {
		if err := resetStorage(cfg.IdentityStorageDir); err != nil {
			return err
		}
	}

	result, err := client.Bootstrap(ctx, sdk.BootstrapRequest{
		DeviceID: cfg.DeviceID,
		TenantID: cfg.TenantID,
		Model:    cfg.DeviceModel,
		Profile:  cfg.Profile,
		Subject: sdk.CSRSubject{
			CommonName:   expectedCommonName(cfg.TenantID, cfg.DeviceID),
			Organization: []string{cfg.TenantID},
		},
	})
	if err != nil {
		return err
	}

	return r.console.PrintSection("bootstrap success", map[string]string{
		"device_id":     cfg.DeviceID,
		"serial_number": result.Identity.Metadata.SerialNumber,
		"validity":      output.FormatValidityWindow(result.Identity.Metadata.NotBefore, result.Identity.Metadata.NotAfter),
		"fingerprint":   result.Identity.Metadata.FingerprintSHA256,
	})
}

func (r *Runner) runStatus(ctx context.Context, cfg config.Config, client SDKClient) error {
	identity, err := client.LoadIdentity(ctx)
	if err != nil {
		if errors.Is(err, sdk.ErrIdentityNotFound) {
			return r.console.PrintSection("identity status", map[string]string{
				"status":       "NOT_FOUND",
				"storage_path": cfg.IdentityStorageDir,
			})
		}
		return err
	}

	status := "READY"
	if err := client.ValidateIdentity(ctx); err != nil {
		status = "INVALID"
		r.logger.Warn("identity validation failed", slog.String("error", err.Error()))
	} else if r.now().Add(cfg.RenewBefore).After(identity.Metadata.NotAfter) {
		status = "EXPIRING_SOON"
	}

	return r.console.PrintSection("identity status", map[string]string{
		"status":        status,
		"device_id":     identity.Metadata.DeviceID,
		"tenant_id":     identity.Metadata.TenantID,
		"model":         identity.Metadata.Model,
		"profile":       identity.Metadata.Profile,
		"serial_number": identity.Metadata.SerialNumber,
		"not_before":    output.FormatTime(identity.Metadata.NotBefore),
		"not_after":     output.FormatTime(identity.Metadata.NotAfter),
		"fingerprint":   identity.Metadata.FingerprintSHA256,
		"storage_path":  cfg.IdentityStorageDir,
	})
}

func (r *Runner) runPing(ctx context.Context, cfg config.Config, client SDKClient, gatewayClient GatewayClient) error {
	result, err := gatewayClient.Ping(ctx, client, cfg.PingPath)
	if err != nil {
		return err
	}

	return r.console.PrintSection("ping result", map[string]string{
		"status":      statusFromCode(result.StatusCode),
		"http_status": fmt.Sprintf("%d", result.StatusCode),
		"response":    result.Body,
	})
}

func (r *Runner) runTelemetry(ctx context.Context, cfg config.Config, client SDKClient, gatewayClient GatewayClient) error {
	payload := gateway.TelemetryPayload{
		DeviceID:        cfg.DeviceID,
		SentAt:          r.now().UTC(),
		Temperature:     cfg.TelemetryTemperature,
		BatteryPct:      cfg.TelemetryBatteryPct,
		FirmwareVersion: cfg.TelemetryFirmwareVersion,
		Status:          cfg.TelemetryStatus,
	}

	result, err := gatewayClient.SendTelemetry(ctx, client, cfg.TelemetryPath, payload)
	if err != nil {
		return err
	}

	return r.console.PrintSection("telemetry result", map[string]string{
		"status":      statusFromCode(result.StatusCode),
		"device_id":   payload.DeviceID,
		"http_status": fmt.Sprintf("%d", result.StatusCode),
		"response":    result.Body,
	})
}

func (r *Runner) runRenew(ctx context.Context, cfg config.Config, client SDKClient, args config.Args) error {
	if err := r.issuerReadyCheck(ctx, cfg); err != nil {
		return err
	}

	result, err := client.Renew(ctx, sdk.RenewRequest{
		DeviceID:  cfg.DeviceID,
		TenantID:  cfg.TenantID,
		Model:     cfg.DeviceModel,
		Profile:   cfg.Profile,
		RotateKey: args.RotateKey,
		Subject: sdk.CSRSubject{
			CommonName:   expectedCommonName(cfg.TenantID, cfg.DeviceID),
			Organization: []string{cfg.TenantID},
		},
	})
	if err != nil {
		return err
	}

	return r.console.PrintSection("renew success", map[string]string{
		"device_id":     cfg.DeviceID,
		"serial_number": result.Identity.Metadata.SerialNumber,
		"validity":      output.FormatValidityWindow(result.Identity.Metadata.NotBefore, result.Identity.Metadata.NotAfter),
		"fingerprint":   result.Identity.Metadata.FingerprintSHA256,
	})
}

func (r *Runner) runReset(cfg config.Config) error {
	if err := resetStorage(cfg.IdentityStorageDir); err != nil {
		return err
	}
	return r.console.PrintSection("reset complete", map[string]string{
		"status":       "NOT_FOUND",
		"storage_path": cfg.IdentityStorageDir,
	})
}

func resetStorage(dir string) error {
	cleanDir := filepath.Clean(dir)
	if cleanDir == "." || cleanDir == "/" {
		return fmt.Errorf("refusing to reset unsafe storage path %q", dir)
	}
	if err := os.RemoveAll(cleanDir); err != nil {
		return fmt.Errorf("remove identity storage: %w", err)
	}
	return nil
}

func statusFromCode(code int) string {
	if code >= 200 && code < 300 {
		return "READY"
	}
	return "FAILED"
}

func expectedCommonName(tenantID, deviceID string) string {
	return tenantID + ":" + deviceID
}

func ensureIssuerReady(ctx context.Context, cfg config.Config) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.IssuerBaseURL+"/readyz", nil)
	if err != nil {
		return fmt.Errorf("build issuer readiness request: %w", err)
	}

	client := &http.Client{Timeout: cfg.HTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("issuer is not reachable at %s/readyz: %w", cfg.IssuerBaseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("issuer is not ready at %s/readyz: status %d", cfg.IssuerBaseURL, resp.StatusCode)
	}
	return nil
}
