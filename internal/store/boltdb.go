package store

import (
	"encoding/binary"
	"fmt"
	"slices"
	"time"

	"github.com/vmihailenco/msgpack/v5"

	"github.com/jhillyerd/labcoat/internal/nix"
	bolt "go.etcd.io/bbolt"
)

const (
	flakeVersions = "flake-versions"
	hostLogs      = "host-logs"
	deployHistory = "deploy-history"
)

type BoltDB struct {
	db *bolt.DB
}

func NewBoltDB(db *bolt.DB) (*BoltDB, error) {
	err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range []string{flakeVersions, hostLogs, deployHistory} {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return fmt.Errorf("Failed to create %q root bucket: %w", name, err)
			}
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

type FlakeVersion struct {
	ResolvedUrl  string
	Revision     string
	LastModified time.Time
	Fingerprint  string
	Dirty        bool
	StoredAt     time.Time
}

func (b *BoltDB) StoreFlakeVersion(meta nix.FlakeMetadata) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(flakeVersions))
		if bucket == nil {
			return fmt.Errorf("Failed to get %q bucket, was nil", flakeVersions)
		}

		key := []byte(meta.Fingerprint)
		if existing := bucket.Get(key); existing != nil {
			return nil
		}

		lastModified := time.Unix(meta.LastModified, 0)
		if meta.Dirty {
			lastModified = time.Now()
		}

		version := FlakeVersion{
			ResolvedUrl:  meta.ResolvedUrl,
			Revision:     meta.Revision,
			LastModified: lastModified,
			Fingerprint:  meta.Fingerprint,
			Dirty:        meta.Dirty,
			StoredAt:     time.Now(),
		}

		data, err := msgpack.Marshal(version)
		if err != nil {
			return fmt.Errorf("Failed to marshal flake version: %w", err)
		}

		return bucket.Put(key, data)
	})
}

func (b *BoltDB) ListFlakeVersions() ([]FlakeVersion, error) {
	var versions []FlakeVersion

	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(flakeVersions))
		if bucket == nil {
			return fmt.Errorf("Failed to get %q bucket, was nil", flakeVersions)
		}

		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var fv FlakeVersion
			if err := msgpack.Unmarshal(v, &fv); err != nil {
				return fmt.Errorf("Failed to unmarshal flake version: %w", err)
			}
			versions = append(versions, fv)
		}

		return nil
	})

	slices.SortFunc(versions, func(a, b FlakeVersion) int {
		if a.StoredAt.After(b.StoredAt) {
			return -1
		} else if a.StoredAt.Before(b.StoredAt) {
			return 1
		}
		return 0
	})

	return versions, err
}

func (b *BoltDB) GetFlakeVersion(fingerprint string) (*FlakeVersion, error) {
	var version *FlakeVersion

	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(flakeVersions))
		if bucket == nil {
			return fmt.Errorf("Failed to get %q bucket, was nil", flakeVersions)
		}

		data := bucket.Get([]byte(fingerprint))
		if data == nil {
			return nil
		}

		var v FlakeVersion
		if err := msgpack.Unmarshal(data, &v); err != nil {
			return fmt.Errorf("Failed to unmarshal flake version: %w", err)
		}

		version = &v
		return nil
	})

	return version, err
}

// DeploymentRecord represents a single deployment attempt for a host.
type DeploymentRecord struct {
	Timestamp   time.Time
	Fingerprint string
	Success     bool
}

// RecordDeployment records a deployment attempt for a host.
func (b *BoltDB) RecordDeployment(host string, record DeploymentRecord) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(deployHistory))
		if root == nil {
			return fmt.Errorf("Failed to get %q bucket, was nil", deployHistory)
		}
		bucket, err := root.CreateBucketIfNotExists([]byte(host))
		if err != nil {
			return fmt.Errorf("Failed to create %q deployment bucket: %w", host, err)
		}

		data, err := msgpack.Marshal(record)
		if err != nil {
			return fmt.Errorf("Failed to marshal deployment record: %w", err)
		}

		return bucket.Put(ttob(record.Timestamp), data)
	})
}

