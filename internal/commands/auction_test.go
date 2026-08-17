package commands

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"lazy-fanta/internal/costants"
	"lazy-fanta/internal/model"
)

func writeCommandSession(t *testing.T, session *model.Session) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(session.Filepath), 0755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(session.Filepath, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestSessionCommandsAreRegistered(t *testing.T) {
	want := map[string]bool{
		"new": true, "load": true, "update": true, "watch": true,
		"list": true, "save": true, "info": true, "clear": true, "end": true,
	}
	got := make(map[string]bool)
	for _, command := range SessionCmd.Commands() {
		got[command.Name()] = true
	}
	for name := range want {
		if !got[name] {
			t.Errorf("session command %q is not registered", name)
		}
	}
}

func TestWatchCommandRegistersTmuxFlag(t *testing.T) {
	flag := watchSessionCmd.Flags().Lookup("tmux")
	if flag == nil {
		t.Fatal("watch command does not register the tmux flag")
	}
	if got := flag.Value.Type(); got != "bool" {
		t.Fatalf("tmux flag type = %q, want bool", got)
	}
}

func TestTmuxSessionNameUsesSessionID(t *testing.T) {
	sess := &model.Session{ID: "ludo-5300"}
	if got := tmuxSessionName(sess); got != "lazy-fanta-ludo-5300" {
		t.Fatalf("tmux session name = %q, want %q", got, "lazy-fanta-ludo-5300")
	}
}

func TestTmuxWatchArgsBuildsChildCommand(t *testing.T) {
	got := tmuxWatchArgs("/tmp/lazy-fanta", "data/session/ludo-5300.json", "John", 500*time.Millisecond)
	want := []string{
		"/tmp/lazy-fanta", "auction", "watch", "data/session/ludo-5300.json",
		"--name", "John", "--interval", "500ms",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tmux watch args = %#v, want %#v", got, want)
	}

	got = tmuxWatchArgs("/tmp/lazy-fanta", "data/session/ludo-5300.json", "John", 2*time.Second)
	want = []string{
		"/tmp/lazy-fanta", "auction", "watch", "data/session/ludo-5300.json",
		"--name", "John",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default-interval tmux watch args = %#v, want %#v", got, want)
	}
}

func setupFakeTmux(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux")
	const script = `#!/bin/sh
printf '%s\n' "$*" >> "$TMUX_LOG"
if [ "$1" = "has-session" ] && [ "${TMUX_EXISTS:-0}" = "1" ]; then
    exit 0
fi
if [ "$1" = "has-session" ]; then
    exit 1
fi
exit 0
`
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "tmux.log")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMUX_LOG", logPath)
	return logPath
}

func readTmuxLog(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func TestLaunchTmuxWatchCreatesOnePanePerCoach(t *testing.T) {
	logPath := setupFakeTmux(t)
	t.Setenv("TMUX_EXISTS", "0")
	sess := &model.Session{
		ID:       "ludo-5300",
		Filepath: "data/session/ludo-5300.json",
		Coaches:  []model.Coach{{Name: "John"}, {Name: "Jane"}, {Name: "Luca"}},
	}

	if err := launchTmuxWatch(sess, sess.Filepath, 2*time.Second); err != nil {
		t.Fatal(err)
	}

	lines := readTmuxLog(t, logPath)
	if len(lines) != 6 {
		t.Fatalf("tmux calls = %d, want 6: %v", len(lines), lines)
	}
	name := tmuxSessionName(sess)
	if lines[0] != "has-session -t "+name {
		t.Fatalf("first tmux call = %q, want has-session", lines[0])
	}
	for i, coach := range sess.Coaches {
		line := lines[i+1]
		command := "auction watch " + sess.Filepath + " --name " + coach.Name
		if i == 0 {
			if !strings.HasPrefix(line, "new-session -d -s "+name+" -n watch -- ") {
				t.Fatalf("first coach call = %q, want new-session for %s", line, coach.Name)
			}
		} else if !strings.HasPrefix(line, "split-window -t "+name+":0 -- ") {
			t.Fatalf("coach call = %q, want split-window for %s", line, coach.Name)
		}
		if !strings.Contains(line, command) {
			t.Fatalf("coach call = %q, missing child command %q", line, command)
		}
	}
	if lines[4] != "select-layout -t "+name+":0 tiled" {
		t.Fatalf("layout call = %q, want tiled layout", lines[4])
	}
	if lines[5] != "attach-session -t "+name {
		t.Fatalf("last tmux call = %q, want attach-session", lines[5])
	}
	creations := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "new-session ") || strings.HasPrefix(line, "split-window ") {
			creations++
		}
		if strings.Contains(line, "--tmux") {
			t.Fatalf("child tmux call unexpectedly contains --tmux: %q", line)
		}
	}
	if creations != len(sess.Coaches) {
		t.Fatalf("coach pane creations = %d, want %d", creations, len(sess.Coaches))
	}
}

