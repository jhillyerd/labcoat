package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
	bolt "go.etcd.io/bbolt"
)

func setupTestDB(t *testing.T) *BoltDB {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := bolt.Open(dbPath, 0600, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	bdb, err := NewBoltDB(db)
	require.NoError(t, err)
	return bdb
}

func TestRecordDeployment(t *testing.T) {
	bdb := setupTestDB(t)
	host := "testhost"

	record := DeploymentRecord{
		Timestamp:   time.Date(2025, 5, 1, 12, 0, 0, 0, time.UTC),
		Fingerprint: "abc123def456",
		Success:     true,
	}

	err := bdb.RecordDeployment(host, record)
	require.NoError(t, err)
}

func TestListDeploymentsEmpty(t *testing.T) {
	bdb := setupTestDB(t)

	records, err := bdb.ListDeployments("nonexistent")
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestListDeploymentsOrderedByMostRecent(t *testing.T) {
	bdb := setupTestDB(t)
	host := "myhost"

	// Insert in chronological order.
	records := []DeploymentRecord{
		{Timestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Fingerprint: "aaa", Success: true},
		{Timestamp: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC), Fingerprint: "bbb", Success: false},
		{Timestamp: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC), Fingerprint: "ccc", Success: true},
	}

	for _, r := range records {
		err := bdb.RecordDeployment(host, r)
		require.NoError(t, err)
	}

	got, err := bdb.ListDeployments(host)
	require.NoError(t, err)
	require.Len(t, got, 3)

	// Should be most recent first.
	assert.Equal(t, "bbb", got[0].Fingerprint)
	assert.False(t, got[0].Success)
	assert.Equal(t, "ccc", got[1].Fingerprint)
	assert.True(t, got[1].Success)
	assert.Equal(t, "aaa", got[2].Fingerprint)
	assert.True(t, got[2].Success)
}

func TestListDeploymentsPerHostIsolation(t *testing.T) {
	bdb := setupTestDB(t)

	err := bdb.RecordDeployment("host-a", DeploymentRecord{
		Timestamp:   time.Now(),
		Fingerprint: "fp-a",
		Success:     true,
	})
	require.NoError(t, err)

	err = bdb.RecordDeployment("host-b", DeploymentRecord{
		Timestamp:   time.Now(),
		Fingerprint: "fp-b",
		Success:     false,
	})
	require.NoError(t, err)

	recordsA, err := bdb.ListDeployments("host-a")
	require.NoError(t, err)
	require.Len(t, recordsA, 1)
	assert.Equal(t, "fp-a", recordsA[0].Fingerprint)

	recordsB, err := bdb.ListDeployments("host-b")
	require.NoError(t, err)
	require.Len(t, recordsB, 1)
	assert.Equal(t, "fp-b", recordsB[0].Fingerprint)
}

func TestLatestDeploymentNone(t *testing.T) {
	bdb := setupTestDB(t)

	rec, err := bdb.LatestDeployment("nonexistent")
	require.NoError(t, err)
	assert.Nil(t, rec)
}

func TestLatestDeployment(t *testing.T) {
	bdb := setupTestDB(t)
	host := "myhost"

	err := bdb.RecordDeployment(host, DeploymentRecord{
		Timestamp:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Fingerprint: "old",
		Success:     true,
	})
	require.NoError(t, err)

	err = bdb.RecordDeployment(host, DeploymentRecord{
		Timestamp:   time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		Fingerprint: "new",
		Success:     false,
	})
	require.NoError(t, err)

	rec, err := bdb.LatestDeployment(host)
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, "new", rec.Fingerprint)
	assert.False(t, rec.Success)
}

// --- FlakeVersion tests ---

func TestStoreAndGetFlakeVersion(t *testing.T) {
	bdb := setupTestDB(t)

	meta := FlakeVersion{
		ResolvedUrl:  "github:nixos/nixpkgs",
		Revision:     "abc123",
		LastModified: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Fingerprint:  "fp123",
		StoredAt:     time.Now(),
	}

	err := bdb.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(flakeVersions))
		data, merr := msgpack.Marshal(meta)
		require.NoError(t, merr)
		return bucket.Put([]byte(meta.Fingerprint), data)
	})
	require.NoError(t, err)

	got, err := bdb.GetFlakeVersion("fp123")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "abc123", got.Revision)
}

func TestGetFlakeVersionNotFound(t *testing.T) {
	bdb := setupTestDB(t)

	got, err := bdb.GetFlakeVersion("nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

// --- HostLog tests ---

func TestReadHostLogsEmpty(t *testing.T) {
	bdb := setupTestDB(t)

	entries, err := bdb.ReadHostLogs("nonexistent")
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestWriteAndReadHostLogs(t *testing.T) {
	bdb := setupTestDB(t)

	err := bdb.WriteHostLog("host1", "entry 1")
	require.NoError(t, err)

	err = bdb.WriteHostLog("host1", "entry 2")
	require.NoError(t, err)

	entries, err := bdb.ReadHostLogs("host1")
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "entry 1", entries[0].Entry)
	assert.Equal(t, "entry 2", entries[1].Entry)
}

// Silence unused import warning for os.
var _ = os.DevNull
