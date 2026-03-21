package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/logrender"
	"github.com/dorkusprime/wolfcastle/internal/signals"
	"github.com/spf13/cobra"
)

func newLogCmd(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "log",
		Short: "Display daemon activity",
		Long: `Shows daemon activity reconstructed from NDJSON log files.

Default output is a summary: one line per completed stage showing what
the daemon did (or is doing). Flags control verbosity.

When the daemon is running, output streams in real time (implicit --follow).
When it is stopped, the last session's output is displayed and the command exits.

Output modes (mutually exclusive, last flag wins):
  (default)       Summary: one line per stage with duration
  --thoughts/-t   Raw agent output only
  --interleaved/-i Stage headers and agent output with timestamps
  --json          Raw NDJSON, no formatting

Examples:
  wolfcastle log
  wolfcastle log --thoughts
  wolfcastle log -i -f
  wolfcastle log --session 1
  wolfcastle log --json | jq '.type'`,
		Aliases: []string{"follow"},
		RunE: func(cmd *cobra.Command, args []string) error {
			logDir := app.Daemon.LogDir()
			sessionIdx, _ := cmd.Flags().GetInt("session")
			follow, _ := cmd.Flags().GetBool("follow")
			thoughts, _ := cmd.Flags().GetBool("thoughts")
			interleaved, _ := cmd.Flags().GetBool("interleaved")
			jsonOut, _ := cmd.Flags().GetBool("json")

			// Resolve session.
			session, err := logrender.ResolveSession(logDir, sessionIdx)
			if err != nil {
				fmt.Fprintln(os.Stderr, "No log files found.")
				return nil
			}

			// Implicit follow when daemon is running and session 0.
			if !follow && sessionIdx == 0 && app.Daemon.IsAlive() {
				follow = true
			}

			// Determine output mode: last flag wins among mutually exclusive options.
			// We check in order so that if multiple are set, the "last" semantically
			// (json > interleaved > thoughts) takes precedence.
			mode := modeSummary
			if thoughts {
				mode = modeThoughts
			}
			if interleaved {
				mode = modeInterleaved
			}
			if jsonOut {
				mode = modeJSON
			}

			ctx, stop := signal.NotifyContext(context.Background(), signals.Shutdown...)
			defer stop()

			return runLog(ctx, logDir, session, mode, follow)
		},
	}
}

type outputMode int

const (
	modeSummary outputMode = iota
	modeThoughts
	modeInterleaved
	modeJSON
)

// runLog dispatches to the appropriate renderer based on mode and follow flag.
func runLog(ctx context.Context, logDir string, session logrender.Session, mode outputMode, follow bool) error {
	w := os.Stdout

	if mode == modeJSON {
		return runJSONMode(ctx, logDir, session, follow)
	}

	if follow {
		reader := logrender.NewFollowReader(logDir, 200*time.Millisecond)
		records := reader.Records(ctx)

		switch mode {
		case modeSummary:
			sr := logrender.NewSummaryRenderer(w)
			sr.Follow(ctx, records)
		case modeThoughts:
			tr := logrender.NewThoughtsRenderer(w)
			tr.Render(ctx, records)
		case modeInterleaved:
			ir := logrender.NewInterleavedRenderer(w)
			ir.Render(ctx, records)
		}
		return nil
	}

	// Replay mode: read from the session's files.
	reader := logrender.NewReplayReader(session.Files)
	records := reader.Records()

	switch mode {
	case modeSummary:
		sr := logrender.NewSummaryRenderer(w)
		sr.Replay(records)
	case modeThoughts:
		tr := logrender.NewThoughtsRenderer(w)
		tr.Render(context.Background(), records)
	case modeInterleaved:
		ir := logrender.NewInterleavedRenderer(w)
		ir.Render(context.Background(), records)
	}
	return nil
}

// runJSONMode dumps raw NDJSON lines with no parsing or formatting.
func runJSONMode(ctx context.Context, logDir string, session logrender.Session, follow bool) error {
	if follow {
		return followJSON(ctx, logDir)
	}
	return replayJSON(session)
}

// replayJSON writes raw lines from the session's files to stdout.
func replayJSON(session logrender.Session) error {
	for _, path := range session.Files {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		os.Stdout.Write(data)
	}
	return nil
}

// followJSON tails the log directory and writes raw lines to stdout.
func followJSON(ctx context.Context, logDir string) error {
	reader := logrender.NewFollowReader(logDir, 200*time.Millisecond)
	records := reader.Records(ctx)
	for rec := range records {
		raw, err := json.Marshal(rec.Raw)
		if err != nil {
			continue
		}
		os.Stdout.Write(raw)
		os.Stdout.Write([]byte{'\n'})
	}
	return nil
}
