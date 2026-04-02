package gateway

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"device-identity-sdk/sdk"
)

type TLSConfigProvider interface {
	TLSConfig(ctx context.Context, opts sdk.TLSConfigOptions) (*tls.Config, error)
}

type Client struct {
	baseURL        string
	timeout        time.Duration
	rootCAs        *x509.CertPool
	serverName     string
	httpDoer       HTTPDoer
	transportMaker TransportMaker
}

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type TransportMaker func(tlsConfig *tls.Config) http.RoundTripper

type PingResult struct {
	StatusCode int
	Body       string
}

type TelemetryPayload struct {
	DeviceID        string    `json:"device_id"`
	SentAt          time.Time `json:"sent_at"`
	Temperature     float64   `json:"temperature"`
	BatteryPct      int       `json:"battery_pct"`
	FirmwareVersion string    `json:"firmware_version"`
	Status          string    `json:"status"`
}

func New(baseURL string, timeout time.Duration, rootCAs *x509.CertPool, serverName string) *Client {
	return &Client{
		baseURL:        strings.TrimRight(baseURL, "/"),
		timeout:        timeout,
		rootCAs:        rootCAs,
		serverName:     serverName,
		transportMaker: defaultTransportMaker,
	}
}

func (c *Client) WithHTTPDoer(doer HTTPDoer) *Client {
	c.httpDoer = doer
	return c
}

func (c *Client) Ping(ctx context.Context, provider TLSConfigProvider, path string) (*PingResult, error) {
	client, err := c.httpClient(ctx, provider)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build ping request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call ping endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, fmt.Errorf("read ping response: %w", err)
	}
	return &PingResult{
		StatusCode: resp.StatusCode,
		Body:       strings.TrimSpace(string(body)),
	}, nil
}

func (c *Client) SendTelemetry(ctx context.Context, provider TLSConfigProvider, path string, payload TelemetryPayload) (*PingResult, error) {
	client, err := c.httpClient(ctx, provider)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode telemetry payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build telemetry request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call telemetry endpoint: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, fmt.Errorf("read telemetry response: %w", err)
	}
	return &PingResult{
		StatusCode: resp.StatusCode,
		Body:       strings.TrimSpace(string(respBody)),
	}, nil
}

func LoadRootCAs(caFile string) (*x509.CertPool, error) {
	if strings.TrimSpace(caFile) == "" {
		return x509.SystemCertPool()
	}

	pemBytes, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read gateway ca file: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil, fmt.Errorf("parse gateway ca file: no certificates found")
	}
	return pool, nil
}

func (c *Client) httpClient(ctx context.Context, provider TLSConfigProvider) (HTTPDoer, error) {
	if c.httpDoer != nil {
		return c.httpDoer, nil
	}

	tlsConfig, err := provider.TLSConfig(ctx, sdk.TLSConfigOptions{
		RootCAs:    c.rootCAs,
		ServerName: c.serverName,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return nil, fmt.Errorf("build tls config: %w", err)
	}

	return &http.Client{
		Timeout:   c.timeout,
		Transport: c.transportMaker(tlsConfig),
	}, nil
}

func defaultTransportMaker(tlsConfig *tls.Config) http.RoundTripper {
	return &http.Transport{TLSClientConfig: tlsConfig}
}
