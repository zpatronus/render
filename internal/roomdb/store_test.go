package roomdb

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestMaintenanceSweepsWithoutRequests(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "rooms.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now()
	if err := store.Save("magic", "expired", now, map[string]string{"secret": "old"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save("magic", "active", now.Add(time.Minute), map[string]string{"secret": "new"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sweep := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		Maintain(ctx, time.Millisecond, func() { sweep <- store.Sweep("magic", now); cancel() }, nil)
	}()
	select {
	case err := <-sweep:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cleanup did not run")
	}
	<-done
	var expired, active int
	if err := store.db.QueryRow("SELECT count(*) FROM room_snapshots WHERE code='expired'").Scan(&expired); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow("SELECT count(*) FROM room_snapshots WHERE code='active'").Scan(&active); err != nil {
		t.Fatal(err)
	}
	if expired != 0 || active != 1 {
		t.Fatalf("expired rows=%d, active rows=%d", expired, active)
	}
}
