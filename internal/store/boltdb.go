package store

import (
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	hostLogs  = "host-logs"
	logKeyFmt = "2006-01-02T15:04:05.000000000Z07:00"
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
	return b.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(hostLogs))
		if root == nil {
			return fmt.Errorf("Failed to get %q bucket, was nil", hostLogs)
		}
		bucket, err := root.CreateBucketIfNotExists([]byte(host))
		if err != nil {
			return fmt.Errorf("Failed to create %q bucket: %w", host, err)
		}

		key := []byte(time.Now().UTC().Format(logKeyFmt))
		return bucket.Put(key, []byte(entry))
	})
}