func TestLaunchTmuxWatchReusesExistingSession(t *testing.T) {
	logPath := setupFakeTmux(t)
	t.Setenv("TMUX_EXISTS", "1")
	sess := &model.Session{
		ID:       "ludo-5300",
		Filepath: "data/session/ludo-5300.json",
		Coaches:  []model.Coach{{Name: "John"}, {Name: "Jane"}},
	}

	if err := launchTmuxWatch(sess, sess.Filepath, 2*time.Second); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"has-session -t " + tmuxSessionName(sess),
		"attach-session -t " + tmuxSessionName(sess),
	}
	if got := readTmuxLog(t, logPath); !reflect.DeepEqual(got, want) {
		t.Fatalf("tmux calls = %v, want %v", got, want)
	}
}

func TestLaunchTmuxWatchRejectsEmptyRoster(t *testing.T) {
	logPath := setupFakeTmux(t)
	sess := &model.Session{ID: "ludo-5300", Filepath: "data/session/ludo-5300.json"}

	err := launchTmuxWatch(sess, sess.Filepath, 2*time.Second)
	if err == nil || !strings.Contains(err.Error(), "no coaches") {
		t.Fatalf("launchTmuxWatch error = %v, want no-coaches error", err)
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("tmux was invoked for empty roster; stat error = %v", err)
	}
}

func TestLaunchTmuxWatchReportsMissingTmux(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	sess := &model.Session{
		ID: "ludo-5300", Filepath: "data/session/ludo-5300.json",
		Coaches: []model.Coach{{Name: "John"}},
	}

	err := launchTmuxWatch(sess, sess.Filepath, 2*time.Second)
	if err == nil || !strings.Contains(err.Error(), "tmux") {
		t.Fatalf("launchTmuxWatch error = %v, want missing-tmux error", err)
	}
}

func TestResetFlagsRestoresREPLDefaults(t *testing.T) {
	coachName = "coach"
	byCoachName, playerName, playerRole = "by", "player", "role"
	cost = 10
	coachesNum = 3
	purchaseID = 4
	listRole = "fw"
	listCost = true
	watchInterval = time.Minute
	watchCoachName = "John"
	watchTmux = true

	ResetFlags()

	if coachName != "" || byCoachName != "" || playerName != "" || playerRole != "" ||
		cost != 0 || coachesNum != 0 || purchaseID != 0 || listRole != "" || listCost ||
		watchInterval != 2*time.Second {
		t.Fatalf("flags were not reset: coach=%q by=%q player=%q role=%q cost=%d coaches=%d id=%d listRole=%q listCost=%t interval=%s",
			coachName, byCoachName, playerName, playerRole, cost, coachesNum, purchaseID, listRole, listCost, watchInterval)
	}
	if watchCoachName != "" {
		t.Fatalf("watch coach name was not reset: %q", watchCoachName)
	}
	if watchTmux {
		t.Fatal("tmux flag was not reset")
	}
}

