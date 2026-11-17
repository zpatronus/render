package identity

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIdleRoomCredentialsAreRemoved(t *testing.T) {
	s := New()
	s.idle = 20 * time.Millisecond
	backend := http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "test_ABC", Value: "private"})
		w.WriteHeader(201)
	})
	h := s.Wrap("test", "test_", backend)
	q := httptest.NewRequest("POST", "/api/create", strings.NewReader(`{"name":"Ada","pin":"0123"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, q)
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		empty := len(s.rooms) == 0
		s.mu.Unlock()
		if empty {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("idle room credentials were retained")
}
