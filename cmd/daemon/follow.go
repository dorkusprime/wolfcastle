package daemon

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/logrender"
	"github.com/dorkusprime/wolfcastle/internal/signals"
	"github.com/spf13/cobra"
)

// modeFlag implements pflag.Value for output mode flags with last-wins
// semantics. Each flag instance writes its designated mode to a shared
// *outputMode when parsed. Because pflag processes flags left-to-right,
// the last mode flag on the command line wins naturally.
type modeFlag struct {
	target *outputMode
	value  outputMode
}

func (f *modeFlag) String() string   { return "false" }
func (f *modeFlag) Set(string) error { *f.target = f.value; return nil }
func (f *modeFlag) Type() string     { return "bool" }
func (f *modeFlag) IsBoolFlag() bool { return true }

// registerModeFlags adds the mutually exclusive output mode flags to cmd,
// wiring them to write into *mode on parse. NoOptDefVal makes each flag
// behave like a boolean: --thoughts (no argument) is equivalent to
// --thoughts=true.
func registerModeFlags(cmd *cobra.Command, mode *outputMode) {
	cmd.Flags().VarP(&modeFlag{target: mode, value: modeThoughts}, "thoughts", "t", "Raw agent output only")
	cmd.Flags().Lookup("thoughts").NoOptDefVal = "true"

	cmd.Flags().VarP(&modeFlag{target: mode, value: modeInterleaved}, "interleaved", "i", "Stage headers and agent output with timestamps")
	cmd.Flags().Lookup("interleaved").NoOptDefVal = "true"

	cmd.Flags().Var(&modeFlag{target: mode, value: modeJSON}, "json", "Raw NDJSON output, no formatting")
	cmd.Flags().Lookup("json").NoOptDefVal = "true"
}

func newLogCmd(app *cmdutil.App) *cobra.Command {
	mode := modeSummary

	cmd := &cobra.Command{
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

			// Resolve session. Exit 1 when the log directory is missing
			// (project not initialized) or contains no sessions.
			session, err := logrender.ResolveSession(logDir, sessionIdx)
			if err != nil {
				return fmt.Errorf("no log files found")
			}

			// Implicit follow when daemon is running and session 0.
			if !follow && sessionIdx == 0 && app.Daemon.IsAlive() {
				follow = true
			}

			// Downgrade explicit --follow when daemon is stopped.
			// The spec says --follow is a no-op when the daemon is not running;
			// without this guard the command hangs waiting for log lines that
			// will never arrive.
			if follow && !app.Daemon.IsAlive() {
				follow = false
			}

			ctx, stop := signal.NotifyContext(context.Background(), signals.Shutdown...)
			defer stop()

			return runLog(ctx, logDir, session, mode, follow, app.Daemon.IsAlive)
		},
	}

	registerModeFlags(cmd, &mode)

	return cmd
}

type outputMode int

const (
	modeSummary outputMode = iota
	modeThoughts
	modeInterleaved
	modeJSON
)

// runLog dispatches to the appropriate renderer based on mode and follow flag.
func runLog(ctx context.Context, logDir string, session logrender.Session, mode outputMode, follow bool, aliveCheck func() bool) error {
	w := os.Stdout

	if mode == modeJSON {
		return runJSONMode(ctx, logDir, session, follow, aliveCheck)
	}

	if follow {
		reader := logrender.NewFollowReader(logDir, 200*time.Millisecond)
		reader.SetAliveCheck(aliveCheck)
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
func runJSONMode(ctx context.Context, logDir string, session logrender.Session, follow bool, aliveCheck func() bool) error {
	if follow {
		return followJSON(ctx, logDir, aliveCheck)
	}
	return replayJSON(session)
}

// replayJSON streams raw lines from the session's files to stdout,
// decompressing .gz files on the fly.
func replayJSON(session logrender.Session) error {
	for _, path := range session.Files {
		if err := replayJSONFile(path); err != nil {
			continue
		}
	}
	return nil
}

func replayJSONFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var reader io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gz.Close()
		reader = gz
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	for scanner.Scan() {
		os.Stdout.Write(scanner.Bytes())
		os.Stdout.Write([]byte{'\n'})
	}
	return scanner.Err()
}

// followJSON tails the log directory and writes raw lines to stdout.
func followJSON(ctx context.Context, logDir string, aliveCheck func() bool) error {
	reader := logrender.NewFollowReader(logDir, 200*time.Millisecond)
	reader.SetAliveCheck(aliveCheck)
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
