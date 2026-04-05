package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// DaemonActivity is a snapshot of the daemon's current activity, written
// to disk so that `wolfcastle status` can detect stalls and report what
// the daemon is working on without filesystem stat tricks.
type DaemonActivity struct {
	LastActivityAt time.Time `json:"last_activity_at"`
	Iteration      int64     `json:"iteration"`
	CurrentNode    string    `json:"current_node,omitempty"`
	CurrentTask    string    `json:"current_task,omitempty"`
}

// writeActivity writes a DaemonActivity snapshot to daemon-activity.json.
func (d *Daemon) writeActivity(node, task string) {
	activity := DaemonActivity{
		LastActivityAt: d.Clock.Now(),
		Iteration:      d.iteration.Load(),
		CurrentNode:    node,
		CurrentTask:    task,
	}
	data, err := json.MarshalIndent(activity, "", "  ")
	if err != nil {
		return
	}
	if err := state.AtomicWriteFile(activityPath(d.WolfcastleDir), data); err != nil {
		_ = d.Logger.Log(map[string]any{
			"type":  "activity_write_error",
			"error": err.Error(),
		})
	}
}

// removeActivityFile removes the daemon-activity.json file on shutdown
// so stale activity doesn't persist.
func (d *Daemon) removeActivityFile() {
	_ = os.Remove(activityPath(d.WolfcastleDir))
}

// activityPath returns the path to the daemon activity snapshot file.
func activityPath(wolfcastleDir string) string {
	return filepath.Join(wolfcastleDir, "system", "daemon-activity.json")
}

// LoadDaemonActivity reads the daemon activity snapshot from disk.
// Returns nil if the file doesn't exist or can't be parsed.
func LoadDaemonActivity(wolfcastleDir string) *DaemonActivity {
	data, err := os.ReadFile(activityPath(wolfcastleDir))
	if err != nil {
		return nil
	}
	var activity DaemonActivity
	if err := json.Unmarshal(data, &activity); err != nil {
		return nil
	}
	return &activity
}
