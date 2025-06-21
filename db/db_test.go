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
	defer func() { db.Close(); os.Remove("test.db") }()

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