// ListDeployments returns deployment history for a host, most recent first.
func (b *BoltDB) ListDeployments(host string) ([]DeploymentRecord, error) {
	var records []DeploymentRecord

	err := b.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(deployHistory))
		if root == nil {
			return fmt.Errorf("Failed to get %q bucket, was nil", deployHistory)
		}
		bucket := root.Bucket([]byte(host))
		if bucket == nil {
			return nil
		}

		// Keys are big-endian timestamps, so iterating Last/Prev yields
		// most-recent-first without needing an in-memory sort.
		c := bucket.Cursor()
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var rec DeploymentRecord
			if err := msgpack.Unmarshal(v, &rec); err != nil {
				return fmt.Errorf("Failed to unmarshal deployment record: %w", err)
			}
			records = append(records, rec)
		}

		return nil
	})

	return records, err
}

// LatestDeployment returns the most recent deployment for a host, or nil if none.
func (b *BoltDB) LatestDeployment(host string) (*DeploymentRecord, error) {
	var record *DeploymentRecord

	err := b.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(deployHistory))
		if root == nil {
			return fmt.Errorf("Failed to get %q bucket, was nil", deployHistory)
		}
		bucket := root.Bucket([]byte(host))
		if bucket == nil {
			return nil
		}

		// Keys are big-endian timestamps, so last key is most recent.
		_, v := bucket.Cursor().Last()
		if v == nil {
			return nil
		}

		var rec DeploymentRecord
		if err := msgpack.Unmarshal(v, &rec); err != nil {
			return fmt.Errorf("Failed to unmarshal deployment record: %w", err)
		}
		record = &rec
		return nil
	})

	return record, err
}

// LatestSuccessfulDeployment returns the most recent successful deployment for a
// host, or nil if none.
func (b *BoltDB) LatestSuccessfulDeployment(host string) (*DeploymentRecord, error) {
	var record *DeploymentRecord

	err := b.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(deployHistory))
		if root == nil {
			return fmt.Errorf("Failed to get %q bucket, was nil", deployHistory)
		}
		bucket := root.Bucket([]byte(host))
		if bucket == nil {
			return nil
		}

		// Keys are big-endian timestamps, so iterating Last/Prev yields
		// most-recent-first. Return the first record with Success == true.
		c := bucket.Cursor()
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var rec DeploymentRecord
			if err := msgpack.Unmarshal(v, &rec); err != nil {
				return fmt.Errorf("Failed to unmarshal deployment record: %w", err)
			}
			if rec.Success {
				record = &rec
				return nil
			}
		}

		return nil
	})

	return record, err
}

// CommitsBehindInfo contains behind-count information for a host's deployment.
type CommitsBehindInfo struct {
	Behind int
	Dirty  bool
}

// HostsCommitsBehind returns the number of clean (non-dirty) flake versions stored after
// each host's most recent successful deployment, and whether that deployment was from a dirty tree.
// Hosts with no successful deployment or whose fingerprint is not stored are omitted from the result.
func (b *BoltDB) HostsCommitsBehind(hosts []string) (map[string]CommitsBehindInfo, error) {
	versions, err := b.ListFlakeVersions()
	if err != nil {
		return nil, fmt.Errorf("failed to list flake versions: %w", err)
	}

	result := make(map[string]CommitsBehindInfo, len(hosts))

	for _, host := range hosts {
		deploy, err := b.LatestSuccessfulDeployment(host)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest deployment for %q: %w", host, err)
		}
		if deploy == nil {
			continue
		}

		// Find the deployed version in the versions list.
		var deployVer *FlakeVersion
		for i := range versions {
			if versions[i].Fingerprint == deploy.Fingerprint {
				deployVer = &versions[i]
				break
			}
		}
		if deployVer == nil {
			continue
		}

		// Count clean versions modified after the deployed version.
		behind := 0
		for _, v := range versions {
			if v.LastModified.After(deployVer.LastModified) && !v.Dirty {
				behind++
			}
		}

		result[host] = CommitsBehindInfo{
			Behind: behind,
			Dirty:  deployVer.Dirty,
		}
	}

	return result, nil
}
