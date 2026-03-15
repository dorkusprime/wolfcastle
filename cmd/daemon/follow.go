package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/logging"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

func newFollowCmd(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "follow",
		Short: "Watch the daemon work",
		Long: `Streams model output in real time. Follows new iterations automatically.
Ctrl+C to disengage.

Examples:
  wolfcastle follow
  wolfcastle follow --lines 100`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logDir := filepath.Join(app.WolfcastleDir, "logs")
			lines, _ := cmd.Flags().GetInt("lines")
			levelFilter, _ := cmd.Flags().GetString("level")
			minLevel := logging.LevelInfo
			if levelFilter != "" {
				if parsed, ok := logging.ParseLevel(levelFilter); ok {
					minLevel = parsed
				} else {
					return fmt.Errorf("unknown log level %q (use debug, info, warn, error)", levelFilter)
				}
			}
			var currentFile string
			historicalShown := false
			waitMessageShown := false

			for {
				latestPath, err := logging.LatestLogFile(logDir)
				if err != nil {
					if !waitMessageShown {
						output.PrintHuman("Waiting for the daemon to produce output...")
						waitMessageShown = true
					}
					time.Sleep(2 * time.Second)
					continue
				}

				if latestPath != currentFile {
					if currentFile != "" {
						fmt.Printf("\n--- New iteration: %s ---\n\n", filepath.Base(latestPath))
					} else {
						fmt.Printf("--- Following: %s ---\n\n", filepath.Base(latestPath))
						// Show historical lines from the initial log file
						if lines > 0 && !historicalShown {
							showHistoricalLines(latestPath, lines, minLevel)
							historicalShown = true
						}
					}
					currentFile = latestPath
				}

				if err := tailFileStreaming(currentFile, minLevel); err != nil {
					return err
				}
				// After EOF, poll for new data or new files
				time.Sleep(500 * time.Millisecond)
			}
		},
	}
}

// showHistoricalLines reads the last N NDJSON lines from a log file and
// formats them for display, giving the user context before live streaming begins.
func showHistoricalLines(path string, n int, minLevel logging.Level) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	// Read all lines, then take the last n
	var allLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	start := 0
	if len(allLines) > n {
		start = len(allLines) - n
	}

	for _, line := range allLines[start:] {
		formatAndPrintLogLine(line, minLevel)
	}

	// Set the offset so tailFileStreaming doesn't re-print what we just showed
	if info, err := os.Stat(path); err == nil {
		setOffset(path, info.Size())
	}
}

func tailFileStreaming(path string, minLevel logging.Level) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	// Seek to where we last left off by checking current file size
	// We use a static map to track offsets across calls
	offset := getOffset(path)
	if offset > 0 {
		if _, err := f.Seek(offset, 0); err != nil {
			return err
		}
	}

	// Check if the file has grown
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() <= offset {
		return nil // No new data
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		formatAndPrintLogLine(scanner.Text(), minLevel)
	}

	// Update the offset to the current file size. We re-stat because
	// the file may have grown since our initial check. Using f.Seek(0,1)
	// would be unreliable because bufio.Scanner reads ahead into an
	// internal buffer.
	if endInfo, err := os.Stat(path); err == nil {
		setOffset(path, endInfo.Size())
	}

	return scanner.Err()
}

// formatAndPrintLogLine parses a single NDJSON line and prints it in
// human-readable form. Lines below minLevel are silently dropped.
func formatAndPrintLogLine(line string, minLevel logging.Level) {
	var record map[string]any
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		return
	}

	// Filter by log level
	if lvlStr, ok := record["level"].(string); ok {
		if lvl, ok := logging.ParseLevel(lvlStr); ok && lvl < minLevel {
			return
		}
	}

	typ, _ := record["type"].(string)
	switch typ {
	case "stage_start":
		stage, _ := record["stage"].(string)
		node, _ := record["node"].(string)
		task, _ := record["task"].(string)
		if node != "" {
			fmt.Printf("[%s] Starting %s/%s\n", stage, node, task)
		} else {
			fmt.Printf("[%s] Starting\n", stage)
		}
	case "stage_complete":
		stage, _ := record["stage"].(string)
		exitCode, _ := record["exit_code"].(float64)
		fmt.Printf("[%s] Complete (exit=%d)\n", stage, int(exitCode))
	case "stage_error":
		stage, _ := record["stage"].(string)
		errMsg, _ := record["error"].(string)
		fmt.Printf("[%s] Error: %s\n", stage, errMsg)
	case "assistant":
		if content, ok := record["text"].(string); ok {
			if strings.HasSuffix(content, "\n") {
				fmt.Print(content)
			} else {
				fmt.Println(content)
			}
		}
	case "failure_increment":
		task, _ := record["task"].(string)
		count, _ := record["count"].(float64)
		fmt.Printf("[failure] Task %s failure count: %d\n", task, int(count))
	case "auto_block":
		task, _ := record["task"].(string)
		reason, _ := record["reason"].(string)
		fmt.Printf("[blocked] Task %s auto-blocked: %s\n", task, reason)
	}
}

// Simple offset tracking for tail -f behavior
var fileOffsets = make(map[string]int64)

func getOffset(path string) int64 {
	return fileOffsets[path]
}

func setOffset(path string, offset int64) {
	fileOffsets[path] = offset
}
