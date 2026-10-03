package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Error struct {
	StatusCode int
	Code       string
	Message    string
	RequestID  string
	Retryable  bool
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("%s (request_id=%s)", e.Message, e.RequestID)
	}
	return e.Message
}

func IsNotFound(err error) bool {
	var apiError *Error
	return errors.As(err, &apiError) && apiError.StatusCode == http.StatusNotFound
}

type Client struct {
	baseURL *url.URL
	apiKey  string
	http    *http.Client
}

func New(baseURL, apiKey string) (*Client, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("base_url must be a credential-free HTTPS URL")
	}
	if !strings.HasPrefix(apiKey, "ac_live_") || len(apiKey) > 256 {
		return nil, errors.New("api_key has an invalid format")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	return &Client{
		baseURL: parsed,
		apiKey:  apiKey,
		http: &http.Client{
			Timeout:   60 * time.Second,
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func idempotencyKey() (string, error) {
	value := make([]byte, 24)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func (c *Client) endpoint(path string) (string, error) {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.ContainsAny(path, "?#") {
		return "", errors.New("API path must be relative to the configured endpoint")
	}
	escapedPath := strings.TrimRight(c.baseURL.EscapedPath(), "/") + path
	decodedPath, err := url.PathUnescape(escapedPath)
	if err != nil {
		return "", errors.New("API path contains invalid escaping")
	}
	target := *c.baseURL
	target.Path = decodedPath
	target.RawPath = escapedPath
	return target.String(), nil
}

func (c *Client) request(ctx context.Context, method, path string, body, result any) error {
	return c.requestWithHeaders(ctx, method, path, body, result, nil)
}

func (c *Client) requestWithHeaders(
	ctx context.Context,
	method, path string,
	body, result any,
	headers map[string]string,
) error {
	target, err := c.endpoint(path)
	if err != nil {
		return err
	}
	var encoded []byte
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	key := ""
	if method != http.MethodGet {
		var keyErr error
		key, keyErr = idempotencyKey()
		if keyErr != nil {
			return keyErr
		}
	}
	for attempt := 0; attempt < 3; attempt++ {
		var payload io.Reader
		if encoded != nil {
			payload = bytes.NewReader(encoded)
		}
		request, requestErr := http.NewRequestWithContext(ctx, method, target, payload)
		if requestErr != nil {
			return requestErr
		}
		request.Header.Set("Accept", "application/json")
		request.Header.Set("User-Agent", "terraform-provider-anatoliacore/1.0")
		request.Header.Set("X-API-Key", c.apiKey)
		if body != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		if key != "" {
			request.Header.Set("Idempotency-Key", key)
		}
		for name, value := range headers {
			request.Header.Set(name, value)
		}
		response, requestErr := c.http.Do(request)
		if requestErr != nil {
			if attempt < 2 {
				if waitErr := waitForRetry(ctx, time.Duration(1<<attempt)*time.Second); waitErr != nil {
					return waitErr
				}
				continue
			}
			return fmt.Errorf("AnatoliaCore API request failed: %w", requestErr)
		}
		limited := io.LimitReader(response.Body, 2<<20)
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			var envelope struct {
				Detail any `json:"detail"`
				Error  struct {
					Code      string `json:"code"`
					Message   string `json:"message"`
					RequestID string `json:"request_id"`
					Retryable bool   `json:"retryable"`
				} `json:"error"`
			}
			_ = json.NewDecoder(limited).Decode(&envelope)
			_ = response.Body.Close()
			retryAfter := parseRetryAfter(response.Header.Get("Retry-After"))
			retryable := envelope.Error.Retryable || response.StatusCode == 429 || response.StatusCode >= 500
			message := envelope.Error.Message
			if message == "" {
				message = "AnatoliaCore API request failed"
			}
			apiError := &Error{
				StatusCode: response.StatusCode,
				Code:       envelope.Error.Code,
				Message:    message,
				RequestID:  envelope.Error.RequestID,
				Retryable:  retryable,
				RetryAfter: retryAfter,
			}
			if retryable && attempt < 2 {
				delay := retryAfter
				if delay <= 0 || delay > 30*time.Second {
					delay = time.Duration(1<<attempt) * time.Second
				}
				if waitErr := waitForRetry(ctx, delay); waitErr != nil {
					return waitErr
				}
				continue
			}
			return apiError
		}
		if result == nil || response.StatusCode == http.StatusNoContent {
			_, _ = io.Copy(io.Discard, limited)
			_ = response.Body.Close()
			return nil
		}
		if !strings.Contains(response.Header.Get("Content-Type"), "application/json") {
			_ = response.Body.Close()
			return errors.New("AnatoliaCore API returned an unexpected content type")
		}
		decodeErr := json.NewDecoder(limited).Decode(result)
		_ = response.Body.Close()
		return decodeErr
	}
	return errors.New("AnatoliaCore API retry budget exhausted")
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := time.ParseDuration(strings.TrimSpace(value) + "s")
	if err != nil || seconds < 0 {
		return 0
	}
	return seconds
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type Instance struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	CPUCores     int64   `json:"cpu_cores"`
	RAMMB        int64   `json:"ram_mb"`
	DiskGB       int64   `json:"disk_gb"`
	OSTemplate   string  `json:"os_template"`
	VDCID        *string `json:"vdc_id"`
	SSHKeyID     *string `json:"ssh_key_id"`
	IPAddress    *string `json:"ip_address"`
	Status       string  `json:"status"`
	PowerState   *string `json:"power_state"`
	HourlyPrice  float64 `json:"hourly_price"`
	MonthlyPrice float64 `json:"monthly_price"`
}

type InstanceCreate struct {
	Name       string  `json:"name"`
	CPUCores   int64   `json:"cpu_cores"`
	RAMMB      int64   `json:"ram_mb"`
	DiskGB     int64   `json:"disk_gb"`
	OSTemplate string  `json:"os_template"`
	VDCID      *string `json:"vdc_id,omitempty"`
	SSHKeyID   *string `json:"ssh_key_id,omitempty"`
	UserData   *string `json:"user_data,omitempty"`
}

func (c *Client) CreateInstance(ctx context.Context, data InstanceCreate) (*Instance, error) {
	var instance Instance
	return &instance, c.request(ctx, http.MethodPost, "/instances", data, &instance)
}

func (c *Client) GetInstance(ctx context.Context, id string) (*Instance, error) {
	var instance Instance
	return &instance, c.request(ctx, http.MethodGet, "/instances/"+url.PathEscape(id), nil, &instance)
}

func (c *Client) ResizeInstance(ctx context.Context, id string, cpu, ram, disk int64) (*Instance, error) {
	var instance Instance
	body := map[string]int64{"cpu_cores": cpu, "ram_mb": ram, "disk_gb": disk}
	return &instance, c.request(ctx, http.MethodPost, "/instances/"+url.PathEscape(id)+"/resize", body, &instance)
}

func (c *Client) DeleteInstance(ctx context.Context, id, confirmationName string) error {
	return c.requestWithHeaders(
		ctx,
		http.MethodDelete,
		"/instances/"+url.PathEscape(id),
		nil,
		nil,
		map[string]string{"X-Confirm-Resource-Name": confirmationName},
	)
}

type VDC struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	CPULimitMHz   int64    `json:"cpu_limit_mhz"`
	RAMLimitMB    int64    `json:"ram_limit_mb"`
	DiskLimitGB   int64    `json:"disk_limit_gb"`
	Status        string   `json:"status"`
	HourlyPrice   float64  `json:"hourly_price"`
	PublicIPCount int64    `json:"public_ip_count"`
	PublicIPs     []string `json:"public_ips"`
}

func (c *Client) CreateVDC(ctx context.Context, data map[string]any) (*VDC, error) {
	var vdc VDC
	return &vdc, c.request(ctx, http.MethodPost, "/vdcs", data, &vdc)
}

func (c *Client) GetVDC(ctx context.Context, id string) (*VDC, error) {
	var vdc VDC
	return &vdc, c.request(ctx, http.MethodGet, "/vdcs/"+url.PathEscape(id), nil, &vdc)
}

func (c *Client) UpdateVDC(ctx context.Context, id string, data map[string]any) (*VDC, error) {
	var vdc VDC
	return &vdc, c.request(ctx, http.MethodPatch, "/vdcs/"+url.PathEscape(id), data, &vdc)
}

func (c *Client) DeleteVDC(ctx context.Context, id, confirmationName string) error {
	return c.requestWithHeaders(
		ctx,
		http.MethodDelete,
		"/vdcs/"+url.PathEscape(id),
		nil,
		nil,
		map[string]string{"X-Confirm-Resource-Name": confirmationName},
	)
}

type InstanceType struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	CPUCores     int64    `json:"cpu_cores"`
	RAMMB        int64    `json:"ram_mb"`
	DiskGB       int64    `json:"disk_gb"`
	Description  *string  `json:"description"`
	PricePerHour *float64 `json:"price_per_hour"`
}

func (c *Client) ListInstanceTypes(ctx context.Context) ([]InstanceType, error) {
	var result []InstanceType
	if err := c.request(ctx, http.MethodGet, "/instance-types", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

type Volume struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	SizeGB     int64   `json:"size_gb"`
	InstanceID *string `json:"instance_id"`
	Status     string  `json:"status"`
}

func (c *Client) CreateVolume(ctx context.Context, name string, sizeGB int64) (*Volume, error) {
	var result Volume
	return &result, c.request(ctx, http.MethodPost, "/volumes", map[string]any{"name": name, "size_gb": sizeGB}, &result)
}
func (c *Client) GetVolume(ctx context.Context, id string) (*Volume, error) {
	var result Volume
	return &result, c.request(ctx, http.MethodGet, "/volumes/"+url.PathEscape(id), nil, &result)
}
func (c *Client) ResizeVolume(ctx context.Context, id string, sizeGB int64) (*Volume, error) {
	var result Volume
	return &result, c.request(ctx, http.MethodPost, "/volumes/"+url.PathEscape(id)+"/resize", map[string]any{"size_gb": sizeGB}, &result)
}
func (c *Client) AttachVolume(ctx context.Context, id, instanceID string) (*Volume, error) {
	var result Volume
	return &result, c.request(ctx, http.MethodPost, "/volumes/"+url.PathEscape(id)+"/attach", map[string]string{"instance_id": instanceID}, &result)
}
func (c *Client) DetachVolume(ctx context.Context, id string) (*Volume, error) {
	var result Volume
	return &result, c.request(ctx, http.MethodPost, "/volumes/"+url.PathEscape(id)+"/detach", nil, &result)
}
func (c *Client) DeleteVolume(ctx context.Context, id string) error {
	return c.request(ctx, http.MethodDelete, "/volumes/"+url.PathEscape(id), nil, nil)
}

type FloatingIP struct {
	ID                string  `json:"id"`
	VDCID             *string `json:"vdc_id"`
	IPAddress         string  `json:"ip_address"`
	DesiredInstanceID *string `json:"desired_instance_id"`
	Status            string  `json:"status"`
	DesiredRevision   int64   `json:"desired_revision"`
	AppliedRevision   int64   `json:"applied_revision"`
}

func (c *Client) CreateFloatingIP(ctx context.Context, vdcID string) (*FloatingIP, error) {
	var result FloatingIP
	return &result, c.request(ctx, http.MethodPost, "/floating-ips", map[string]string{"vdc_id": vdcID}, &result)
}
func (c *Client) GetFloatingIP(ctx context.Context, id string) (*FloatingIP, error) {
	var result FloatingIP
	return &result, c.request(ctx, http.MethodGet, "/floating-ips/"+url.PathEscape(id), nil, &result)
}
func (c *Client) AssociateFloatingIP(ctx context.Context, id, instanceID string) (*FloatingIP, error) {
	var result FloatingIP
	return &result, c.request(ctx, http.MethodPost, "/floating-ips/"+url.PathEscape(id)+"/associate", map[string]string{"instance_id": instanceID}, &result)
}
func (c *Client) DisassociateFloatingIP(ctx context.Context, id string) (*FloatingIP, error) {
	var result FloatingIP
	return &result, c.request(ctx, http.MethodPost, "/floating-ips/"+url.PathEscape(id)+"/disassociate", nil, &result)
}
func (c *Client) DeleteFloatingIP(ctx context.Context, id string) error {
	return c.request(ctx, http.MethodDelete, "/floating-ips/"+url.PathEscape(id), nil, nil)
}

type SecurityGroupRule struct {
	ID         string `json:"id,omitempty"`
	Direction  string `json:"direction"`
	Protocol   string `json:"protocol"`
	PortStart  *int64 `json:"port_start"`
	PortEnd    *int64 `json:"port_end"`
	SourceCIDR string `json:"source_cidr"`
	Action     string `json:"action"`
}
type SecurityGroup struct {
	ID              string              `json:"id"`
	VDCID           *string             `json:"vdc_id"`
	Name            string              `json:"name"`
	Description     *string             `json:"description"`
	Status          string              `json:"status"`
	DesiredRevision int64               `json:"desired_revision"`
	AppliedRevision int64               `json:"applied_revision"`
	Rules           []SecurityGroupRule `json:"rules"`
}

func (c *Client) CreateSecurityGroup(ctx context.Context, body map[string]any) (*SecurityGroup, error) {
	var result SecurityGroup
	return &result, c.request(ctx, http.MethodPost, "/security-groups", body, &result)
}
func (c *Client) GetSecurityGroup(ctx context.Context, id string) (*SecurityGroup, error) {
	var result SecurityGroup
	return &result, c.request(ctx, http.MethodGet, "/security-groups/"+url.PathEscape(id), nil, &result)
}
func (c *Client) DeleteSecurityGroup(ctx context.Context, id string) error {
	return c.request(ctx, http.MethodDelete, "/security-groups/"+url.PathEscape(id), nil, nil)
}

type BackupTarget struct {
	InstanceID string `json:"instance_id"`
}
type BackupPolicy struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Description     *string        `json:"description"`
	Status          string         `json:"status"`
	BackupTier      string         `json:"backup_tier"`
	ScheduleType    string         `json:"schedule_type"`
	ScheduleHours   int64          `json:"schedule_hours"`
	StartTime       string         `json:"start_time"`
	Timezone        string         `json:"timezone"`
	WeeklyDay       *string        `json:"weekly_day"`
	RetentionDays   int64          `json:"retention_days"`
	ScheduleEnabled bool           `json:"schedule_enabled"`
	Targets         []BackupTarget `json:"targets"`
}

func (c *Client) CreateBackupPolicy(ctx context.Context, body map[string]any) (*BackupPolicy, error) {
	var result BackupPolicy
	return &result, c.request(ctx, http.MethodPost, "/backup-policies", body, &result)
}
func (c *Client) GetBackupPolicy(ctx context.Context, id string) (*BackupPolicy, error) {
	var result BackupPolicy
	return &result, c.request(ctx, http.MethodGet, "/backup-policies/"+url.PathEscape(id), nil, &result)
}
func (c *Client) UpdateBackupPolicy(ctx context.Context, id string, body map[string]any) (*BackupPolicy, error) {
	var result BackupPolicy
	return &result, c.request(ctx, http.MethodPut, "/backup-policies/"+url.PathEscape(id), body, &result)
}
func (c *Client) DeleteBackupPolicy(ctx context.Context, id string) error {
	return c.request(ctx, http.MethodDelete, "/backup-policies/"+url.PathEscape(id), nil, nil)
}
