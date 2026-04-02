package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"device-identity-sdk/sdk"
)

const issuePath = "/api/v1/device-certificates:issue"

type issuerClient struct {
	baseURL string
	token   string
	client  *http.Client
}

type issueRequestDTO struct {
	DeviceID string `json:"device_id"`
	TenantID string `json:"tenant_id"`
	Model    string `json:"model"`
	Profile  string `json:"profile,omitempty"`
	CSRPEM   string `json:"csr_pem"`
}

type issueResponseDTO struct {
	DeviceCertificatePEM string    `json:"device_certificate_pem"`
	CertificateChainPEM  string    `json:"certificate_chain_pem"`
	SerialNumber         string    `json:"serial_number"`
	NotBefore            time.Time `json:"not_before"`
	NotAfter             time.Time `json:"not_after"`
}

func NewIssuerClient(baseURL, token string, timeout time.Duration) sdk.IssuerClient {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &issuerClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   strings.TrimSpace(token),
		client:  &http.Client{Timeout: timeout},
	}
}

func (c *issuerClient) IssueCertificate(ctx context.Context, req sdk.IssueRequest) (*sdk.IssueResponse, error) {
	payload, err := json.Marshal(issueRequestDTO{
		DeviceID: req.DeviceID,
		TenantID: req.TenantID,
		Model:    req.Model,
		Profile:  req.Profile,
		CSRPEM:   string(req.CSRPEM),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: encode issuer request", sdk.ErrEnrollmentRequest)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+issuePath, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("%w: build issuer request", sdk.ErrEnrollmentRequest)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: call issuer endpoint", sdk.ErrEnrollmentRequest)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
	if err != nil {
		return nil, fmt.Errorf("%w: read issuer response", sdk.ErrEnrollmentRequest)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if message == "" {
			return nil, fmt.Errorf("%w: issuer returned status %d", sdk.ErrEnrollmentRequest, resp.StatusCode)
		}
		return nil, fmt.Errorf("%w: issuer returned status %d: %s", sdk.ErrEnrollmentRequest, resp.StatusCode, message)
	}

	var dto issueResponseDTO
	if err := json.Unmarshal(body, &dto); err != nil {
		return nil, fmt.Errorf("%w: decode issuer response", sdk.ErrEnrollmentRequest)
	}

	return &sdk.IssueResponse{
		DeviceCertificatePEM: []byte(dto.DeviceCertificatePEM),
		CertificateChainPEM:  []byte(dto.CertificateChainPEM),
		SerialNumber:         dto.SerialNumber,
		NotBefore:            dto.NotBefore,
		NotAfter:             dto.NotAfter,
	}, nil
}
