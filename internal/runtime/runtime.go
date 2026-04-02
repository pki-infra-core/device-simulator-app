package runtime

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"device-identity-sdk/platform/file"
	"device-identity-sdk/sdk"
	"device-simulator-app/internal/app"
	"device-simulator-app/internal/config"
	"device-simulator-app/internal/gateway"
)

type Factory struct {
	logger *slog.Logger
}

func NewFactory(logger *slog.Logger) *Factory {
	return &Factory{logger: logger}
}

func (f *Factory) Build(cfg config.Config) (*app.Dependencies, error) {
	store, err := file.NewStore(cfg.IdentityStorageDir)
	if err != nil {
		return nil, fmt.Errorf("create identity store: %w", err)
	}

	client, err := sdk.New(sdk.Config{
		Store:              store,
		Issuer:             NewIssuerClient(cfg.IssuerBaseURL, cfg.BootstrapToken, cfg.HTTPTimeout),
		PassphraseProvider: sdk.NewStaticPassphraseProvider([]byte(cfg.IdentityPassphrase)),
		Logger:             f.logger,
	})
	if err != nil {
		return nil, fmt.Errorf("create sdk client: %w", err)
	}

	rootCAs, err := gateway.LoadRootCAs(cfg.GatewayCAFile)
	if err != nil {
		return nil, err
	}

	return &app.Dependencies{
		SDK: client,
		Gateway: gateway.New(
			cfg.GatewayBaseURL,
			cfg.HTTPTimeout,
			rootCAs,
			cfg.GatewayServerName,
		),
	}, nil
}

func NewLogger(level string) *slog.Logger {
	var slogLevel slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slogLevel}))
}
