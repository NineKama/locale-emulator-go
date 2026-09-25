package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLibrarySurvivesReopenAndRemoveKeepsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings", "library.json")
	game := filepath.Join(dir, "\u65e5\u672c\u8a9e Game.exe")
	if e := os.WriteFile(game, []byte("fixture"), 0600); e != nil {
		t.Fatal(e)
	}
	store := New(path)
	entries, e := store.List()
	if e != nil || len(entries) != 0 {
		t.Fatalf("initial library: %v %v", entries, e)
	}
	if e = store.Record(game, "386"); e != nil {
		t.Fatal(e)
	}
	reopened := New(path)
	entries, e = reopened.List()
	if e != nil || len(entries) != 1 {
		t.Fatalf("reopen: %v %v", entries, e)
	}
	if entries[0].Name != "\u65e5\u672c\u8a9e Game" || !entries[0].Available || entries[0].LaunchCount != 1 || entries[0].Architecture != "386" || entries[0].LastPlayed == "" {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
	if e = reopened.Record(strings.ToUpper(game), "386"); e != nil {
		t.Fatal(e)
	}
	entries, e = reopened.List()
	if e != nil || len(entries) != 1 || entries[0].LaunchCount != 2 {
		t.Fatalf("duplicate path: %+v %v", entries, e)
	}
	if e = reopened.Remove(game); e != nil {
		t.Fatal(e)
	}
	entries, e = New(path).List()
	if e != nil || len(entries) != 0 {
		t.Fatalf("remove: %+v %v", entries, e)
	}
	if _, e = os.Stat(game); e != nil {
		t.Fatalf("removing history deleted the game: %v", e)
	}
}

func TestRecentOrderAndMissingFile(t *testing.T) {
	dir := t.TempDir()
	store := New(filepath.Join(dir, "library.json"))
	a := filepath.Join(dir, "a.exe")
	b := filepath.Join(dir, "b.exe")
	for _, p := range []string{a, b, a} {
		if e := store.Record(p, "amd64"); e != nil {
			t.Fatal(e)
		}
	}
	entries, e := store.List()
	if e != nil {
		t.Fatal(e)
	}
	if len(entries) != 2 || entries[0].Path != a || entries[0].LaunchCount != 2 || entries[0].Available {
		t.Fatalf("unexpected recents: %+v", entries)
	}
}

func TestCorruptLibraryIsNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")
	for _, data := range []string{"broken json", `{"version":99,"entries":[]}`, `{"version":1,"entries":[]} trailing`} {
		if e := os.WriteFile(path, []byte(data), 0600); e != nil {
			t.Fatal(e)
		}
		store := New(path)
		if _, e := store.List(); e == nil {
			t.Fatal("accepted invalid library")
		}
		if e := store.Record(filepath.Join(dir, "game.exe"), "386"); e == nil {
			t.Fatal("overwrote corrupt history")
		}
		if e := store.Remove(filepath.Join(dir, "game.exe")); e == nil {
			t.Fatal("overwrote corrupt history on removal")
		}
		preserved, e := os.ReadFile(path)
		if e != nil || string(preserved) != data {
			t.Fatalf("data loss: %q %v", preserved, e)
		}
	}
}
