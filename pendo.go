package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Pendo Track Event names. These must match the track types registered in
// Pendo exactly.
const (
	eventLogFollowStarted    = "log_follow_started"
	eventPatternCompileFail  = "pattern_compile_failed"
	eventLogFileOpenFailed   = "log_file_open_failed"
	defaultPendoDataHost     = "data.pendo-dev.pendo-dev.com"
	defaultPendoID           = "system"
	pendoTrackRequestTimeout = 3 * time.Second
)

// pendoTrackPayload is the body of a request to Pendo's server-side Track API.
type pendoTrackPayload struct {
	Type       string         `json:"type"`
	Event      string         `json:"event"`
	VisitorID  string         `json:"visitorId"`
	AccountID  string         `json:"accountId"`
	Timestamp  int64          `json:"timestamp"`
	Properties map[string]any `json:"properties,omitempty"`
}

// envOrDefault returns the value of the environment variable key, or def when
// it is unset or empty.
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// truncate shortens s to at most n bytes so event properties stay within
// Pendo's size limits.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// pendoTrack sends a Track Event to Pendo. It is a no-op when
// PENDO_TRACK_EVENT_SECRET is not set, and it never returns an error: a
// tracking failure is reported on stderr and must not affect the CLI.
//
// logtail has no signed-in user, so the visitor and account default to
// "system" unless PENDO_VISITOR_ID / PENDO_ACCOUNT_ID are provided.
func pendoTrack(event string, properties map[string]any) {
	secret := os.Getenv("PENDO_TRACK_EVENT_SECRET")
	if secret == "" {
		return
	}

	body, err := json.Marshal(pendoTrackPayload{
		Type:       "track",
		Event:      event,
		VisitorID:  envOrDefault("PENDO_VISITOR_ID", defaultPendoID),
		AccountID:  envOrDefault("PENDO_ACCOUNT_ID", defaultPendoID),
		Timestamp:  time.Now().UnixMilli(),
		Properties: properties,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "pendo track:", err)
		return
	}

	url := "https://" + envOrDefault("PENDO_DATA_HOST", defaultPendoDataHost) + "/data/track"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		fmt.Fprintln(os.Stderr, "pendo track:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-pendo-integration-key", secret)

	client := &http.Client{Timeout: pendoTrackRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pendo track:", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		fmt.Fprintln(os.Stderr, "pendo track: unexpected status", resp.Status)
	}
}
