package badgerstore

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// BadgerStore wraps BadgerDB for high-write operational data
type BadgerStore struct {
	db *badger.DB
}

// Config holds BadgerDB configuration
type Config struct {
	Path string // Directory path for BadgerDB files
}

// Open creates a new BadgerDB instance
func Open(config *Config) (*BadgerStore, error) {
	if config == nil || config.Path == "" {
		return nil, fmt.Errorf("badger config path is required")
	}

	opts := badger.DefaultOptions(config.Path)
	opts.Logger = nil // Disable BadgerDB's internal logging (use slog instead)

	// Reduce memtable size for memory-constrained environments
	// Default is 64MB, we use 16MB (still plenty for high write throughput)
	// Tradeoff: More frequent flushes to disk, but 75% less memory usage
	opts.MemTableSize = 16 << 20 // 16MB per memtable (vs 64MB default)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger: %w", err)
	}

	store := &BadgerStore{db: db}

	// Start garbage collection goroutine
	go store.runGC()

	slog.Info("BadgerDB opened successfully", "path", config.Path)
	return store, nil
}

// Close closes the BadgerDB instance
func (b *BadgerStore) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}

// runGC runs periodic garbage collection
func (b *BadgerStore) runGC() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
	again:
		err := b.db.RunValueLogGC(0.5) // GC if 50% can be reclaimed
		if err == nil {
			goto again // Run again if successful
		}
		// Stop when no more GC needed (badger.ErrNoRewrite)
	}
}

// Get retrieves a value by key
func (b *BadgerStore) Get(key string) ([]byte, error) {
	var value []byte
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		value, err = item.ValueCopy(nil)
		return err
	})
	if err == badger.ErrKeyNotFound {
		return nil, nil // Return nil for not found (not an error)
	}
	if err != nil {
		return nil, err
	}
	return value, nil
}

// Set stores a key-value pair with optional TTL
func (b *BadgerStore) Set(key string, value []byte, ttl time.Duration) error {
	return b.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry([]byte(key), value)
		if ttl > 0 {
			entry = entry.WithTTL(ttl)
		}
		return txn.SetEntry(entry)
	})
}

// Delete removes a key
func (b *BadgerStore) Delete(key string) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// DeletePrefix deletes all keys with the given prefix
func (b *BadgerStore) DeletePrefix(prefix string) error {
	return b.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefix)
		opts.PrefetchValues = false // Only need keys

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := it.Item().KeyCopy(nil)
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})
}

// ListKeysWithPrefix returns all keys matching the prefix
func (b *BadgerStore) ListKeysWithPrefix(prefix string) ([]string, error) {
	var keys []string
	err := b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefix)
		opts.PrefetchValues = false // Only need keys

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().KeyCopy(nil))
			keys = append(keys, key)
		}
		return nil
	})
	return keys, err
}

// BatchSet sets multiple key-value pairs in a single transaction
func (b *BadgerStore) BatchSet(items map[string][]byte, ttl time.Duration) error {
	return b.db.Update(func(txn *badger.Txn) error {
		for key, value := range items {
			entry := badger.NewEntry([]byte(key), value)
			if ttl > 0 {
				entry = entry.WithTTL(ttl)
			}
			if err := txn.SetEntry(entry); err != nil {
				return err
			}
		}
		return nil
	})
}
