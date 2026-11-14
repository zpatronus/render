package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMountedApps(t *testing.T) {
	h := handler()
	get := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	for _, path := range []string{"/", "/style.css", "/healthz", "/formula-war/", "/formula-war/api/rooms", "/undercover/", "/undercover/style.css", "/undercover/app.js", "/undercover/api/info", "/secret-hitler/", "/secret-hitler/style.css", "/secret-hitler/app.js", "/secret-hitler/LICENSE"} {
		if w := get(path); w.Code != 200 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	if w := get("/formula-war"); w.Code != 301 || w.Header().Get("Location") != "/formula-war/" {
		t.Fatal("missing canonical redirect")
	}
	if w := get("/undercover"); w.Code != 301 || w.Header().Get("Location") != "/undercover/" {
		t.Fatal("missing undercover redirect")
	}
	undercoverPage := get("/undercover/").Body.String()
	for _, link := range []string{`href="/">Sidequests home`, `href="/undercover/">谁是卧底首页`} {
		if !strings.Contains(undercoverPage, link) {
			t.Fatal("missing undercover navigation", link)
		}
	}
	if w := get("/secret-hitler"); w.Code != 301 || w.Header().Get("Location") != "/secret-hitler/" {
		t.Fatal("missing Secret Hitler redirect")
	}
	shPage := get("/secret-hitler/").Body.String()
	for _, label := range []string{`href="/">Sidequests home`, `href="/secret-hitler/">揭秘希特勒首页`, `id="rules-dialog"`} {
		if !strings.Contains(shPage, label) {
			t.Fatal("missing Chinese game navigation", label)
		}
	}
	page := get("/formula-war/").Body.String()
	for _, link := range []string{`class="site-home" href="/">Sidequests home`, `id="home-link" class="game-home" href="/formula-war/">Formula War home`} {
		if !strings.Contains(page, link) {
			t.Fatalf("missing distinct home navigation: %s", link)
		}
	}
	for _, marker := range []string{`href="/formula-war/assets/`, `src="/formula-war/assets/`, `href="/formula-war/"`} {
		if !strings.Contains(page, marker) {
			t.Fatalf("missing mounted URL %s", marker)
		}
	}
	_, after, _ := strings.Cut(page, `src="`)
	asset, _, _ := strings.Cut(after, `"`)
	script := get(asset)
	if script.Code != 200 || !strings.Contains(script.Body.String(), "/formula-war/api/rooms") {
		t.Fatal("mounted frontend cannot reach API")
	}
	prefix := strings.TrimSuffix(asset, "app.mjs")
	for _, name := range []string{"game.mjs", "ui.mjs", "style.css"} {
		if get(prefix+name).Code != 200 {
			t.Fatal("missing asset", name)
		}
	}
	for _, path := range []string{"/api/rooms", "/unknown", "/main.go", "/formula-war/main.go"} {
		if get(path).Code != 404 {
			t.Fatal("unexpected route", path)
		}
	}
	req := httptest.NewRequest("POST", "/formula-war/api/rooms", strings.NewReader(`{"name":"Test","player":"Ada"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("room creation: %d %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest("POST", "/formula-war/api/rooms", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://other.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal("mounted API lost origin protection")
	}
}
