package undercover

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

func (b browser) request(method, path string, body any, status int) map[string]any {
	b.t.Helper()
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, b.root+"/undercover/api/"+path, bytes.NewReader(data))
	if method == "POST" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := b.client.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		b.t.Fatal(err)
	}
	if res.StatusCode != status {
		b.t.Fatalf("%s %s: got %d, want %d: %v", method, path, res.StatusCode, status, out)
	}
	return out
}
func TestPrivateDealingAndRoomLifecycle(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/undercover/", http.StripPrefix("/undercover", Handler()))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	newBrowser := func() browser { jar, _ := cookiejar.New(nil); return browser{t, &http.Client{Jar: jar}, srv.URL} }
	host := newBrowser()
	created := host.request("POST", "create", map[string]any{"name": "房主"}, 201)
	if created["spies"] != float64(1) || created["blanks"] != float64(0) {
		t.Fatal("unexpected initial room settings", created)
	}
	code := created["code"].(string)
	players := []browser{host}
	for i := 1; i < 7; i++ {
		b := newBrowser()
		b.request("POST", "join", map[string]any{"code": code, "name": "玩家" + strconv.Itoa(i)}, 200)
		players = append(players, b)
	}
	outsider := newBrowser()
	outsider.request("GET", "room?code="+code, nil, 403)
	// A non-host cannot deal, reset, or access another member's word.
	players[1].request("POST", "deal", map[string]any{"code": code, "spies": 2, "blanks": 2}, 403)
	host.request("POST", "deal", map[string]any{"code": code, "spies": 7, "blanks": 0}, 400)
	host.request("POST", "deal", map[string]any{"code": code, "spies": 2, "blanks": 2}, 200)
	host.request("POST", "deal", map[string]any{"code": code, "spies": 2, "blanks": 2}, 409)
	outsider.request("POST", "join", map[string]any{"code": code, "name": "迟到"}, 409)
	counts := map[string]int{}
	for _, p := range players {
		public := p.request("GET", "room?code="+code, nil, 200)
		encoded, _ := json.Marshal(public)
		for _, key := range []string{`"word"`, `"id"`, `"token"`} {
			if strings.Contains(string(encoded), key) {
				t.Fatalf("private data leaked: %s", encoded)
			}
		}
		own := p.request("POST", "word", map[string]any{"code": code, "round": 1, "player": "someone-else"}, 200)
		if _, ok := own["role"]; ok {
			t.Fatal("ordinary players must not be told their faction")
		}
		counts[own["word"].(string)]++
	}
	if counts[""] != 2 || len(counts) != 3 {
		t.Fatalf("wrong role counts: %v", counts)
	}
	delete(counts, "")
	distribution := map[int]int{}
	for _, count := range counts {
		distribution[count]++
	}
	if distribution[2] != 1 || distribution[3] != 1 {
		t.Fatal("expected 2 undercover and 3 civilian cards", counts)
	}
	players[1].request("POST", "reset", map[string]any{"code": code, "round": 1}, 403)
	host.request("POST", "reset", map[string]any{"code": code, "round": 1}, 200)
	host.request("POST", "word", map[string]any{"code": code, "round": 1}, 409)
	host.request("POST", "deal", map[string]any{"code": code, "round": 1, "spies": 1, "blanks": 0}, 200)
	host.request("POST", "word", map[string]any{"code": code, "round": 1}, 409)
	host.request("POST", "word", map[string]any{"code": code, "round": 2}, 200)
	host.request("POST", "leave", map[string]any{"code": code}, 200)
	state := players[1].request("GET", "room?code="+code, nil, 200)
	if state["host"] != true {
		t.Fatal("host was not transferred")
	}
	host.request("GET", "room?code="+code, nil, 403)
}
func TestOriginAndPrivateAssets(t *testing.T) {
	h := Handler()
	for _, path := range []string{"/words.tsv", "/app.go", "/app_test.go"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatal("exposed", path)
		}
	}
	req := httptest.NewRequest("POST", "http://example.com/api/create", strings.NewReader(`{"name":"x","spies":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://other.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal("cross-origin create allowed")
	}
}
func TestImportedBankIntegrity(t *testing.T) {
	pairs := loadPairs()
	if len(pairs) < 1000 {
		t.Fatal("word import unexpectedly shrank", len(pairs))
	}
	data, err := assets.ReadFile("words.tsv")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, p := range pairs {
		a, b := p.a, p.b
		if a > b {
			a, b = b, a
		}
		key := a + "\t" + b
		if a == b || seen[key] {
			t.Fatal("duplicate or same-word pair", p)
		}
		seen[key] = true
	}
	if len(strings.Split(strings.TrimSpace(string(data)), "\n")) != len(pairs) {
		t.Fatal("invalid or duplicate rows in bank")
	}
	for _, bad := range []string{"金瓶梅", "大便", "人妖", "太监", "婊", "妇炎洁", "大麻", "黑丝", "渔网袜", "丝袜", "丁字裤", "内裤", "做爱", "叫床"} {
		if strings.Contains(string(data), bad) {
			t.Fatal("unfiltered content", bad)
		}
	}
}

func TestOrdinaryBodyPairsRemainAvailable(t *testing.T) {
	for _, p := range loadPairs() {
		if (p.a == "狐臭" && p.b == "香港脚") || (p.b == "狐臭" && p.a == "香港脚") {
			return
		}
	}
	t.Fatal("ordinary body topics should not be removed as adult content")
}

func TestFifteenMinuteExpiryAndEveryRoomRequestCounts(t *testing.T) {
	var clock atomic.Int64
	clock.Store(time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC).UnixNano())
	s := &server{rooms: map[string]*room{}, pairs: loadPairs(), now: func() time.Time { return time.Unix(0, clock.Load()) }}
	mux := http.NewServeMux()
	mux.Handle("/undercover/", http.StripPrefix("/undercover", s.handler()))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	jar, _ := cookiejar.New(nil)
	host := browser{t, &http.Client{Jar: jar}, srv.URL}
	outsider := browser{t, &http.Client{}, srv.URL}
	created := host.request("POST", "create", map[string]any{"name": "房主"}, 201)
	code := created["code"].(string)
	advance := func(d time.Duration) { clock.Add(int64(d)) }
	advance(14 * time.Minute)
	outsider.request("GET", "room?code="+code, nil, 403) // Even rejected room requests count.
	advance(14 * time.Minute)
	host.request("GET", "room?code="+code, nil, 200) // Polling counts too.
	advance(15 * time.Minute)
	host.request("GET", "room?code="+code, nil, 404) // At exactly 15 minutes the room has expired.
	s.mu.Lock()
	_, present := s.rooms[code]
	s.mu.Unlock()
	if present {
		t.Fatal("expired room was not deleted")
	}
	created = host.request("POST", "create", map[string]any{"name": "房主"}, 201)
	code = created["code"].(string)
	advance(14 * time.Minute)
	host.request("GET", "info", nil, 200) // Global requests do not renew a room.
	advance(time.Minute)
	host.request("GET", "room?code="+code, nil, 404)
}
