package secrethitler

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRoomEventsPushPersonalizedStateAndRevokeAccess(t *testing.T) {
	s, newBrowser := testServer(t, time.Now)
	host := newBrowser()
	created := host.request("POST", "create", map[string]any{"name": "host", "avatar": 1}, 201)
	code := created["code"].(string)
	outsider := newBrowser()
	outsider.request("GET", "events?code="+code, nil, 403)
	open := func(b browser) (*http.Response, *bufio.Scanner) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		t.Cleanup(cancel)
		req, _ := http.NewRequestWithContext(ctx, "GET", b.root+"/secret-hitler/api/events?code="+code, nil)
		res, err := b.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != 200 || res.Header.Get("Content-Type") != "text/event-stream" || res.Header.Get("X-Accel-Buffering") != "no" {
			t.Fatalf("invalid SSE response: %v", res)
		}
		t.Cleanup(func() { res.Body.Close() })
		return res, bufio.NewScanner(res.Body)
	}
	next := func(scanner *bufio.Scanner) map[string]any {
		t.Helper()
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				var v map[string]any
				if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &v); err != nil {
					t.Fatal(err)
				}
				return v
			}
		}
		t.Fatalf("stream ended: %v", scanner.Err())
		return nil
	}
	hostRes, hostScan := open(host)
	next(hostScan)
	guest := newBrowser()
	joined := guest.request("POST", "join", map[string]any{"code": code, "name": "guest", "avatar": 2}, 200)
	pushed := next(hostScan)
	if len(pushed["members"].([]any)) != 2 || pushed["you"] != created["you"] {
		t.Fatal("missing personalized join update", pushed)
	}
	_, guestScan := open(guest)
	personal := next(guestScan)
	if personal["you"] != joined["you"] || personal["you"] == created["you"] {
		t.Fatal("guest received wrong view")
	}
	// Disconnecting and reconnecting gets a complete current snapshot.
	hostRes.Body.Close()
	hostRes, hostScan = open(host)
	next(hostScan)
	host.request("POST", "start", map[string]any{"code": code}, 200)
	for {
		v := next(hostScan)
		encoded, _ := json.Marshal(v)
		for _, key := range []string{`"role"`, `"party"`, `"hand"`, `"token"`} {
			if strings.Contains(string(encoded), key) {
				t.Fatal("private data leaked", string(encoded))
			}
		}
		if v["game"] != nil {
			break
		}
	}
	// No public stream ever contains words, roles, hands, or tokens.
	s.mu.Lock()
	r := s.rooms[code]
	for token, owner := range r.Tokens {
		if owner == joined["you"] {
			delete(r.Tokens, token)
		}
	}
	r.notify()
	s.mu.Unlock()
	for guestScan.Scan() {
		if guestScan.Text() == "event: expired" {
			break
		}
	}
	if guestScan.Text() != "event: expired" {
		t.Fatalf("removed player was not disconnected: %v", guestScan.Err())
	}
	hostRes.Body.Close()
	deadline := time.Now().Add(time.Second)
	for {
		s.mu.Lock()
		remaining := len(s.rooms[code].listeners)
		s.mu.Unlock()
		if remaining == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("disconnected stream listeners leaked")
		}
		time.Sleep(time.Millisecond)
	}
}
