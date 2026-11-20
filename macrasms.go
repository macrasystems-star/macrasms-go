// Package macrasms is the official Go SDK for the MacraSMS bulk SMS API (https://macrasms.com).
//
//	client := macrasms.NewClient("MYSENDER", "your-api-key")
//	res, err := client.Send(ctx, []string{"0712345678"}, "Hello from MacraSMS")
package macrasms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Version of this SDK.
const Version = "1.0.0"

// DefaultBaseURL is the MacraSMS API address.
const DefaultBaseURL = "https://sms.macrasms.com/api/v1/"

// Client talks to the MacraSMS API with an approved sender ID and its API key.
type Client struct {
	SenderID   string
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient returns a client with the default address and a 30 second timeout.
func NewClient(senderID, apiKey string) *Client {
	return &Client{SenderID: senderID, APIKey: apiKey, BaseURL: DefaultBaseURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second}}
}

// SendResult is the reply to a successful Send.
type SendResult struct {
	MessageID          int64  `json:"message_id"`
	Status             string `json:"status"`
	SenderID           string `json:"sender_id"`
	TotalRecipients    int    `json:"total_recipients"`
	SegmentsPerMessage int    `json:"segments_per_message"`
	CreditsDeducted    int    `json:"credits_deducted"`
	BalanceRemaining   int    `json:"balance_remaining"`
}

// MessageStatus is the state of a sent message: pending, sent, partial or failed.
type MessageStatus struct {
	MessageID       int64   `json:"message_id"`
	SenderID        string  `json:"sender_id"`
	Status          string  `json:"status"`
	TotalRecipients int     `json:"total_recipients"`
	SuccessfulCount int     `json:"successful_count"`
	FailedCount     int     `json:"failed_count"`
	CreatedAt       *string `json:"created_at"`
	SentAt          *string `json:"sent_at"`
}

// Error kinds, for use with errors.Is.
var (
	ErrValidation          = errors.New("macrasms: validation failed")        // 400
	ErrAuthentication      = errors.New("macrasms: invalid sender ID or key") // 401
	ErrInsufficientBalance = errors.New("macrasms: insufficient balance")     // 402
	ErrAccountSuspended    = errors.New("macrasms: account suspended")        // 403
	ErrNotFound            = errors.New("macrasms: not found")                // 404
	ErrRateLimited         = errors.New("macrasms: rate limited")             // 429
	ErrAPI                 = errors.New("macrasms: api error")                // anything else
)

// APIError carries the HTTP status and the API's JSON reply.
type APIError struct {
	Status  int
	Message string
	Body    map[string]interface{}
	kind    error
}

func (e *APIError) Error() string { return fmt.Sprintf("macrasms: %s (HTTP %d)", e.Message, e.Status) }

// Unwrap lets errors.Is match the error kind, e.g. errors.Is(err, macrasms.ErrInsufficientBalance).
func (e *APIError) Unwrap() error { return e.kind }

// Int returns a numeric field of the reply, such as "required", "available" or "retry_after".
func (e *APIError) Int(field string) (int, bool) {
	if v, ok := e.Body[field].(float64); ok {
		return int(v), true
	}
	return 0, false
}

var kinds = map[int]error{400: ErrValidation, 401: ErrAuthentication, 402: ErrInsufficientBalance,
	403: ErrAccountSuspended, 404: ErrNotFound, 429: ErrRateLimited}

// Send queues message to one or more Kenyan phone numbers.
func (c *Client) Send(ctx context.Context, to []string, message string) (*SendResult, error) {
	clean := make([]string, 0, len(to))
	for _, n := range to {
		if n = strings.TrimSpace(n); n != "" {
			clean = append(clean, n)
		}
	}
	if len(clean) == 0 {
		return nil, fmt.Errorf("%w: at least one recipient is required", ErrValidation)
	}
	if message == "" {
		return nil, fmt.Errorf("%w: message is required", ErrValidation)
	}
	var res SendResult
	err := c.do(ctx, http.MethodPost, "", nil, map[string]string{"recipients": strings.Join(clean, ","), "message_text": message}, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// Balance returns the credits left on the account that owns the sender ID.
func (c *Client) Balance(ctx context.Context) (int, error) {
	var res struct {
		Balance int `json:"balance"`
	}
	if err := c.do(ctx, http.MethodGet, "balance/", nil, nil, &res); err != nil {
		return 0, err
	}
	return res.Balance, nil
}

// MessageStatus returns the status of a message this account sent.
func (c *Client) MessageStatus(ctx context.Context, messageID int64) (*MessageStatus, error) {
	var res MessageStatus
	q := url.Values{"id": {strconv.FormatInt(messageID, 10)}}
	if err := c.do(ctx, http.MethodGet, "messages/", q, nil, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) do(ctx context.Context, method, path string, q url.Values, body interface{}, out interface{}) error {
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	u := strings.TrimRight(base, "/") + "/" + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("X-Sender-Id", c.SenderID)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "macrasms-go/"+Version)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	hc := c.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return &APIError{Message: "could not reach MacraSMS: " + err.Error(), kind: ErrAPI}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return &APIError{Status: resp.StatusCode, Message: err.Error(), kind: ErrAPI}
	}
	var generic map[string]interface{}
	if err := json.Unmarshal(raw, &generic); err != nil {
		return &APIError{Status: resp.StatusCode, Message: "non JSON reply", kind: ErrAPI}
	}
	if ok, _ := generic["success"].(bool); ok && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return json.Unmarshal(raw, out)
	}
	msg, _ := generic["error"].(string)
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
	}
	kind, found := kinds[resp.StatusCode]
	if !found {
		kind = ErrAPI
	}
	return &APIError{Status: resp.StatusCode, Message: msg, Body: generic, kind: kind}
}
