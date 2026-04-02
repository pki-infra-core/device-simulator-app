package gateway

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"strings"
	"testing"

	"device-identity-sdk/sdk"
)

func Test텔레메트리_요청을_JSON으로_보낸다(t *testing.T) {
	t.Parallel()

	var gotMethod string
	var gotPath string
	var gotBody string

	client := New("https://gateway.example", 0, nil, "")
	client.WithHTTPDoer(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		gotMethod = req.Method
		gotPath = req.URL.Path
		body, _ := io.ReadAll(req.Body)
		gotBody = string(body)
		return &http.Response{
			StatusCode: 202,
			Body:       io.NopCloser(strings.NewReader(`{"accepted":true}`)),
			Header:     make(http.Header),
		}, nil
	}))

	_, err := client.SendTelemetry(context.Background(), tlsProviderStub{}, "/api/v1/telemetry", TelemetryPayload{
		DeviceID:        "device-001",
		FirmwareVersion: "simulator-1.0.0",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("unexpected method %q", gotMethod)
	}
	if gotPath != "/api/v1/telemetry" {
		t.Fatalf("unexpected path %q", gotPath)
	}
	if !strings.Contains(gotBody, `"device_id":"device-001"`) {
		t.Fatalf("unexpected body %q", gotBody)
	}
}

type tlsProviderStub struct{}

func (tlsProviderStub) TLSConfig(context.Context, sdk.TLSConfigOptions) (*tls.Config, error) {
	return &tls.Config{}, nil
}

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}
