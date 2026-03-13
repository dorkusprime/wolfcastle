package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/logging"
	"github.com/spf13/cobra"
)

var followCmd = &cobra.Command{
	Use:   "follow",
	Short: "Tail the latest iteration's model output in real time",
	RunE: func(cmd *cobra.Command, args []string) error {
		logDir := filepath.Join(wolfcastleDir, "logs")

		for {
			latestPath, err := logging.LatestLogFile(logDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Waiting for logs...\n")
				time.Sleep(2 * time.Second)
				continue
			}

			if err := tailFile(latestPath); err != nil {
				return err
			}
			// File ended, look for a new one
			time.Sleep(1 * time.Second)
		}
	},
}

func tailFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}

		// Extract relevant info
		typ, _ := record["type"].(string)
		switch typ {
		case "stage_start":
			stage, _ := record["stage"].(string)
			node, _ := record["node"].(string)
			task, _ := record["task"].(string)
			fmt.Printf("[%s] Starting %s/%s\n", stage, node, task)
		case "stage_complete":
			stage, _ := record["stage"].(string)
			fmt.Printf("[%s] Complete\n", stage)
		case "stage_error":
			stage, _ := record["stage"].(string)
			errMsg, _ := record["error"].(string)
			fmt.Printf("[%s] Error: %s\n", stage, errMsg)
		case "assistant":
			// Model output
			if content, ok := record["text"].(string); ok {
				fmt.Print(content)
			}
		}
	}
	return scanner.Err()
}

func init() {
	rootCmd.AddCommand(followCmd)
}
