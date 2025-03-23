package store

import (
	"encoding/binary"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	hostLogs = "host-logs"
)

type BoltDB struct {
	db *bolt.DB
}

func NewBoltDB(db *bolt.DB) (*BoltDB, error) {
	err := db.Update(func(tx *bolt.Tx) error {
		// Create buckets.
		_, err := tx.CreateBucketIfNotExists([]byte(hostLogs))
		if err != nil {
			return fmt.Errorf("Failed to create %q root bucket: %w", hostLogs, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &BoltDB{db: db}, nil
}

func (b *BoltDB) WriteHostLog(host string, entry string) error {
	now := time.Now()

	return b.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(hostLogs))
		if root == nil {
			return fmt.Errorf("Failed to get %q bucket, was nil", hostLogs)
		}
		bucket, err := root.CreateBucketIfNotExists([]byte(host))
		if err != nil {
			return fmt.Errorf("Failed to create %q bucket: %w", host, err)
		}

		return bucket.Put(ttob(now), []byte(entry))
	})
}

type HostLogEntry struct {
	Time  time.Time
	Entry string
}

func (b *BoltDB) ReadHostLogs(host string) ([]HostLogEntry, error) {
	entries := make([]HostLogEntry, 0, 64)

	err := b.db.View(func(tx *bolt.Tx) error {
		entries = entries[:0]

		root := tx.Bucket([]byte(hostLogs))
		if root == nil {
			return fmt.Errorf("Failed to get %q bucket, was nil", hostLogs)
		}
		bucket := root.Bucket([]byte(host))
		if bucket == nil {
			return nil
		}

		// Append entire bucket to entries.
		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			// TODO expire old log entries
			entries = append(entries, HostLogEntry{
				Time:  btot(k),
				Entry: string(v),
			})
		}

		return nil
	})

	return entries, err
}

func ttob(t time.Time) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(t.UnixMicro()))
	return b
}

func btot(b []byte) time.Time {
	return time.UnixMicro(int64(binary.BigEndian.Uint64(b)))
}
