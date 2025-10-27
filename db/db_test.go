package db

import (
	"os"
	"testing"
	"time"
)

func TestHeartbeatAndMissingState(t *testing.T) {
	os.Remove("test.db")
	db, err := Open("test.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close() error: %v", err)
		}
		if err := os.Remove("test.db"); err != nil && !os.IsNotExist(err) {
			t.Fatalf("os.Remove error: %v", err)
		}
	}()

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
	os.Remove("test_multi.db")
	db, err := Open("test_multi.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() {
		db.Close()
		os.Remove("test_multi.db")
	}()

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
	os.Remove("test_overwrite.db")
	db, err := Open("test_overwrite.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() {
		db.Close()
		os.Remove("test_overwrite.db")
	}()

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

func TestSetMissingNonexistent(t *testing.T) {
	os.Remove("test_nonexist.db")
	db, err := Open("test_nonexist.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() {
		db.Close()
		os.Remove("test_nonexist.db")
	}()

	// SetMissing on non-existent entry should not error
	err = db.SetMissing("nonexistent", true)
	if err != nil {
		t.Errorf("SetMissing on nonexistent entry returned error: %v", err)
	}
}

func TestGetAllHeartbeatsEmpty(t *testing.T) {
	os.Remove("test_empty.db")
	db, err := Open("test_empty.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() {
		db.Close()
		os.Remove("test_empty.db")
	}()

	beats, err := db.GetAllHeartbeats()
	if err != nil {
		t.Errorf("GetAllHeartbeats on empty db returned error: %v", err)
	}
	if len(beats) != 0 {
		t.Errorf("expected 0 heartbeats, got %d", len(beats))
	}
}
