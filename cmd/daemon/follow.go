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

func newLogCmd(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "log",
		Short: "Read the daemon's logs",
		Long: `Shows daemon log output. Without --follow, prints recent log lines
and exits (like reading a file). With --follow, streams output in
real time and tracks new iterations automatically.

Examples:
  wolfcastle log
  wolfcastle log --lines 50
  wolfcastle log --follow
  wolfcastle log -f -l debug`,
		Aliases: []string{"follow"},
		RunE: func(cmd *cobra.Command, args []string) error {
			logDir := filepath.Join(app.WolfcastleDir, "logs")
			lines, _ := cmd.Flags().GetInt("lines")
			follow, _ := cmd.Flags().GetBool("follow")
			levelFilter, _ := cmd.Flags().GetString("level")
			minLevel := logging.LevelInfo
			if levelFilter != "" {
				if parsed, ok := logging.ParseLevel(levelFilter); ok {
					minLevel = parsed
				} else {
					return fmt.Errorf("unknown log level %q (use debug, info, warn, error)", levelFilter)
				}
			}

			if !follow {
				return showRecentLogs(logDir, lines, minLevel)
			}
			return followLogs(logDir, lines, minLevel)
		},
	}
}

// showRecentLogs prints the last N lines from the most recent log file and exits.
func showRecentLogs(logDir string, lines int, minLevel logging.Level) error {
	latestPath, err := logging.LatestLogFile(logDir)
	if err != nil {
		output.PrintHuman("No logs yet.")
		return nil
	}

	fmt.Printf("--- %s ---\n\n", filepath.Base(latestPath))
	showHistoricalLines(latestPath, lines, minLevel)
	return nil
}

// followLogs streams log output in real time, following new iterations.
func followLogs(logDir string, lines int, minLevel logging.Level) error {
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
		time.Sleep(500 * time.Millisecond)
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

	offset := getOffset(path)
	if offset > 0 {
		if _, err := f.Seek(offset, 0); err != nil {
			return err
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() <= offset {
		return nil
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		formatAndPrintLogLine(scanner.Text(), minLevel)
	}

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

	if lvlStr, ok := record["level"].(string); ok {
		if lvl, ok := logging.ParseLevel(lvlStr); ok && lvl < minLevel {
			return
		}
	}

	typ, _ := record["type"].(string)
	trace, _ := record["trace"].(string)
	prefix := ""
	if trace != "" {
		prefix = "[" + trace + "] "
	}

	switch typ {
	case "stage_start":
		stage, _ := record["stage"].(string)
		node, _ := record["node"].(string)
		task, _ := record["task"].(string)
		if node != "" {
			fmt.Printf("%s[%s] Starting %s/%s\n", prefix, stage, node, task)
		} else {
			fmt.Printf("%s[%s] Starting\n", prefix, stage)
		}
	case "stage_complete":
		stage, _ := record["stage"].(string)
		exitCode, _ := record["exit_code"].(float64)
		outputLen, _ := record["output_len"].(float64)
		fmt.Printf("%s[%s] Complete (exit=%d, %d bytes)\n", prefix, stage, int(exitCode), int(outputLen))
	case "stage_error":
		stage, _ := record["stage"].(string)
		errMsg, _ := record["error"].(string)
		fmt.Printf("%s[%s] Error: %s\n", prefix, stage, errMsg)
	case "assistant":
		if text, ok := record["text"].(string); ok {
			formatted := formatAssistantText(text)
			if formatted != "" {
				fmt.Println(formatted)
			}
		}
	case "failure_increment":
		task, _ := record["task"].(string)
		count, _ := record["count"].(float64)
		fmt.Printf("%s[failure] Task %s failure count: %d\n", prefix, task, int(count))
	case "auto_block":
		task, _ := record["task"].(string)
		reason, _ := record["reason"].(string)
		fmt.Printf("%s[blocked] Task %s auto-blocked: %s\n", prefix, task, reason)
	case "terminal_marker":
		marker, _ := record["marker"].(string)
		fmt.Printf("%s%s\n", prefix, marker)
	case "deliverable_missing":
		task, _ := record["task"].(string)
		fmt.Printf("%s[deliverable] Task %s: missing files\n", prefix, task)
	case "deliverable_unchanged":
		task, _ := record["task"].(string)
		fmt.Printf("%s[deliverable] Task %s: files unchanged since claim\n", prefix, task)
	case "retry":
		stage, _ := record["stage"].(string)
		attempt, _ := record["attempt"].(float64)
		errMsg, _ := record["error"].(string)
		fmt.Printf("%s[retry] %s attempt %d: %s\n", prefix, stage, int(attempt), errMsg)
	case "retry_exhausted":
		stage, _ := record["stage"].(string)
		attempts, _ := record["attempts"].(float64)
		fmt.Printf("%s[retry] %s exhausted after %d attempts\n", prefix, stage, int(attempts))
	case "daemon_start":
		scope, _ := record["scope"].(string)
		fmt.Printf("%sDaemon started (scope=%s)\n", prefix, scope)
	case "daemon_stop":
		reason, _ := record["reason"].(string)
		fmt.Printf("%sDaemon stopped (%s)\n", prefix, reason)
	case "propagate_error":
		errMsg, _ := record["error"].(string)
		fmt.Printf("%s[propagate] Error: %s\n", prefix, errMsg)
	default:
		if msg, ok := record["message"].(string); ok && msg != "" {
			fmt.Printf("%s[%s] %s\n", prefix, typ, msg)
		} else if typ != "" {
			fmt.Printf("%s[%s]\n", prefix, typ)
		}
	}
}

// formatAssistantText extracts human-readable content from Claude Code's
// streaming JSON output.
func formatAssistantText(raw string) string {
	var envelope struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
		Message struct {
			Content []struct {
				Type  string `json:"type"`
				Text  string `json:"text"`
				Name  string `json:"name"`
				Input any    `json:"input"`
			} `json:"content"`
		} `json:"message"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		if len(raw) > 200 {
			return raw[:200] + "..."
		}
		return raw
	}

	switch envelope.Type {
	case "assistant":
		var parts []string
		for _, c := range envelope.Message.Content {
			switch c.Type {
			case "text":
				if c.Text != "" {
					parts = append(parts, c.Text)
				}
			case "tool_use":
				if c.Name != "" {
					if inputMap, ok := c.Input.(map[string]any); ok {
						if desc, ok := inputMap["description"].(string); ok {
							parts = append(parts, fmt.Sprintf("  → %s: %s", c.Name, desc))
						} else if cmd, ok := inputMap["command"].(string); ok {
							if len(cmd) > 80 {
								cmd = cmd[:80] + "..."
							}
							parts = append(parts, fmt.Sprintf("  → %s: %s", c.Name, cmd))
						} else {
							parts = append(parts, fmt.Sprintf("  → %s", c.Name))
						}
					} else {
						parts = append(parts, fmt.Sprintf("  → %s", c.Name))
					}
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
	case "result":
		if envelope.Result != "" {
			return "[result] " + envelope.Result
		}
	case "system":
		if envelope.Subtype == "init" {
			return "[session started]"
		}
	}

	return ""
}

// Simple offset tracking for tail -f behavior
var fileOffsets = make(map[string]int64)

func getOffset(path string) int64 {
	return fileOffsets[path]
}

func setOffset(path string, offset int64) {
	fileOffsets[path] = offset
}
