// Secret Hitler Chinese adaptation. Licensed under CC BY-NC-SA 4.0; see LICENSE.
package secrethitler

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

//go:embed web/index.html web/style.css web/app.js web/art/* LICENSE
var assets embed.FS

const roomIdleTTL = 15 * time.Minute
const imageCacheTTL = 24 * time.Hour

type room struct {
	Code, Host string
	Members    []member
	Tokens     map[string]string
	Bots       int
	Game       *game
	Active     time.Time
}
type server struct {
	mu    sync.Mutex
	rooms map[string]*room
	now   func() time.Time
}

func Handler() http.Handler { return (&server{rooms: map[string]*room{}, now: time.Now}).handler() }
func (s *server) handler() http.Handler {
	files := map[string][]byte{}
	etags := map[string]string{}
	fs.WalkDir(assets, "web", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := assets.ReadFile(name)
		if err != nil {
			return err
		}
		p := strings.TrimPrefix(name, "web")
		if p == "/index.html" {
			p = "/"
		}
		files[p] = data
		sum := sha256.Sum256(data)
		etags[p] = `"` + hex.EncodeToString(sum[:12]) + `"`
		return nil
	})
	files["/LICENSE"], _ = assets.ReadFile("LICENSE")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			s.api(w, r)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		data, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/art/") {
			w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
			w.Header().Set("Expires", s.now().Add(imageCacheTTL).UTC().Format(http.TimeFormat))
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		if etag := etags[r.URL.Path]; etag != "" {
			w.Header().Set("ETag", etag)
		}
		name := path.Base(r.URL.Path)
		if r.URL.Path == "/" {
			name = "index.html"
		}
		if r.URL.Path == "/LICENSE" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		}
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
	})
}
func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
func fail(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]string{"error": message})
}
func validName(name string) bool {
	return utf8.RuneCountInString(name) >= 1 && utf8.RuneCountInString(name) <= 20 && !strings.ContainsAny(name, "\r\n\t")
}
func roomCookie(w http.ResponseWriter, r *http.Request, code, token string) {
	http.SetCookie(w, &http.Cookie{Name: "sh_" + code, Value: token, Path: "/secret-hitler/", HttpOnly: true, Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https", SameSite: http.SameSiteStrictMode, MaxAge: 86400})
}

type request struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Avatar int    `json:"avatar"`
	Target string `json:"target"`
	Game   string `json:"game"`
	Step   int    `json:"step"`
	Count  *int   `json:"count"`
	Card   *int   `json:"card"`
	Yes    *bool  `json:"yes"`
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
			if err != nil || !strings.EqualFold(u.Host, q.Host) || (u.Scheme != "http" && u.Scheme != "https") {
				fail(w, 403, "请在本站操作")
				return
			}
		}
		if q.Header.Get("Content-Type") != "application/json" {
			fail(w, 415, "需要 JSON 请求")
			return
		}
	}
	var in request
	if q.Method == "POST" {
		if err := json.NewDecoder(http.MaxBytesReader(w, q.Body, 2048)).Decode(&in); err != nil {
			fail(w, 400, "输入格式有误")
			return
		}
	} else {
		in.Code = q.URL.Query().Get("code")
	}
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Name = strings.TrimSpace(in.Name)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for code, r := range s.rooms {
		if now.Sub(r.Active) >= roomIdleTTL {
			delete(s.rooms, code)
		}
	}
	if q.URL.Path == "/api/create" && q.Method == "POST" {
		if !validName(in.Name) || in.Avatar < 1 || in.Avatar > 20 {
			fail(w, 400, "请填写昵称并选择头像")
			return
		}
		if len(s.rooms) >= 100 {
			fail(w, 503, "房间已满，请稍后重试")
			return
		}
		code := strings.ToUpper(randomID(3))
		for s.rooms[code] != nil {
			code = strings.ToUpper(randomID(3))
		}
		id, token := randomID(8), randomID(24)
		r := &room{Code: code, Host: id, Members: []member{{ID: id, Name: in.Name, Avatar: in.Avatar, Bot: false}}, Tokens: map[string]string{token: id}, Bots: -1, Active: now}
		s.rooms[code] = r
		roomCookie(w, q, code, token)
		respond(w, 201, r.view(id))
		return
	}
	r := s.rooms[in.Code]
	if r == nil {
		fail(w, 404, "房间不存在或已过期")
		return
	}
	r.Active = now
	id := ""
	if c, err := q.Cookie("sh_" + r.Code); err == nil {
		id = r.Tokens[c.Value]
	}
	index := -1
	for i, m := range r.Members {
		if m.ID == id {
			index = i
			break
		}
	}
	if q.URL.Path == "/api/join" && q.Method == "POST" {
		if index >= 0 {
			respond(w, 200, r.view(id))
			return
		}
		if r.Game != nil {
			fail(w, 409, "本局已开始，请等待下一局；已有玩家可用原浏览器重连")
			return
		}
		if len(r.Members) >= 10 {
			fail(w, 409, "房间最多 10 人")
			return
		}
		if !validName(in.Name) || in.Avatar < 1 || in.Avatar > 20 {
			fail(w, 400, "请填写昵称并选择头像")
			return
		}
		for _, m := range r.Members {
			if m.Name == in.Name {
				fail(w, 409, "昵称已被使用")
				return
			}
		}
		id, token := randomID(8), randomID(24)
		r.Members = append(r.Members, member{ID: id, Name: in.Name, Avatar: in.Avatar})
		r.Tokens[token] = id
		roomCookie(w, q, r.Code, token)
		respond(w, 200, r.view(id))
		return
	}
	if index < 0 {
		fail(w, 403, "请先加入房间")
		return
	}
	if q.URL.Path == "/api/room" && q.Method == "GET" {
		if r.Game != nil {
			advanceBots(r.Game)
		}
		respond(w, 200, r.view(id))
		return
	}
	if q.URL.Path == "/api/private" && q.Method == "GET" {
		if r.Game == nil {
			fail(w, 409, "游戏尚未开始")
			return
		}
		respond(w, 200, r.Game.privateView(id))
		return
	}
	if q.Method != "POST" {
		fail(w, 405, "请使用 POST")
		return
	}
	action := strings.TrimPrefix(q.URL.Path, "/api/")
	switch action {
	case "bots":
		if id != r.Host || r.Game != nil || in.Count == nil {
			fail(w, 403, "只有房主可以设置机器人")
			return
		}
		minimum := max(0, 5-len(r.Members))
		maximum := 10 - len(r.Members)
		if *in.Count < minimum || *in.Count > maximum {
			fail(w, 400, "总人数需要保持 5–10 人")
			return
		}
		r.Bots = *in.Count
	case "avatar":
		if r.Game != nil || in.Avatar < 1 || in.Avatar > 20 {
			fail(w, 400, "当前不能更换头像")
			return
		}
		r.Members[index].Avatar = in.Avatar
	case "start":
		if id != r.Host || r.Game != nil {
			fail(w, 403, "只有房主可以开始新游戏")
			return
		}

		g, err := newGame(r.gameMembers())
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		r.Game = g
	case "reset":
		if id != r.Host || r.Game == nil || r.Game.Phase != finished || in.Game != r.Game.ID {
			fail(w, 409, "只能在本局结束后返回房间")
			return
		}
		r.Game = nil

	case "leave", "kick":
		if r.Game != nil && r.Game.Phase != finished {
			fail(w, 409, "游戏中保留席位，请使用原浏览器重新连接")
			return
		}
		remove := index
		if action == "kick" {
			if id != r.Host || in.Target == id {
				fail(w, 403, "只有房主可以移除其他玩家")
				return
			}
			remove = -1
			for i, m := range r.Members {
				if m.ID == in.Target {
					remove = i
				}
			}
			if remove < 0 {
				fail(w, 404, "玩家不存在")
				return
			}
		}
		removed := r.Members[remove].ID
		r.Members = append(r.Members[:remove], r.Members[remove+1:]...)
		for token, owner := range r.Tokens {
			if owner == removed {
				delete(r.Tokens, token)
			}
		}
		if len(r.Members) == 0 {
			delete(s.rooms, r.Code)
		} else if r.Host == removed {
			r.Host = r.Members[0].ID
		}
		if action == "leave" {
			respond(w, 200, map[string]bool{"left": true})
			return
		}
	default:
		g := r.Game
		if g == nil {
			fail(w, 409, "游戏尚未开始")
			return
		}
		if in.Game != g.ID || in.Step != g.Step {
			fail(w, 409, "游戏阶段已更新，请重试")
			return
		}
		if (action == "discard" || action == "enact") && in.Card == nil {
			fail(w, 400, "请明确选择一张政策")
			return
		}
		if (action == "vote" || action == "veto") && in.Yes == nil {
			fail(w, 400, "请选择 Ja! 或 Nein!")
			return
		}
		card := 0
		if in.Card != nil {
			card = *in.Card
		}
		yes := false
		if in.Yes != nil {
			yes = *in.Yes
		}
		if err := g.act(id, action, in.Target, card, yes); err != nil {
			fail(w, 409, err.Error())
			return
		}
	}
	if r.Game != nil {
		advanceBots(r.Game)
	}
	respond(w, 200, r.view(id))
}
