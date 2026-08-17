package storage

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lazy-fanta/internal/costants"
	"lazy-fanta/internal/model"
)

// SaveSession writes the session to its JSON file.
func SaveSession(session *model.Session) error {
	if session == nil {
		return fmt.Errorf("cannot save nil auction session")
	}
	dir := filepath.Dir(session.Filepath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for session data: %w", err)
	}
	session.UpdatedAt = time.Now().Format(costants.SessionTimeFormat)
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize auction session data: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(session.Filepath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary auction session file: %w", err)
	}
	tmpPath := tmp.Name()
	removeTemp := true
	defer func() {
		_ = tmp.Close()
		if removeTemp {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0644); err != nil {
		return fmt.Errorf("failed to set temporary auction session file mode: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("failed to write auction session data: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("failed to sync auction session data: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close auction session data: %w", err)
	}
	if err := os.Rename(tmpPath, session.Filepath); err != nil {
		return fmt.Errorf("failed to replace auction session data: %w", err)
	}
	removeTemp = false
	return nil
}

// LoadSession loads a session from a JSON file.
func LoadSession(path string) (*model.Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read auction session file: %w", err)
	}
	var session model.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse auction session file: %w", err)
	}
	return &session, nil
}

// ClearSession removes the session file.
func ClearSession(session *model.Session) error {
	if err := os.Remove(session.Filepath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to clear auction session data: %w", err)
	}
	fmt.Printf("Auction session data cleared from %s\n", session.Filepath)
	return nil
}

// ListSessions returns all sessions stored in the session directory.
func ListSessions() ([]model.Session, error) {
	files, err := os.ReadDir(costants.SessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list auction sessions: %w", err)
	}
	var sessions []model.Session
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		sess, err := LoadSession(filepath.Join(costants.SessionDir, f.Name()))
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *sess)
	}
	return sessions, nil
}

// ResolveSession finds a session by file path, session ID, or auction name.
// When a name matches multiple sessions, the most recently updated is returned.
func ResolveSession(key string) (*model.Session, error) {
	// 1) exact file path
	if s, err := LoadSession(key); err == nil {
		return s, nil
	}
	// 2) ID or auction name among stored sessions
	sessions, err := ListSessions()
	if err != nil {
		return nil, err
	}
	var match *model.Session
	for i := range sessions {
		s := &sessions[i]
		if strings.EqualFold(s.ID, key) || strings.EqualFold(s.AuctionName, key) {
			if match == nil || s.UpdatedAt > match.UpdatedAt {
				match = s
			}
		}
	}
	if match == nil {
		return nil, fmt.Errorf("No auction session found for %q — use 'auction list' to see available auction sessions", key)
	}
	return match, nil
}

// ExportCSV writes the session's purchases to data/export/<id>.csv.
func ExportCSV(s *model.Session) error {
	dir := filepath.Join(costants.DataDir, "export")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}
	path := filepath.Join(dir, s.ID+".csv")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"purchase_id", "player", "role", "coach", "cost", "timestamp"}); err != nil {
		return err
	}
	for _, p := range s.Purchases {
		row := []string{
			strconv.Itoa(p.ID),
			p.PlayerName,
			string(p.PlayerRole),
			p.CoachName,
			strconv.Itoa(p.Cost),
			p.Timestamp.Format(costants.SessionTimeFormat),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	fmt.Printf("  Exported %d purchases to %s\n", len(s.Purchases), path)
	return nil
}
