package macrasms

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func server(t *testing.T, status int, reply map[string]interface{}, check func(r *http.Request, body []byte)) *Client {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if check != nil {
			check(r, b)
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(reply)
	}))
	t.Cleanup(ts.Close)
	c := NewClient("TEST", "k1")
	c.BaseURL = ts.URL + "/api/v1/"
	return c
}

func TestSend(t *testing.T) {
	c := server(t, 200, map[string]interface{}{"success": true, "message_id": 7, "status": "pending", "sender_id": "TEST",
		"total_recipients": 2, "segments_per_message": 1, "credits_deducted": 2, "balance_remaining": 98},
		func(r *http.Request, b []byte) {
			if r.Method != "POST" || r.URL.Path != "/api/v1/" {
				t.Errorf("got %s %s", r.Method, r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer k1" || r.Header.Get("X-Sender-Id") != "TEST" {
				t.Errorf("bad auth headers")
			}
			if strings.Contains(r.URL.String(), "k1") {
				t.Errorf("api key leaked into the url")
			}
			var m map[string]string
			_ = json.Unmarshal(b, &m)
			if m["recipients"] != "0712345678,0722000111" || m["message_text"] != "Hi" {
				t.Errorf("bad body %s", b)
			}
		})
	res, err := c.Send(context.Background(), []string{"0712345678", " 0722000111 "}, "Hi")
	if err != nil || res.MessageID != 7 || res.BalanceRemaining != 98 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestBalanceAndStatus(t *testing.T) {
	c := server(t, 200, map[string]interface{}{"success": true, "balance": 41}, func(r *http.Request, _ []byte) {
		if r.URL.Path != "/api/v1/balance/" {
			t.Errorf("path %s", r.URL.Path)
		}
	})
	if b, err := c.Balance(context.Background()); err != nil || b != 41 {
		t.Fatalf("balance %d %v", b, err)
	}
	c = server(t, 200, map[string]interface{}{"success": true, "message_id": 5, "status": "sent", "total_recipients": 1, "successful_count": 1},
		func(r *http.Request, _ []byte) {
			if r.URL.Path != "/api/v1/messages/" || r.URL.Query().Get("id") != "5" {
				t.Errorf("url %s", r.URL)
			}
		})
	if st, err := c.MessageStatus(context.Background(), 5); err != nil || st.Status != "sent" {
		t.Fatalf("status %+v %v", st, err)
	}
}

func TestErrors(t *testing.T) {
	for code, kind := range map[int]error{400: ErrValidation, 401: ErrAuthentication, 402: ErrInsufficientBalance,
		403: ErrAccountSuspended, 404: ErrNotFound, 429: ErrRateLimited, 500: ErrAPI} {
		c := server(t, code, map[string]interface{}{"success": false, "error": "e", "required": 4, "available": 1, "retry_after": 6}, nil)
		_, err := c.Send(context.Background(), []string{"0712345678"}, "x")
		var ae *APIError
		if !errors.Is(err, kind) || !errors.As(err, &ae) || ae.Status != code {
			t.Errorf("HTTP %d: got %v", code, err)
		}
		if code == 402 {
			if v, ok := ae.Int("required"); !ok || v != 4 {
				t.Errorf("required missing")
			}
		}
	}
}

func TestSegmentsMatchServer(t *testing.T) {
	raw, err := os.ReadFile("testdata/segment_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vs []struct {
		Text     string `json:"text"`
		Segments int    `json:"segments"`
	}
	_ = json.Unmarshal(raw, &vs)
	for _, v := range vs {
		if got := Segments(v.Text); got != v.Segments {
			t.Errorf("%q: got %d want %d", v.Text[:min(len(v.Text), 20)], got, v.Segments)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