func TestReloadSessionReplacesOnlyAfterSuccessfulLoad(t *testing.T) {
	t.Chdir(t.TempDir())
	old := &model.Session{Filepath: "old.json", ID: "old"}
	fresh := &model.Session{Filepath: "fresh.json", ID: "fresh"}
	writeCommandSession(t, fresh)
	currentSession = old
	t.Cleanup(func() { currentSession = nil })

	if err := reloadSession(fresh.Filepath); err != nil {
		t.Fatal(err)
	}
	if currentSession == old || currentSession.ID != "fresh" {
		t.Fatalf("current session = %+v, want freshly loaded session", currentSession)
	}
}

func TestReloadSessionKeepsActiveSessionOnParseFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	path := "broken.json"
	if err := os.WriteFile(path, []byte("{"), 0644); err != nil {
		t.Fatal(err)
	}
	old := &model.Session{Filepath: path, ID: "old"}
	currentSession = old
	t.Cleanup(func() { currentSession = nil })

	if err := reloadSession(path); err == nil {
		t.Fatal("expected parse error")
	}
	if currentSession != old {
		t.Fatalf("current session replaced after failure: %+v", currentSession)
	}
}

func TestSelectWatchSessionUsesSelectorOrActiveSession(t *testing.T) {
	t.Chdir(t.TempDir())
	selected := &model.Session{Filepath: "data/session/selected.json", ID: "selected", AuctionName: "Final"}
	writeCommandSession(t, selected)
	active := &model.Session{Filepath: "active.json", ID: "active"}
	currentSession = active
	t.Cleanup(func() { currentSession = nil })

	got, err := selectWatchSession("selected")
	if err != nil {
		t.Fatal(err)
	}
	if got != currentSession || got.ID != "selected" {
		t.Fatalf("selected session = %+v, active = %+v", got, currentSession)
	}

	currentSession = active
	got, err = selectWatchSession("")
	if err != nil || got != active {
		t.Fatalf("active selection = %v, %v", got, err)
	}

	if _, err := selectWatchSession("missing"); err == nil {
		t.Fatal("expected unknown selector error")
	}
}

func TestSelectWatchSessionRequiresActiveSessionWithoutSelector(t *testing.T) {
	currentSession = nil
	t.Cleanup(func() { currentSession = nil })
	if _, err := selectWatchSession(""); err == nil {
		t.Fatal("expected missing active session error")
	}
}

func TestPrintWatchSnapshotClearsScreenBeforeRendering(t *testing.T) {
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	t.Cleanup(func() {
		os.Stdout = oldStdout
		_ = writer.Close()
		_ = reader.Close()
	})

	if err := printWatchSnapshot(&model.Session{AuctionName: "Final"}, ""); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	const clearScreen = "\033[2J\033[H"
	text := string(output)
	if !strings.HasPrefix(text, clearScreen) {
		t.Fatalf("watch output does not start with screen clear: %q", text)
	}
	if !strings.Contains(text, "Final") {
		t.Fatalf("watch output does not contain the rendered session: %s", text)
	}
}

func TestPrintWatchSnapshotFiltersByCoach(t *testing.T) {
	sess := &model.Session{
		AuctionName: "Final",
		Coaches: []model.Coach{
			{
				Name:    "John",
				Players: []model.Player{{Name: "Messi", Role: costants.Forward, Cost: 100}},
			},
			{
				Name:    "Jane",
				Players: []model.Player{{Name: "Ronaldo", Role: costants.Forward, Cost: 90}},
			},
		},
	}

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	if err := printWatchSnapshot(sess, "John"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = oldStdout
	output, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}

	text := string(output)
	if !strings.HasPrefix(text, "\033[2J\033[H") {
		t.Fatalf("watch output does not start with screen clear: %q", text)
	}
	for _, want := range []string{"John", "Messi"} {
		if !strings.Contains(text, want) {
			t.Errorf("watch output missing %q: %s", want, text)
		}
	}
	for _, unwanted := range []string{"Jane", "Ronaldo"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("watch output unexpectedly contains %q: %s", unwanted, text)
		}
	}
}

func TestFileSignatureChangesWhenFileChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	first, err := sessionFileSignature(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("new content"), 0644); err != nil {
		t.Fatal(err)
	}
	second, err := sessionFileSignature(path)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("file signature did not change: %+v", first)
	}
}
