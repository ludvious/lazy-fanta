package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lazy-fanta/internal/costants"
	"lazy-fanta/internal/model"
)

func writeSessionFile(t *testing.T, s *model.Session) {
	t.Helper()
	if err := os.MkdirAll(costants.SessionDir, 0755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.Filepath, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestSaveSessionRejectsNilSession(t *testing.T) {
	if err := SaveSession(nil); err == nil {
		t.Fatal("expected nil session error")
	}
}

func TestSaveSessionReplacesCompleteContentAndCleansTempFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	session := &model.Session{
		Filepath: "sessions/auction.json",
		ID:       "auction-0001",
		Coaches:  []model.Coach{{Name: "Old"}},
	}
	if err := SaveSession(session); err != nil {
		t.Fatal(err)
	}

	session.Coaches = []model.Coach{{Name: "New", Players: []model.Player{{Name: "Player"}}}}
	if err := SaveSession(session); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadSession(session.Filepath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Coaches) != 1 || loaded.Coaches[0].Name != "New" || len(loaded.Coaches[0].Players) != 1 {
		t.Fatalf("loaded replacement = %+v", loaded.Coaches)
	}
	entries, err := os.ReadDir("sessions")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".auction.json.tmp-") {
			t.Fatalf("temporary file left behind: %s", entry.Name())
		}
	}
}

func TestSaveSessionCleansTempFileWhenRenameFails(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "auction.json")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := SaveSession(&model.Session{Filepath: target, ID: "auction-0001"}); err == nil {
		t.Fatal("expected rename error when destination is a directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".auction.json.tmp-") {
			t.Fatalf("temporary file left behind after failed save: %s", entry.Name())
		}
	}
}

func TestResolveSessionByIDAndName(t *testing.T) {
	t.Chdir(t.TempDir())
	old := &model.Session{
		Filepath:    "data/session/old-0001.json",
		ID:          "old-0001",
		AuctionName: "Serie A",
		UpdatedAt:   "2026-01-01 00:00:00",
		Status:      "Active",
		Purchases:   []model.Purchase{},
		Coaches:     []model.Coach{},
	}
	newer := &model.Session{
		Filepath:    "data/session/old-0002.json",
		ID:          "old-0002",
		AuctionName: "Serie A",
		UpdatedAt:   "2026-02-01 00:00:00",
		Status:      "Active",
		Purchases:   []model.Purchase{},
		Coaches:     []model.Coach{},
	}
	writeSessionFile(t, old)
	writeSessionFile(t, newer)

	byID, err := ResolveSession("old-0001")
	if err != nil || byID.ID != "old-0001" {
		t.Fatalf("resolve by ID: %v, %v", byID, err)
	}
	byName, err := ResolveSession("serie a") // case-insensitive, newest wins
	if err != nil || byName.ID != "old-0002" {
		t.Fatalf("resolve by name: %v, %v", byName, err)
	}
	if _, err := ResolveSession("nope"); err == nil {
		t.Fatal("expected error for unknown session")
	}
}
