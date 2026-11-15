package undercover

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

//go:embed web/index.html web/style.css web/app.js words.tsv
var assets embed.FS

const roomIdleTTL = 15 * time.Minute

type pair struct{ a, b string }
type player struct{ id, name, word string }
type room struct {
	listeners            map[chan struct{}]struct{}
	code, host           string
	players              []player
	spies, blanks, round int
	dealt                bool
	active               time.Time
	lastPair             int
}
type server struct {
	mu    sync.Mutex
	rooms map[string]*room
	pairs []pair
	now   func() time.Time
}

func loadPairs() []pair {
	data, err := assets.ReadFile("words.tsv")
	if err != nil {
		panic(err)
	}
	var result []pair
	seen := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Split(strings.TrimSpace(line), "\t")
		if len(fields) != 2 || fields[0] == "" || fields[1] == "" || fields[0] == fields[1] {
			continue
		}
		a, b := fields[0], fields[1]
		if a > b {
			a, b = b, a
		}
		key := a + "\t" + b
		if !seen[key] {
			seen[key] = true
			result = append(result, pair{fields[0], fields[1]})
		}
	}
	if len(result) == 0 {
		panic("undercover: empty word bank")
	}
	return result
}
func random(n int) int {
	v, e := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if e != nil {
		panic(e)
	}
	return int(v.Int64())
}
func token(n int) string {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func Handler() http.Handler {
	return (&server{rooms: map[string]*room{}, pairs: loadPairs(), now: time.Now}).handler()
}

func (s *server) handler() http.Handler {
	files, _ := fs.Sub(assets, "web")
	static := http.FileServer(http.FS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			if r.URL.Path == "/api/events" {
				s.events(w, r)
				return
			}
			s.api(w, r)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		if r.URL.Path != "/" && r.URL.Path != "/style.css" && r.URL.Path != "/app.js" {
			http.NotFound(w, r)
			return
		}
		static.ServeHTTP(w, r)
	})
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}

type memberView struct {
	Name string `json:"name"`
	Host bool   `json:"host"`
	You  bool   `json:"you"`
}
type roomView struct {
	Code    string       `json:"code"`
	Players []memberView `json:"players"`
	Spies   int          `json:"spies"`
	Blanks  int          `json:"blanks"`
	Round   int          `json:"round"`
	Dealt   bool         `json:"dealt"`
	Host    bool         `json:"host"`
}

