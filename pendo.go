package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Pendo server-side Track Events.
//
// logtail is a CLI with no browser agent, so events are sent to Pendo's
// /data/track endpoint over HTTP. Tracking is disabled unless the
// integration key is provided via the PENDO_TRACK_EVENT_SECRET environment
// variable; the data host can be overridden with PENDO_DATA_HOST.
const (
	pendoDefaultDataHost = "data.pendo-dev.pendo-dev.com"
	pendoTrackTimeout    = 2 * time.Second

	eventLogFollowStarted    = "log_follow_started"
	eventLogFileOpenFailed   = "log_file_open_failed"
	eventMatchPatternInvalid = "match_pattern_invalid"
)

type pendoTrackPayload struct {
	Type       string         `json:"type"`
	Event      string         `json:"event"`
	VisitorID  string         `json:"visitorId"`
	AccountID  string         `json:"accountId"`
	Timestamp  int64          `json:"timestamp"`
	Properties map[string]any `json:"properties,omitempty"`
}

// pendoTrack sends a Track Event synchronously with a short timeout. It never
// returns an error: tracking failures are reported on stderr and must not
// change the behaviour of the CLI.
func pendoTrack(event string, properties map[string]any) {
	secret := strings.TrimSpace(os.Getenv("PENDO_TRACK_EVENT_SECRET"))
	if secret == "" {
		return
	}
	host := strings.TrimSpace(os.Getenv("PENDO_DATA_HOST"))
	if host == "" {
		host = pendoDefaultDataHost
	}

	body, err := json.Marshal(pendoTrackPayload{
		Type:  "track",
		Event: event,
		// A CLI has no signed-in user; a stable per-install ID identifies
		// the visitor, and there is no account concept.
		VisitorID:  pendoInstallID(),
		AccountID:  "system",
		Timestamp:  time.Now().UnixMilli(),
		Properties: properties,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "pendo track:", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), pendoTrackTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+host+"/data/track", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintln(os.Stderr, "pendo track:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-pendo-integration-key", secret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pendo track:", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		fmt.Fprintln(os.Stderr, "pendo track: unexpected status", resp.Status)
	}
}

// pendoInstallID returns a random ID persisted in the user's config
// directory so events from the same installation share a visitor. It falls
// back to "system" if the ID cannot be read or stored.
func pendoInstallID() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "system"
	}
	idPath := filepath.Join(dir, "logtail", "install_id")
	if data, err := os.ReadFile(idPath); err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			return id
		}
	}

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "system"
	}
	id := hex.EncodeToString(buf)
	if err := os.MkdirAll(filepath.Dir(idPath), 0o700); err != nil {
		return "system"
	}
	if err := os.WriteFile(idPath, []byte(id+"\n"), 0o600); err != nil {
		return "system"
	}
	return id
}
