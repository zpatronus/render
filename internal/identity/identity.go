// Package identity restores room seats using a username and a four-digit PIN.
package identity

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"time"
)

type seat struct {
	hash       [32]byte
	credential string
	failures   int
	locked     time.Time
}
type room struct {
	key   string
	seats map[string]*seat
	timer *time.Timer
}
type Store struct {
	mu    sync.Mutex
	rooms map[string]*room
	idle  time.Duration
}

func New() *Store { return &Store{rooms: map[string]*room{}, idle: 15 * time.Minute} }
func validPIN(pin string) bool {
	if len(pin) != 4 {
		return false
	}
	for _, c := range pin {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
func reply(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
func digest(key, name, pin string) [32]byte {
	return sha256.Sum256([]byte(key + ":" + name + ":" + pin))
}
func (s *Store) touch(key string, r *room) {
	if r.timer != nil {
		r.timer.Stop()
	}
	var timer *time.Timer
	timer = time.AfterFunc(s.idle, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.rooms[key] == r && r.timer == timer {
			for alias, current := range s.rooms {
				if current == r {
					delete(s.rooms, alias)
				}
			}
		}
	})
	r.timer = timer
}

// Existing game tokens are kept internally; browsers authenticate each request with the pair.
func (s *Store) Wrap(base, prefix string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		path := q.URL.Path
		if !strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, q)
			return
		}
		if q.Method == "GET" && (path == "/api/info" || path == "/api/emojis" || path == "/api/rooms") {
			next.ServeHTTP(w, q)
			return
		}
		if q.Header.Get("Sec-Fetch-Site") == "cross-site" {
			reply(w, 403, "请在本站操作")
			return
		}
		if origin := q.Header.Get("Origin"); origin != "" {
			u, e := url.Parse(origin)
			if e != nil || !strings.EqualFold(u.Host, q.Host) {
				reply(w, 403, "请在本站操作")
				return
			}
		}
		var data map[string]any
		if q.Method == "POST" {
			raw, e := io.ReadAll(io.LimitReader(q.Body, 32769))
			if e != nil || len(raw) > 32768 || json.Unmarshal(raw, &data) != nil || data == nil {
				reply(w, 400, "输入有误")
				return
			}
		}
		value := func(key string) string {
			if data != nil {
				v, _ := data[key].(string)
				return v
			}
			return q.URL.Query().Get(key)
		}
		formula := prefix == ""
		name := strings.TrimSpace(value("username"))
		if name == "" {
			name = strings.TrimSpace(value("name"))
		}
		pin := value("pin")
		code := value("code")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		entry := path == "/api/create" || path == "/api/join"
		if formula {
			entry = path == "/api/rooms" && q.Method == "POST" || strings.HasSuffix(path, "/join")
			if len(parts) >= 3 {
				code = parts[2]
			}
			if path == "/api/rooms" && q.Method == "POST" {
				name = strings.TrimSpace(value("player"))
				code = strings.ToLower(strings.TrimSpace(value("name")))
			}
		} else {
			code = strings.ToUpper(strings.TrimSpace(code))
		}
		if name == "" || len([]rune(name)) > 24 || !validPIN(pin) {
			reply(w, 403, "请填写用户名和 4 位 PIN")
			return
		}
		key := base + ":" + code
		lookup := strings.ToLower(name)
		credential := ""
		s.mu.Lock()
		r := s.rooms[key]
		var owner *seat
		if r != nil {
			owner = r.seats[lookup]
		}
		if owner != nil {
			if time.Now().Before(owner.locked) {
				s.mu.Unlock()
				reply(w, 429, "PIN 尝试过多，请稍后再试")
				return
			}
			hash := digest(r.key, lookup, pin)
			if subtle.ConstantTimeCompare(hash[:], owner.hash[:]) != 1 {
				owner.failures++
				if owner.failures >= 5 {
					owner.failures = 0
					owner.locked = time.Now().Add(time.Minute)
				}
				s.mu.Unlock()
				reply(w, 403, "这个用户名的 PIN 不正确")
				return
			}
			owner.failures = 0
			credential = owner.credential
			s.touch(key, r)
		}
		if path == "/api/pin" {
			if owner == nil {
				s.mu.Unlock()
				reply(w, 403, "请先加入房间")
				return
			}
			newPin := value("newPin")
			if !validPIN(newPin) {
				s.mu.Unlock()
				reply(w, 400, "新 PIN 必须是 4 位数字")
				return
			}
			owner.hash = digest(r.key, lookup, newPin)
			s.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"changed":true}`))
			return
		}
		s.mu.Unlock()
		if !entry && owner == nil {
			reply(w, 403, "请用用户名和 PIN 加入房间")
			return
		}
		if formula {
			if data != nil {
				data["token"] = credential
			}
			values := q.URL.Query()
			values.Set("token", credential)
			q.URL.RawQuery = values.Encode()
		} else {
			cookies := q.Cookies()
			q.Header.Del("Cookie")
			for _, c := range cookies {
				if c.Name != prefix+code {
					q.AddCookie(c)
				}
			}
			if credential != "" {
				q.AddCookie(&http.Cookie{Name: prefix + code, Value: credential})
			}
		}
		if data != nil {
			raw, _ := json.Marshal(data)
			q.Body = io.NopCloser(bytes.NewReader(raw))
			q.ContentLength = int64(len(raw))
		}
		if !entry {
			next.ServeHTTP(w, q)
			return
		}
		// Serialize registration so two simultaneous joins cannot claim the same username.
		s.mu.Lock()
		defer s.mu.Unlock()
		if current := s.rooms[key]; current != nil {
			if existing := current.seats[lookup]; existing != nil && existing != owner {
				reply(w, 409, "房间状态已更新，请重试")
				return
			}
		}
		recorder := httptest.NewRecorder()
		next.ServeHTTP(recorder, q)
		if recorder.Code < 300 {
			oldKey := key
			if formula {
				var result struct {
					Token string
					Room  struct{ ID, Name string }
				}
				json.Unmarshal(recorder.Body.Bytes(), &result)
				credential = result.Token
				code = result.Room.ID
				key = base + ":" + code
			} else {
				for _, c := range recorder.Result().Cookies() {
					if strings.HasPrefix(c.Name, prefix) {
						credential = c.Value
						code = strings.TrimPrefix(c.Name, prefix)
						key = base + ":" + code
					}
				}
			}
			if credential != "" {
				r = s.rooms[key]
				if r == nil {
					r = &room{key: key, seats: map[string]*seat{}}
					s.rooms[key] = r
				}
				r.seats[lookup] = &seat{hash: digest(key, lookup, pin), credential: credential}
				s.touch(key, r)
				// Formula War also joins by room name; store one alias pointing to the same room.
				if formula && path == "/api/rooms" && oldKey != key {
					s.rooms[oldKey] = r
				}
			}
		}
		for k, values := range recorder.Header() {
			for _, v := range values {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(recorder.Code)
		w.Write(recorder.Body.Bytes())
	})
}
