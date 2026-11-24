package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/sreenivas-sadhu-prabhakara/fleet-doc-expiry-vault/internal/fleet"
)

// Store keeps the fleet in memory and on disk in one plain JSON file.
//
// There is no database and no server: the file sits wherever the owner points
// -data at, so it can live on a pen drive or inside a folder that is already
// being backed up. Writes go to a temporary file in the same directory and are
// then renamed over the old one, so a crash or a full disk leaves the previous
// good file intact rather than a half-written one.
type Store struct {
	mu    sync.RWMutex
	path  string
	fleet *fleet.Fleet
}

// OpenStore loads the data file, or starts an empty fleet if there is not one
// yet. It does not create the file until something is saved.
func OpenStore(path string) (*Store, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("data file %s: %w", path, err)
	}
	s := &Store{path: absolute, fleet: fleet.New()}

	raw, err := os.ReadFile(absolute)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not read the data file %s: %w", absolute, err)
	}
	if len(raw) == 0 {
		return s, nil
	}
	loaded := fleet.New()
	if err := json.Unmarshal(raw, loaded); err != nil {
		return nil, fmt.Errorf("the data file %s is not valid JSON: %w", absolute, err)
	}
	loaded.Normalise()
	s.fleet = loaded
	return s, nil
}

// Path is where the data file lives, shown in the UI so the owner always knows.
func (s *Store) Path() string { return s.path }

// Read runs fn against the fleet under a read lock. fn must not keep the
// pointer or mutate anything behind it.
func (s *Store) Read(fn func(f *fleet.Fleet)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fn(s.fleet)
}

// Write runs fn against the fleet under a write lock and saves the file if fn
// succeeds. Nothing is saved if fn fails; the domain validates every edit
// before it touches a record, so a rejected edit changes nothing in memory
// either.
func (s *Store) Write(fn func(f *fleet.Fleet) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fn(s.fleet); err != nil {
		return err
	}
	return s.save()
}

// ReplaceRecords swaps every vehicle, driver and document for the ones given,
// used by a CSV import that replaces the whole book rather than merging into
// it. The owner's own policy — which documents block, which validity periods
// have been overridden — is kept, because a CSV of records says nothing about
// settings.
func (s *Store) ReplaceRecords(records *fleet.Fleet) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	records.Blocking = s.fleet.Blocking
	records.Validity = s.fleet.Validity
	records.Normalise()
	s.fleet = records
	return s.save()
}

// save writes the data file atomically: a temporary file in the same directory,
// flushed to disk, then renamed over the target.
func (s *Store) save() error {
	raw, err := json.MarshalIndent(s.fleet, "", "  ")
	if err != nil {
		return fmt.Errorf("could not encode the records: %w", err)
	}
	raw = append(raw, '\n')

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("could not create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(s.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("could not write beside %s: %w", s.path, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // harmless once the rename has happened

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("could not set permissions on %s: %w", tmpName, err)
	}
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return fmt.Errorf("could not write %s: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("could not flush %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("could not close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("could not replace %s: %w", s.path, err)
	}
	return nil
}
