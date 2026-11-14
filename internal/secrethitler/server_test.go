package secrethitler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type browser struct {
	t      *testing.T
	client *http.Client
	root   string
}

func (b browser) request(method, endpoint string, data any, want int) map[string]any {
	b.t.Helper()
	body, _ := json.Marshal(data)
	req, _ := http.NewRequest(method, b.root+"/secret-hitler/api/"+endpoint, bytes.NewReader(body))
	if method == "POST" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := b.client.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	defer res.Body.Close()
	out := map[string]any{}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		b.t.Fatal(err)
	}
	if res.StatusCode != want {
		b.t.Fatalf("%s %s: %d want %d: %v", method, endpoint, res.StatusCode, want, out)
	}
	return out
}
func testServer(t *testing.T, now func() time.Time) (*server, func() browser) {
	t.Helper()
	s := &server{rooms: map[string]*room{}, now: now}
	mux := http.NewServeMux()
	mux.Handle("/secret-hitler/", http.StripPrefix("/secret-hitler", s.handler()))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return s, func() browser { jar, _ := cookiejar.New(nil); return browser{t, &http.Client{Jar: jar}, srv.URL} }
}
func TestRoomBotsPrivateRolesResultsAndRematch(t *testing.T) {
	s, newBrowser := testServer(t, time.Now)
	host := newBrowser()
	created := host.request("POST", "create", map[string]any{"name": "房主", "avatar": 20}, 201)
	code := created["code"].(string)
	if created["bots"] != float64(4) {
		t.Fatal("solo should fill to five players")
	}
	guest := newBrowser()
	joined := guest.request("POST", "join", map[string]any{"code": code, "name": "朋友", "avatar": 19}, 200)
	if joined["bots"] != float64(3) {
		t.Fatal("auto-fill did not adjust to humans")
	}
	guest.request("POST", "start", map[string]any{"code": code}, 403)
	host.request("POST", "bots", map[string]any{"code": code, "count": 5}, 200)
	state := host.request("POST", "start", map[string]any{"code": code}, 200)
	g := state["game"].(map[string]any)
	oldID := g["id"].(string)
	if len(state["members"].([]any)) != 7 {
		t.Fatal("configured bots not added")
	}
	for _, member := range state["members"].([]any) {
		m := member.(map[string]any)
		if _, exists := m["ready"]; exists {
			t.Fatal("readiness should not exist")
		}
	}
	outsider := newBrowser()
	outsider.request("GET", "private?code="+code, nil, 403)
	original := map[string]role{}
	s.mu.Lock()
	for _, p := range s.rooms[code].Game.Players {
		original[p.ID] = p.Role
	}
	s.mu.Unlock()
	for _, client := range []browser{host, guest} {
		own := client.request("GET", "private?code="+code, nil, 200)
		if own["role"] != string(original[own["id"].(string)]) {
			t.Fatal("private role not bound to player ID")
		}
		public := client.request("GET", "room?code="+code, nil, 200)
		encoded, _ := json.Marshal(public)
		for _, key := range []string{`"role"`, `"party"`, `"hand"`, `"token"`, `"results"`} {
			if strings.Contains(string(encoded), key) {
				t.Fatal("private data leaked", string(encoded))
			}
		}
	}
	// Finish the authoritative game, then change room ownership without changing role bindings.
	s.mu.Lock()
	s.rooms[code].Game.end(liberal, "测试结算")
	s.mu.Unlock()
	host.request("POST", "leave", map[string]any{"code": code}, 200)
	final := guest.request("GET", "room?code="+code, nil, 200)
	for _, item := range final["game"].(map[string]any)["results"].([]any) {
		p := item.(map[string]any)
		id := p["id"].(string)
		if p["role"] != string(original[id]) {
			t.Fatal("ownership change corrupted final role", p)
		}
	}
	if final["host"] != joined["you"] {
		t.Fatal("host did not transfer")
	}
	guest.request("POST", "reset", map[string]any{"code": code, "game": oldID}, 200)
	next := guest.request("POST", "start", map[string]any{"code": code}, 200)
	newID := next["game"].(map[string]any)["id"]
	if newID == oldID {
		t.Fatal("new game reused identity")
	}
	guest.request("POST", "vote", map[string]any{"code": code, "game": oldID, "step": 1, "yes": true}, 409)
}
func TestAnyRoomRequestRenewsFifteenMinuteExpiry(t *testing.T) {
	var clock atomic.Int64
	clock.Store(time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC).UnixNano())
	s, newBrowser := testServer(t, func() time.Time { return time.Unix(0, clock.Load()) })
	host := newBrowser()
	created := host.request("POST", "create", map[string]any{"name": "房主", "avatar": 1}, 201)
	code := created["code"].(string)
	clock.Add(int64(14 * time.Minute))
	newBrowser().request("GET", "room?code="+code, nil, 403)
	clock.Add(int64(14 * time.Minute))
	host.request("GET", "room?code="+code, nil, 200)
	clock.Add(int64(15 * time.Minute))
	host.request("GET", "room?code="+code, nil, 404)
	s.mu.Lock()
	_, exists := s.rooms[code]
	s.mu.Unlock()
	if exists {
		t.Fatal("room not deleted after inactivity")
	}
}
func TestAssetsAndBoundaryProtection(t *testing.T) {
	h := Handler()
	for i := 1; i <= 20; i++ {
		path := "/art/avatar-" + strconv.Itoa(i) + ".svg"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatal("locked or missing avatar", i)
		}
	}
	for _, path := range []string{"/game.go", "/game_test.go", "/server.go", "/app.test.mjs", "/../LICENSE"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatal("unexpected file exposure", path, w.Code)
		}
	}
	for _, path := range []string{"/", "/app.js", "/style.css", "/LICENSE", "/art/board-liberal.png"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatal("asset missing", path)
		}
		if etag := w.Header().Get("ETag"); etag != "" {
			q := httptest.NewRequest("GET", path, nil)
			q.Header.Set("If-None-Match", etag)
			cached := httptest.NewRecorder()
			h.ServeHTTP(cached, q)
			if cached.Code != 304 {
				t.Fatal("asset revalidation failure", path)
			}
		}
	}
	q := httptest.NewRequest("POST", "http://example.com/api/create", strings.NewReader(`{"name":"x","avatar":1}`))
	q.Header.Set("Content-Type", "application/json")
	q.Header.Set("Origin", "https://elsewhere.test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, q)
	if w.Code != 403 {
		t.Fatal("cross-site room creation allowed")
	}
}
