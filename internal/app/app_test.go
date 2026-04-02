package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"testing"
	"time"

	"device-identity-sdk/sdk"
	"device-simulator-app/internal/config"
	"device-simulator-app/internal/gateway"
	"device-simulator-app/internal/output"
)

func Test부트스트랩_요약을_출력한다(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	runner := NewRunner(factoryStub{
		deps: &Dependencies{
			SDK: bootstrapSDKStub{
				result: &sdk.BootstrapResult{
					Identity: &sdk.Identity{
						Metadata: sdk.IdentityMetadata{
							SerialNumber:      "serial-001",
							NotBefore:         time.Date(2026, 3, 26, 0, 0, 0, 0, time.UTC),
							NotAfter:          time.Date(2027, 3, 26, 0, 0, 0, 0, time.UTC),
							FingerprintSHA256: "fp-001",
						},
					},
				},
			},
			Gateway: gatewayStub{},
		},
	}, output.NewConsole(&out), slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	runner.issuerReadyCheck = func(context.Context, config.Config) error { return nil }

	cfg := baseConfig()
	err := runner.Run(context.Background(), cfg, config.Args{Command: "bootstrap"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got := out.String()
	assertContains(t, got, "bootstrap success")
	assertContains(t, got, "device_id:")
	assertContains(t, got, "serial-001")
	assertContains(t, got, "fp-001")
}

func Test상태_아이덴티티가_없으면_NOT_FOUND를_출력한다(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	runner := NewRunner(factoryStub{
		deps: &Dependencies{
			SDK: identitySDKStub{
				loadErr: sdk.ErrIdentityNotFound,
			},
			Gateway: gatewayStub{},
		},
	}, output.NewConsole(&out), slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	runner.issuerReadyCheck = func(context.Context, config.Config) error { return nil }

	err := runner.Run(context.Background(), baseConfig(), config.Args{Command: "status"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertContains(t, out.String(), "NOT_FOUND")
}

func Test상태_만료가_가까우면_EXPIRING_SOON을_출력한다(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	runner := NewRunner(factoryStub{
		deps: &Dependencies{
			SDK: identitySDKStub{
				identity: &sdk.Identity{
					Metadata: sdk.IdentityMetadata{
						DeviceID:          "device-001",
						TenantID:          "tenant-a",
						Model:             "sensor-v1",
						SerialNumber:      "serial-001",
						NotBefore:         time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
						NotAfter:          time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC),
						FingerprintSHA256: "fp-001",
					},
				},
			},
			Gateway: gatewayStub{},
		},
	}, output.NewConsole(&out), slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	runner.now = func() time.Time {
		return time.Date(2026, 3, 26, 0, 0, 0, 0, time.UTC)
	}
	runner.issuerReadyCheck = func(context.Context, config.Config) error { return nil }

	err := runner.Run(context.Background(), baseConfig(), config.Args{Command: "status"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertContains(t, out.String(), "EXPIRING_SOON")
}

func Test핑_성공_응답을_출력한다(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	runner := NewRunner(factoryStub{
		deps: &Dependencies{
			SDK: identitySDKStub{},
			Gateway: gatewayStub{
				pingResult: &gateway.PingResult{
					StatusCode: 200,
					Body:       `{"ok":true}`,
				},
			},
		},
	}, output.NewConsole(&out), slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	runner.issuerReadyCheck = func(context.Context, config.Config) error { return nil }

	err := runner.Run(context.Background(), baseConfig(), config.Args{Command: "ping"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertContains(t, out.String(), "ping result")
	assertContains(t, out.String(), "200")
	assertContains(t, out.String(), `{"ok":true}`)
}

func Test핑_게이트웨이_실패를_반환한다(t *testing.T) {
	t.Parallel()

	runner := NewRunner(factoryStub{
		deps: &Dependencies{
			SDK: identitySDKStub{},
			Gateway: gatewayStub{
				pingErr: errors.New("tls handshake failed"),
			},
		},
	}, output.NewConsole(&bytes.Buffer{}), slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	runner.issuerReadyCheck = func(context.Context, config.Config) error { return nil }

	err := runner.Run(context.Background(), baseConfig(), config.Args{Command: "ping"})
	if err == nil {
		t.Fatal("expected error")
	}
}

type factoryStub struct {
	deps *Dependencies
	err  error
}

func (f factoryStub) Build(config.Config) (*Dependencies, error) {
	return f.deps, f.err
}

type bootstrapSDKStub struct {
	result *sdk.BootstrapResult
	err    error
}

func (s bootstrapSDKStub) Bootstrap(context.Context, sdk.BootstrapRequest) (*sdk.BootstrapResult, error) {
	return s.result, s.err
}

func (s bootstrapSDKStub) LoadIdentity(context.Context) (*sdk.Identity, error) {
	return nil, nil
}

func (s bootstrapSDKStub) ValidateIdentity(context.Context) error {
	return nil
}

func (s bootstrapSDKStub) TLSConfig(context.Context, sdk.TLSConfigOptions) (*tls.Config, error) {
	return nil, nil
}

func (s bootstrapSDKStub) Renew(context.Context, sdk.RenewRequest) (*sdk.RenewResult, error) {
	return nil, nil
}

type identitySDKStub struct {
	identity    *sdk.Identity
	loadErr     error
	validateErr error
	renewResult *sdk.RenewResult
	renewErr    error
}

func (s identitySDKStub) Bootstrap(context.Context, sdk.BootstrapRequest) (*sdk.BootstrapResult, error) {
	return nil, nil
}

func (s identitySDKStub) LoadIdentity(context.Context) (*sdk.Identity, error) {
	return s.identity, s.loadErr
}

func (s identitySDKStub) ValidateIdentity(context.Context) error {
	return s.validateErr
}

func (s identitySDKStub) TLSConfig(context.Context, sdk.TLSConfigOptions) (*tls.Config, error) {
	return &tls.Config{}, nil
}

func (s identitySDKStub) Renew(context.Context, sdk.RenewRequest) (*sdk.RenewResult, error) {
	return s.renewResult, s.renewErr
}

type gatewayStub struct {
	pingResult      *gateway.PingResult
	pingErr         error
	telemetryResult *gateway.PingResult
	telemetryErr    error
}

func (g gatewayStub) Ping(context.Context, gateway.TLSConfigProvider, string) (*gateway.PingResult, error) {
	return g.pingResult, g.pingErr
}

func (g gatewayStub) SendTelemetry(context.Context, gateway.TLSConfigProvider, string, gateway.TelemetryPayload) (*gateway.PingResult, error) {
	return g.telemetryResult, g.telemetryErr
}

func baseConfig() config.Config {
	return config.Config{
		DeviceID:           "device-001",
		TenantID:           "tenant-a",
		DeviceModel:        "sensor-v1",
		IssuerBaseURL:      "http://issuer.test",
		IdentityStorageDir: "/tmp/device-simulator-test",
		HTTPTimeout:        5 * time.Second,
		RenewBefore:        72 * time.Hour,
		PingPath:           "/api/v1/ping",
		TelemetryPath:      "/api/v1/telemetry",
	}
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !bytes.Contains([]byte(got), []byte(want)) {
		t.Fatalf("expected %q to contain %q", got, want)
	}
}
