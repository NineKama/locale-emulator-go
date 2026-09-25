// Package library stores successful launches independently from the app binary.
package library

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Entry struct {
	Path         string `json:"path"`
	Name         string `json:"name"`
	Architecture string `json:"architecture"`
	LastPlayed   string `json:"lastPlayed"`
	LaunchCount  uint64 `json:"launchCount"`
	Available    bool   `json:"available"`
}
type document struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}
type Store struct {
	path string
	mu   sync.Mutex
}

func New(path string) *Store { return &Store{path: path} }

// Deduplicate ordinary Windows paths without treating letter case as identity.
func key(path string) string { return strings.ToLower(filepath.Clean(path)) }

func (s *Store) read() ([]Entry, error) {
	f, e := os.Open(s.path)
	if os.IsNotExist(e) {
		return []Entry{}, nil
	}
	if e != nil {
		return nil, e
	}
	defer f.Close()
	var doc document
	decoder := json.NewDecoder(io.LimitReader(f, 10<<20))
	if e = decoder.Decode(&doc); e != nil {
		return nil, fmt.Errorf("read library: %w", e)
	}
	var extra any
	if e = decoder.Decode(&extra); e != io.EOF {
		return nil, fmt.Errorf("invalid trailing library data")
	}
	if doc.Version != 1 {
		return nil, fmt.Errorf("unsupported library version: %d", doc.Version)
	}
	for _, entry := range doc.Entries {
		if !filepath.IsAbs(entry.Path) {
			return nil, fmt.Errorf("invalid library path")
		}
	}
	if doc.Entries == nil {
		return []Entry{}, nil
	}
	return doc.Entries, nil
}
func (s *Store) write(entries []Entry) error {
	if e := os.MkdirAll(filepath.Dir(s.path), 0700); e != nil {
		return e
	}
	data, e := json.MarshalIndent(document{1, entries}, "", "  ")
	if e != nil {
		return e
	}
	// Same-directory replacement preserves the previous file if writing fails.
	f, e := os.CreateTemp(filepath.Dir(s.path), "library-*.tmp")
	if e != nil {
		return e
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, e = f.Write(data); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(temp, s.path)
}
func (s *Store) List() ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, e := s.read()
	if e != nil {
		return nil, e
	}
	for i := range entries {
		info, e := os.Stat(entries[i].Path)
		entries[i].Available = e == nil && !info.IsDir()
	}
	sort.SliceStable(entries, func(i, j int) bool { return playedAfter(entries[i].LastPlayed, entries[j].LastPlayed) })
	return entries, nil
}

// Record is called only after the engine has successfully launched the process.
func (s *Store) Record(path, arch string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("library requires an absolute executable path")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, e := s.read()
	if e != nil {
		return e
	}
	entry := Entry{Path: filepath.Clean(path), Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Architecture: arch, LastPlayed: time.Now().UTC().Format(time.RFC3339Nano), LaunchCount: 1}
	kept := make([]Entry, 0, len(entries)+1)
	for _, old := range entries {
		if key(old.Path) == key(path) {
			entry.LaunchCount += old.LaunchCount
		} else {
			kept = append(kept, old)
		}
	}
	kept = append(kept, entry)
	return s.write(kept)
}

// Remove only edits library.json; it never deletes the executable or its data.
func (s *Store) Remove(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, e := s.read()
	if e != nil {
		return e
	}
	kept := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if key(entry.Path) != key(path) {
			kept = append(kept, entry)
		}
	}
	return s.write(kept)
}

func playedAfter(a, b string) bool {
	left, _ := time.Parse(time.RFC3339Nano, a)
	right, _ := time.Parse(time.RFC3339Nano, b)
	return left.After(right)
}
