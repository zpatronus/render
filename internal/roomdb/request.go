package roomdb

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
	"unicode/utf8"
)

type Seat struct {
	Hash        [32]byte
	Token       string
	Failures    int
	LockedUntil time.Time
}

type Pins map[string]Seat

func pinHash(code, name, pin string) [32]byte {
	return sha256.Sum256([]byte(code + ":" + name + ":" + pin))
}
func validPIN(pin string) bool { return len(pin) == 4 && strings.Trim(pin, "0123456789") == "" }
func ValidLogin(name, pin string) bool {
	return strings.TrimSpace(name) != "" && utf8.RuneCountInString(name) <= 24 && validPIN(pin)
}

// Authenticate returns an internal game token and a status for an existing seat.
// An unknown seat returns an empty token and no status so a new player may join.
func (p Pins) Authenticate(code, name, pin string, now time.Time) (string, int) {
	name = strings.ToLower(strings.TrimSpace(name))
	seat, ok := p[name]
	if !ok {
		return "", 0
	}
	if now.Before(seat.LockedUntil) {
		return "", 429
	}
	hash := pinHash(code, name, pin)
	if subtle.ConstantTimeCompare(hash[:], seat.Hash[:]) != 1 {
		seat.Failures++
		if seat.Failures >= 5 {
			seat.Failures = 0
			seat.LockedUntil = now.Add(time.Minute)
		}
		p[name] = seat
		return "", 403
	}
	if seat.Failures != 0 {
		seat.Failures = 0
		p[name] = seat
	}
	return seat.Token, 0
}

func (p *Pins) Set(code, name, pin, token string) {
	if *p == nil {
		*p = make(Pins)
	}
	name = strings.ToLower(strings.TrimSpace(name))
	(*p)[name] = Seat{Hash: pinHash(code, name, pin), Token: token}
}

func (p Pins) Prune(tokens map[string]string) {
	for name, seat := range p {
		if tokens[seat.Token] == "" {
			delete(p, name)
		}
	}
}

func LoginError(status int) string {
	if status == 429 {
		return "PIN 尝试过多，请稍后再试"
	}
	return "这个用户名的 PIN 不正确"
}

// Record holds HTTP headers and the body until the room change has been saved.
func Record[T any](w http.ResponseWriter, store *Store, game string, code *string, rooms map[string]*T,
	expires func(*T) time.Time, restore func(*T, *T), fail func(http.ResponseWriter, int, string),
) (http.ResponseWriter, func()) {
	original := rooms[*code]
	var before []byte
	if original != nil {
		before, _ = json.Marshal(original)
	}
	response := httptest.NewRecorder()
	return response, func() {
		if current := rooms[*code]; current != nil {
			if err := store.Save(game, *code, expires(current), current); err != nil {
				log.Printf("save %s room: %v", game, err)
				if original == nil {
					delete(rooms, *code)
				} else {
					previous := new(T)
					if err := json.Unmarshal(before, previous); err != nil {
						panic(err)
					}
					restore(original, previous)
				}
				fail(w, 503, "暂时无法保存房间，请重试")
				return
			}
		}
		for key, values := range response.Header() {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(response.Code)
		w.Write(response.Body.Bytes())
	}
}
