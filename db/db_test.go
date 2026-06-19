package db

import (
	"path/filepath"
	"testing"
	"time"
)

func testDBPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name)
}

func TestHeartbeatAndMissingState(t *testing.T) {
	db, err := Open(testDBPath(t, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	name := "client1"
	now := time.Now().Truncate(time.Second)
	if err := db.UpdateHeartbeat(name, now, false); err != nil {
		t.Fatalf("update: %v", err)
	}
	beats, err := db.GetAllHeartbeats()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	ch, ok := beats[name]
	if !ok || !ch.Timestamp.Equal(now) || ch.Missing {
		t.Errorf("unexpected heartbeat: %+v", ch)
	}
	if err := db.SetMissing(name, true); err != nil {
		t.Fatalf("set missing: %v", err)
	}
	beats, _ = db.GetAllHeartbeats()
	ch = beats[name]
	if !ch.Missing {
		t.Error("missing state not set")
	}
	if err := db.SetMissing(name, false); err != nil {
		t.Fatalf("set missing false: %v", err)
	}
	beats, _ = db.GetAllHeartbeats()
	ch = beats[name]
	if ch.Missing {
		t.Error("missing state not cleared")
	}
}

func TestMultipleHeartbeats(t *testing.T) {
	db, err := Open(testDBPath(t, "test_multi.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	now := time.Now()
	names := []string{"client1", "client2", "client3"}
	for i, name := range names {
		if err := db.UpdateHeartbeat(name, now.Add(time.Duration(i)*time.Second), false); err != nil {
			t.Fatalf("update %s: %v", name, err)
		}
	}

	beats, err := db.GetAllHeartbeats()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(beats) != len(names) {
		t.Errorf("expected %d heartbeats, got %d", len(names), len(beats))
	}
	for _, name := range names {
		if _, ok := beats[name]; !ok {
			t.Errorf("heartbeat for %s not found", name)
		}
	}
}

func TestUpdateHeartbeatOverwrite(t *testing.T) {
	db, err := Open(testDBPath(t, "test_overwrite.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	name := "client1"
	time1 := time.Now()
	time2 := time1.Add(1 * time.Hour)

	if err := db.UpdateHeartbeat(name, time1, false); err != nil {
		t.Fatalf("first update: %v", err)
	}
	beats, _ := db.GetAllHeartbeats()
	if !beats[name].Timestamp.Equal(time1) {
		t.Error("first timestamp not stored correctly")
	}

	if err := db.UpdateHeartbeat(name, time2, true); err != nil {
		t.Fatalf("second update: %v", err)
	}
	beats, _ = db.GetAllHeartbeats()
	if !beats[name].Timestamp.Equal(time2) {
		t.Error("timestamp not updated")
	}
	if !beats[name].Missing {
		t.Error("missing state not updated")
	}
}

func TestGetHeartbeat(t *testing.T) {
	db, err := Open(testDBPath(t, "test_get.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	name := "client1"
	now := time.Now().Truncate(time.Second)
	if err := db.UpdateHeartbeat(name, now, false); err != nil {
		t.Fatalf("update: %v", err)
	}

	ch, ok := db.Get(name)
	if !ok {
		t.Fatal("Get returned not-found for existing entry")
	}
	if ch.Name != name || !ch.Timestamp.Equal(now) || ch.Missing {
		t.Errorf("unexpected heartbeat: %+v", ch)
	}

	// Non-existent
	ch, ok = db.Get("nonexistent")
	if ok {
		t.Error("expected not-found for nonexistent entry")
	}
	if ch.Name != "" {
		t.Errorf("expected zero value, got %+v", ch)
	}
}

func TestSetMissingNonexistent(t *testing.T) {
	db, err := Open(testDBPath(t, "test_nonexist.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// SetMissing on non-existent entry should not error
	err = db.SetMissing("nonexistent", true)
	if err != nil {
		t.Errorf("SetMissing on nonexistent entry returned error: %v", err)
	}
}

func TestGetAllHeartbeatsEmpty(t *testing.T) {
	db, err := Open(testDBPath(t, "test_empty.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	beats, err := db.GetAllHeartbeats()
	if err != nil {
		t.Errorf("GetAllHeartbeats on empty db returned error: %v", err)
	}
	if len(beats) != 0 {
		t.Errorf("expected 0 heartbeats, got %d", len(beats))
	}
}

func TestDelete(t *testing.T) {
	db, err := Open(testDBPath(t, "test_delete.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// Insert two entries
	if err := db.UpdateHeartbeat("alpha", time.Now(), false); err != nil {
		t.Fatalf("update alpha: %v", err)
	}
	if err := db.UpdateHeartbeat("beta", time.Now(), false); err != nil {
		t.Fatalf("update beta: %v", err)
	}

	// Delete alpha
	if err := db.Delete("alpha"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	beats, _ := db.GetAllHeartbeats()
	if _, ok := beats["alpha"]; ok {
		t.Error("alpha should have been deleted")
	}
	if _, ok := beats["beta"]; !ok {
		t.Error("beta should still exist")
	}

	// Delete non-existent key should not error
	if err := db.Delete("nonexistent"); err != nil {
		t.Errorf("Delete nonexistent returned error: %v", err)
	}
}
