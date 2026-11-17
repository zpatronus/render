package abracada

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (s *server) events(w http.ResponseWriter, q *http.Request) {
	if q.Method != "GET" {
		fail(w, 405, "请使用 GET")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		fail(w, 500, "不支持实时连接")
		return
	}
	code := strings.ToUpper(strings.TrimSpace(q.URL.Query().Get("code")))
	credential, err := q.Cookie("abr_" + code)
	if err != nil {
		fail(w, 403, "请先加入房间")
		return
	}
	s.mu.Lock()
	r := s.rooms[code]
	if r == nil || s.now().Sub(r.Active) >= roomIdleTTL {
		s.mu.Unlock()
		fail(w, 404, "房间不存在或已过期")
		return
	}
	r.Active = s.now()
	snapshot := func() ([]byte, bool) {
		if s.rooms[code] == r && s.now().Sub(r.Active) >= roomIdleTTL {
			delete(s.rooms, code)
			r.notify()
			return nil, false
		}
		if s.rooms[code] != r {
			return nil, false
		}
		id := credential.Value
		id = r.Tokens[id]
		if id == "" {
			return nil, false
		}

		data, _ := json.Marshal(r.view(id))
		return data, true
	}
	data, authorized := snapshot()
	if !authorized {
		s.mu.Unlock()
		fail(w, 403, "请先加入房间")
		return
	}
	if len(r.listeners) >= 64 {
		s.mu.Unlock()
		fail(w, 429, "实时连接过多")
		return
	}
	if r.listeners == nil {
		r.listeners = make(map[chan struct{}]struct{})
	}
	changes := make(chan struct{}, 1)
	r.listeners[changes] = struct{}{}
	if r.reactions == nil {
		r.reactions = make(map[chan reaction]struct{})
	}
	reactions := make(chan reaction, 24)
	r.reactions[reactions] = struct{}{}
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(r.listeners, changes); delete(r.reactions, reactions); s.mu.Unlock() }()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	write := func(payload []byte) bool {
		// A stalled browser must not hold a handler forever.
		controller := http.NewResponseController(w)
		_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
		_, err := fmt.Fprintf(w, "data: %s\n\n", payload)
		if err == nil {
			flusher.Flush()
		}
		_ = controller.SetWriteDeadline(time.Time{})
		return err == nil
	}
	if !write(data) {
		return
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-q.Context().Done():
			return
		case value := <-reactions:
			s.mu.Lock()
			authorized := s.rooms[code] == r && r.Tokens[credential.Value] != ""
			s.mu.Unlock()
			if !authorized {
				return
			}
			payload, _ := json.Marshal(value)
			controller := http.NewResponseController(w)
			_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
			_, err := fmt.Fprintf(w, "event: reaction\ndata: %s\n\n", payload)
			if err != nil {
				return
			}
			flusher.Flush()
			_ = controller.SetWriteDeadline(time.Time{})
			continue
		case <-changes:
		case <-ticker.C:
		}
		s.mu.Lock()
		next, alive := snapshot()
		s.mu.Unlock()
		if !alive {
			fmt.Fprint(w, "event: expired\ndata: {}\n\n")
			flusher.Flush()
			return
		}
		// Periodic full snapshots also keep proxies alive and reconcile reconnects.
		if !write(next) {
			return
		}
	}
}
