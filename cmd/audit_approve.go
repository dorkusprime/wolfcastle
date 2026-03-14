package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/project"
	"github.com/dorkusprime/wolfcastle/internal/review"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

var auditApproveCmd = &cobra.Command{
	Use:   "approve <finding-id | --all>",
	Short: "Approve an audit finding and create a project for it",
	Long: `Approves a pending audit finding, creating a leaf project in the work tree.
Use --all to approve every remaining pending finding in the batch.

When all findings have been decided, the batch is archived to history
and the pending file is removed.

Examples:
  wolfcastle audit approve finding-1
  wolfcastle audit approve --all`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireResolver(); err != nil {
			return err
		}

		allFlag, _ := cmd.Flags().GetBool("all")
		if !allFlag && len(args) == 0 {
			return fmt.Errorf("provide a finding ID or use --all")
		}

		batchPath := filepath.Join(wolfcastleDir, "audit-review.json")
		batch, err := review.LoadBatch(batchPath)
		if err != nil {
			return err
		}
		if batch == nil {
			return fmt.Errorf("no pending review batch — run 'wolfcastle audit run' first")
		}

		idx, err := resolver.LoadRootIndex()
		if err != nil {
			return fmt.Errorf("loading root index: %w", err)
		}

		now := time.Now().UTC()
		var approved []review.Decision

		for i := range batch.Findings {
			f := &batch.Findings[i]
			if f.Status != review.FindingPending {
				continue
			}
			if !allFlag && (len(args) == 0 || args[0] != f.ID) {
				continue
			}

			// Create project from finding
			slug := tree.ToSlug(f.Title)
			if err := tree.ValidateSlug(slug); err != nil {
				if !allFlag {
					return fmt.Errorf("cannot approve %s: title %q produces invalid slug: %v", f.ID, f.Title, err)
				}
				output.PrintHuman("  Skipped %s (invalid slug from title): %s", f.ID, f.Title)
				continue
			}

			// If the project already exists, mark approved without creating
			if _, exists := idx.Nodes[slug]; exists {
				f.Status = review.FindingApproved
				f.DecidedAt = &now
				f.CreatedNode = slug
				approved = append(approved, review.Decision{
					FindingID:   f.ID,
					Title:       f.Title,
					Action:      string(review.FindingApproved),
					Timestamp:   now,
					CreatedNode: slug,
				})
				output.PrintHuman("  Approved: %s (project %s already exists)", f.ID, slug)
				continue
			}

			ns, addr, createErr := project.CreateProject(idx, "", slug, f.Title, state.NodeLeaf)
			if createErr != nil {
				output.PrintHuman("  Error creating %s: %v", f.Title, createErr)
				continue
			}

			addrParsed, parseErr := tree.ParseAddress(addr)
			if parseErr != nil {
				output.PrintHuman("  Error parsing address %s: %v", addr, parseErr)
				continue
			}
			nodeDir := filepath.Join(resolver.ProjectsDir(), filepath.Join(addrParsed.Parts...))
			if mkdirErr := os.MkdirAll(nodeDir, 0755); mkdirErr != nil {
				// Roll back index entry since we can't persist the node
				delete(idx.Nodes, addr)
				output.PrintHuman("  Error creating directory for %s: %v", f.Title, mkdirErr)
				continue
			}
			if saveErr := state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns); saveErr != nil {
				delete(idx.Nodes, addr)
				output.PrintHuman("  Error saving state for %s: %v", f.Title, saveErr)
				continue
			}

			// Write description with finding detail
			descContent := fmt.Sprintf("# %s\n\nAudit finding from batch %s.\n", f.Title, batch.ID)
			if f.Description != "" {
				descContent += "\n## Details\n\n" + f.Description + "\n"
			}
			descPath := filepath.Join(resolver.ProjectsDir(), slug+".md")
			_ = os.WriteFile(descPath, []byte(descContent), 0644)

			f.Status = review.FindingApproved
			f.DecidedAt = &now
			f.CreatedNode = addr

			approved = append(approved, review.Decision{
				FindingID:   f.ID,
				Title:       f.Title,
				Action:      string(review.FindingApproved),
				Timestamp:   now,
				CreatedNode: addr,
			})

			output.PrintHuman("  Approved: %s → %s", f.ID, addr)
		}

		if len(approved) == 0 {
			if allFlag {
				return fmt.Errorf("no pending findings to approve")
			}
			return fmt.Errorf("finding %q not found or already decided", args[0])
		}

		// Save batch first — if this fails, no index changes are persisted,
		// so the batch remains the source of truth for what's been decided.
		if err := review.SaveBatch(batchPath, batch); err != nil {
			return err
		}

		// Save updated root index (new projects created above)
		if err := state.SaveRootIndex(resolver.RootIndexPath(), idx); err != nil {
			return err
		}

		// Check if batch is fully decided
		if err := finalizeBatchIfComplete(batch, batchPath); err != nil {
			return err
		}

		if jsonOutput {
			output.Print(output.Ok("audit_approve", map[string]any{
				"approved": len(approved),
				"decisions": approved,
			}))
		} else {
			output.PrintHuman("\nApproved %d finding(s).", len(approved))
		}

		return nil
	},
}

// finalizeBatchIfComplete archives the batch to history and removes the
// pending file when all findings have been decided.
func finalizeBatchIfComplete(batch *review.Batch, batchPath string) error {
	for _, f := range batch.Findings {
		if f.Status == review.FindingPending {
			return nil // Still has undecided findings
		}
	}

	// Mark batch as completed
	batch.Status = review.BatchCompleted

	// Build history entry
	var decisions []review.Decision
	for _, f := range batch.Findings {
		d := review.Decision{
			FindingID: f.ID,
			Title:     f.Title,
			Action:    string(f.Status),
		}
		if f.DecidedAt != nil {
			d.Timestamp = *f.DecidedAt
		}
		if f.CreatedNode != "" {
			d.CreatedNode = f.CreatedNode
		}
		decisions = append(decisions, d)
	}

	entry := review.HistoryEntry{
		BatchID:     batch.ID,
		CompletedAt: time.Now().UTC(),
		Scopes:      batch.Scopes,
		Decisions:   decisions,
	}

	// Load, append, enforce retention, and save history
	historyPath := filepath.Join(filepath.Dir(batchPath), "audit-review-history.json")
	history, err := review.LoadHistory(historyPath)
	if err != nil {
		return err
	}
	history.Entries = append(history.Entries, entry)
	review.EnforceRetention(history, 100, 90)
	if err := review.SaveHistory(historyPath, history); err != nil {
		return err
	}

	// Remove the pending batch file
	if err := review.RemoveBatch(batchPath); err != nil {
		return err
	}

	output.PrintHuman("Batch %s complete — archived to history.", batch.ID)
	return nil
}

func init() {
	auditApproveCmd.Flags().Bool("all", false, "Approve all pending findings")
	auditCmd.AddCommand(auditApproveCmd)
}
