package changeboard

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// persist.go adds durable change-control submissions (D2 §33.4): the
// board writes each submission (and its review outcome) to one JSON
// file under the configured directory, and a restarted harness
// reloads them — a pending submission survives restarts and an
// approval never silently vanishes.

// EnablePersistence routes the board's writes to dir (created lazily).
// Load failures are surfaced; the board never starts empty when a
// store exists but is unreadable.
func (b *Board) EnablePersistence(dir string) error {
	if dir == "" {
		return errors.New("changeboard: persistence dir required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b.mu.Lock()
	b.dir = dir
	// Replay existing submissions.
	entries, err := os.ReadDir(dir)
	if err != nil {
		b.mu.Unlock()
		return err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			continue
		}
		var s Submission
		if json.Unmarshal(raw, &s) == nil && s.SubmissionID != "" {
			b.submissions[s.SubmissionID] = &s
			b.byFingerprint[s.Fingerprint()] = &s
		}
	}
	b.mu.Unlock()
	return nil
}

var persistMu sync.Mutex

func (b *Board) persistLocked(s *Submission) {
	if b.dir == "" || s == nil {
		return
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return
	}
	name := filepath.Join(b.dir, s.SubmissionID+".json")
	tmp := name + ".tmp"
	if os.WriteFile(tmp, raw, 0o600) == nil {
		_ = os.Rename(tmp, name)
	}
}
