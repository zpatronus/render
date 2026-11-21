package secrethitler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/zpatronus/sidequests/internal/roomdb"
)

const storageGame = "secret-hitler"

// PersistentHandler restores the rooms before serving traffic and restarts pending bots.
func PersistentHandler(ctx context.Context, store *roomdb.Store) (http.Handler, error) {
	rooms, err := roomdb.Restore(store, storageGame, time.Now(), roomIdleTTL, func(r *room, active time.Time) { r.Active = active })
	if err != nil {
		return nil, err
	}
	s := &server{pinAuth: true, rooms: rooms, now: time.Now, storage: store, done: ctx.Done()}
	s.mu.Lock()
	for _, r := range rooms {
		s.advanceRoomBots(r)
	}
	s.mu.Unlock()
	go roomdb.Maintain(ctx, roomIdleTTL, s.sweepStorage, s.flushActivity)
	return s.handler(), nil
}

func roomExpiry(r *room) time.Time { return r.Active.Add(roomIdleTTL) }

func restoreRoom(current, previous *room) {
	previous.listeners, previous.botsRunning = current.listeners, current.botsRunning
	previous.activityDirty = current.activityDirty
	*current = *previous
}

func MemoryHandler(ctx context.Context) http.Handler {
	s := &server{pinAuth: true, rooms: make(map[string]*room), now: time.Now, done: ctx.Done()}
	go roomdb.Maintain(ctx, roomIdleTTL, s.sweepStorage, nil)
	return s.handler()
}

func (s *server) sweepStorage() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for code, r := range s.rooms {
		if !now.Before(roomExpiry(r)) {
			delete(s.rooms, code)
			r.notify()
		}
	}
	if s.storage != nil && s.flushActivityLocked() {
		if err := s.storage.Sweep(storageGame, now); err != nil {
			log.Printf("expire %s rooms: %v", storageGame, err)
		}
	}
}

func (s *server) flushActivity() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flushActivityLocked()
}

func (s *server) flushActivityLocked() bool {
	ok := true
	for code, r := range s.rooms {
		if !r.activityDirty {
			continue
		}
		if err := s.storage.Touch(storageGame, code, roomExpiry(r)); err != nil {
			log.Printf("refresh %s room expiry: %v", storageGame, err)
			ok = false
		} else {
			r.activityDirty = false
		}
	}
	return ok
}

func (s *server) activeRooms(now time.Time) int {
	count := 0
	for _, r := range s.rooms {
		if now.Before(roomExpiry(r)) {
			count++
		}
	}
	return count
}

func (s *server) waitBot(delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-s.done:
		return false
	case <-timer.C:
		return true
	}
}

func (s *server) beforeBot(r *room) []byte {
	if s.storage == nil {
		return nil
	}
	payload, err := json.Marshal(r)
	if err != nil {
		panic(err)
	}
	return payload
}

func (s *server) saveBot(r *room, before []byte) bool {
	if s.storage == nil {
		return true
	}
	if err := s.storage.Save(storageGame, r.Code, roomExpiry(r), r); err != nil {
		log.Printf("save %s bot action: %v", storageGame, err)
		previous := new(room)
		if err := json.Unmarshal(before, previous); err != nil {
			panic(err)
		}
		restoreRoom(r, previous)
		return false
	}
	r.activityDirty = false
	return true
}
