package db

import (
	"encoding/json"
	"time"

	"go.etcd.io/bbolt"
)

type ClientHeartbeat struct {
	Name      string    `json:"name"`
	Timestamp time.Time `json:"timestamp"`
	Missing   bool      `json:"missing"`
}

type DB struct {
	db *bbolt.DB
}

func Open(path string) (*DB, error) {
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	return &DB{db: db}, nil
}

func (d *DB) UpdateHeartbeat(name string, t time.Time, missing bool) error {
	return d.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("heartbeats"))
		if err != nil {
			return err
		}
		ch := ClientHeartbeat{Name: name, Timestamp: t, Missing: missing}
		data, err := json.Marshal(ch)
		if err != nil {
			return err
		}
		return b.Put([]byte(name), data)
	})
}

func (d *DB) GetAllHeartbeats() (map[string]ClientHeartbeat, error) {
	heartbeats := make(map[string]ClientHeartbeat)
	err := d.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("heartbeats"))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var ch ClientHeartbeat
			if err := json.Unmarshal(v, &ch); err == nil {
				heartbeats[string(k)] = ch
			}
			return nil
		})
	})
	return heartbeats, err
}

func (d *DB) SetMissing(name string, missing bool) error {
	return d.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("heartbeats"))
		if b == nil {
			return nil
		}
		v := b.Get([]byte(name))
		if v == nil {
			return nil
		}
		var ch ClientHeartbeat
		if err := json.Unmarshal(v, &ch); err != nil {
			return err
		}
		ch.Missing = missing
		data, err := json.Marshal(ch)
		if err != nil {
			return err
		}
		return b.Put([]byte(name), data)
	})
}

func (d *DB) Close() error {
	return d.db.Close()
}