func view(r *room, id string) roomView {
	v := roomView{Code: r.code, Spies: r.spies, Blanks: r.blanks, Round: r.round, Dealt: r.dealt, Host: r.host == id}
	for _, p := range r.players {
		v.Players = append(v.Players, memberView{Name: p.name, Host: r.host == p.id, You: p.id == id})
	}
	return v
}
func setCookie(w http.ResponseWriter, r *http.Request, code, id string) {
	http.SetCookie(w, &http.Cookie{Name: "undercover_" + code, Value: id, Path: "/undercover/", HttpOnly: true, Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https", SameSite: http.SameSiteStrictMode, MaxAge: 7200})
}
func validName(n string) bool {
	return utf8.RuneCountInString(n) > 0 && utf8.RuneCountInString(n) <= 20 && !strings.ContainsAny(n, "\r\n\t")
}
func (s *server) api(w http.ResponseWriter, q *http.Request) {
	if q.Method != "GET" && q.Method != "POST" {
		fail(w, 405, "请求方式不支持")
		return
	}
	if q.Method == "POST" {
		if q.Header.Get("Sec-Fetch-Site") == "cross-site" {
			fail(w, 403, "请在本站操作")
			return
		}
		if origin := q.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || !strings.EqualFold(u.Host, q.Host) || (u.Scheme != "https" && u.Scheme != "http") {
				fail(w, 403, "请在本站操作")
				return
			}
		}
		if ct := q.Header.Get("Content-Type"); ct != "application/json" {
			fail(w, 415, "需要 JSON 请求")
			return
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for code, r := range s.rooms {
		if now.Sub(r.active) >= roomIdleTTL {
			delete(s.rooms, code)
			r.notify()
		}
	}
	if q.URL.Path == "/api/info" && q.Method == "GET" {
		respond(w, 200, map[string]int{"pairs": len(s.pairs)})
		return
	}
	var in struct {
		Code   string `json:"code"`
		Name   string `json:"name"`
		Spies  int    `json:"spies"`
		Blanks int    `json:"blanks"`
		Round  int    `json:"round"`
	}
	if q.Method == "POST" {
		if err := json.NewDecoder(http.MaxBytesReader(w, q.Body, 2048)).Decode(&in); err != nil {
			fail(w, 400, "输入有误")
			return
		}
	} else {
		in.Code = q.URL.Query().Get("code")
	}
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Name = strings.TrimSpace(in.Name)
	if q.URL.Path == "/api/create" && q.Method == "POST" {
		if !validName(in.Name) {
			fail(w, 400, "昵称需要 1–20 个字")
			return
		}

		if len(s.rooms) >= 100 {
			fail(w, 503, "房间已满，请稍后再试")
			return
		}
		code := strings.ToUpper(token(3))
		for s.rooms[code] != nil {
			code = strings.ToUpper(token(3))
		}
		id := token(16)
		r := &room{code: code, host: id, players: []player{{id: id, name: in.Name}}, spies: 1, blanks: 0, active: now, lastPair: -1}
		s.rooms[code] = r
		setCookie(w, q, code, id)
		respond(w, 201, view(r, id))
		return
	}
	r := s.rooms[in.Code]
	if r == nil {
		fail(w, 404, "房间不存在或已过期")
		return
	}
	// Every request addressed to an existing room counts as activity.
	r.active = now
	if q.Method == "POST" || q.URL.Path == "/api/room" {
		defer r.notify()
	}
	id := ""
	if c, err := q.Cookie("undercover_" + in.Code); err == nil {
		id = c.Value
	}
	index := -1
	for i, p := range r.players {
		if p.id == id {
			index = i
			break
		}
	}
	if q.URL.Path == "/api/join" && q.Method == "POST" {
		if index >= 0 {
			respond(w, 200, view(r, id))
			return
		}
		if r.dealt {
			fail(w, 409, "本局已发词，等房主开启下一局再加入")
			return
		}
		if len(r.players) >= 20 {
			fail(w, 409, "最多 20 人")
			return
		}
		if !validName(in.Name) {
			fail(w, 400, "昵称需要 1–20 个字")
			return
		}
		for _, p := range r.players {
			if p.name == in.Name {
				fail(w, 409, "昵称已被使用")
				return
			}
		}
		id = token(16)
		r.players = append(r.players, player{id: id, name: in.Name})
		setCookie(w, q, r.code, id)
		respond(w, 200, view(r, id))
		return
	}
	if index < 0 {
		fail(w, 403, "请先加入房间")
		return
	}
	switch q.URL.Path {
	case "/api/room":
		if q.Method != "GET" {
			fail(w, 405, "请使用 GET")
			return
		}
	case "/api/deal":
		if q.Method != "POST" {
			fail(w, 405, "请使用 POST")
			return
		}
		if id != r.host {
			fail(w, 403, "只有房主可以发词")
			return
		}
		if r.dealt || in.Round != r.round {
			fail(w, 409, "本局已发词，请刷新")
			return
		}
		if in.Spies < 1 || in.Spies > 9 || in.Blanks < 0 || in.Blanks > 9 || in.Spies+in.Blanks >= len(r.players) || len(r.players) < 3 {
			fail(w, 400, "至少 3 人，至少 1 名卧底，并保留至少 1 名平民")
			return
		}
		r.spies, r.blanks = in.Spies, in.Blanks
		pick := random(len(s.pairs))
		if len(s.pairs) > 1 && pick == r.lastPair {
			pick = (pick + 1 + random(len(s.pairs)-1)) % len(s.pairs)
		}
		r.lastPair = pick
		a, b := s.pairs[pick].a, s.pairs[pick].b
		if random(2) == 1 {
			a, b = b, a
		}
		words := make([]string, len(r.players))
		for i := range words {
			words[i] = a
			if i < r.spies {
				words[i] = b
			}
			if i >= r.spies && i < r.spies+r.blanks {
				words[i] = ""
			}
		}
		for i := len(words) - 1; i > 0; i-- {
			j := random(i + 1)
			words[i], words[j] = words[j], words[i]
		}
		for i := range r.players {
			r.players[i].word = words[i]
		}
		r.round++
		r.dealt = true
	case "/api/word":
		if q.Method != "POST" {
			fail(w, 405, "请使用 POST")
			return
		}
		if !r.dealt || in.Round != r.round {
			fail(w, 409, "本局未发词或已结束")
			return
		}
		respond(w, 200, map[string]any{"word": r.players[index].word, "blank": r.players[index].word == "", "round": r.round})
		return
	case "/api/reset":
		if q.Method != "POST" {
			fail(w, 405, "请使用 POST")
			return
		}
		if id != r.host {
			fail(w, 403, "只有房主可以开启下一局")
			return
		}
		if !r.dealt || in.Round != r.round {
			fail(w, 409, "房间状态已更新，请刷新")
			return
		}
		r.dealt = false
		for i := range r.players {
			r.players[i].word = ""
		}
	case "/api/leave":
		if q.Method != "POST" {
			fail(w, 405, "请使用 POST")
			return
		}
		r.players = append(r.players[:index], r.players[index+1:]...)
		if len(r.players) == 0 {
			delete(s.rooms, r.code)
		} else if r.host == id {
			r.host = r.players[0].id
		}
		setCookie(w, q, r.code, "")
		respond(w, 200, map[string]bool{"left": true})
		return
	default:
		fail(w, 404, "接口不存在")
		return
	}
	respond(w, 200, view(r, id))
}
