package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/logging"
	"github.com/spf13/cobra"
)

var followCmd = &cobra.Command{
	Use:   "follow",
	Short: "Tail the latest iteration's model output in real time",
	Long: `Streams the daemon's model output in real time, similar to 'tail -f'.

Automatically follows new log files as the daemon starts new iterations.
Press Ctrl+C to stop following.

Examples:
  wolfcastle follow`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logDir := filepath.Join(wolfcastleDir, "logs")
		var currentFile string

		for {
			latestPath, err := logging.LatestLogFile(logDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Waiting for logs...\n")
				time.Sleep(2 * time.Second)
				continue
			}

			if latestPath != currentFile {
				if currentFile != "" {
					fmt.Printf("\n--- New iteration: %s ---\n\n", filepath.Base(latestPath))
				} else {
					fmt.Printf("--- Following: %s ---\n\n", filepath.Base(latestPath))
				}
				currentFile = latestPath
			}

			if err := tailFileStreaming(currentFile); err != nil {
				return err
			}
			// After EOF, poll for new data or new files
			time.Sleep(500 * time.Millisecond)
		}
	},
}

func tailFileStreaming(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

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
		line := scanner.Bytes()
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			continue
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
				// Print without extra newline if content already has one
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

	// Update the offset to the current file size. We re-stat because
	// the file may have grown since our initial check. Using f.Seek(0,1)
	// would be unreliable because bufio.Scanner reads ahead into an
	// internal buffer.
	if endInfo, err := os.Stat(path); err == nil {
		setOffset(path, endInfo.Size())
	}

	return scanner.Err()
}

// Simple offset tracking for tail -f behavior
var fileOffsets = make(map[string]int64)

func getOffset(path string) int64 {
	return fileOffsets[path]
}

func setOffset(path string, offset int64) {
	fileOffsets[path] = offset
}

func init() {
	rootCmd.AddCommand(followCmd)
}
