package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/dorkusprime/wolfcastle/internal/output"
)

// overlapMatch is a single detected overlap between a new project and
// an existing one from another engineer's namespace.
type overlapMatch struct {
	Engineer    string  `json:"engineer"`
	Project     string  `json:"project"`
	Score       float64 `json:"score"`
	SharedTerms []string `json:"shared_terms"`
}

// checkOverlap scans other engineers' project names and descriptions for
// potential scope overlap with the newly created project. Uses bigram
// Jaccard similarity — no model invocation required (ADR-041).
// Purely informational — failures are silently ignored (ADR-027).
func checkOverlap(projectName, description string) {
	if cfg == nil || resolver == nil || !cfg.OverlapAdvisory.Enabled {
		return
	}

	threshold := cfg.OverlapAdvisory.Threshold

	// Tokenize the new project
	newText := projectName + " " + description
	newBigrams := bigrams(tokenize(newText))
	newTerms := significantTerms(tokenize(newText))

	if len(newBigrams) == 0 {
		return
	}

	// Collect and compare against other engineers' projects
	projectsRoot := filepath.Join(wolfcastleDir, "projects")
	entries, err := os.ReadDir(projectsRoot)
	if err != nil {
		return
	}

	var matches []overlapMatch
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == resolver.Namespace {
			continue
		}
		nsDir := filepath.Join(projectsRoot, entry.Name())
		compareNamespace(nsDir, entry.Name(), newBigrams, newTerms, threshold, &matches)
	}

	if len(matches) == 0 {
		return
	}

	output.PrintHuman("")
	output.PrintHuman("Overlap Advisory:")
	for _, m := range matches {
		output.PrintHuman("  [%s] %s (score: %.2f, shared: %s)",
			m.Engineer, m.Project, m.Score, strings.Join(m.SharedTerms, ", "))
	}
}

// compareNamespace walks an engineer's namespace and checks each .md file
// for overlap with the new project's bigram set.
func compareNamespace(dir, engineer string, newBigrams map[string]bool, newTerms map[string]bool, threshold float64, matches *[]overlapMatch) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			compareNamespace(fullPath, engineer, newBigrams, newTerms, threshold, matches)
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}

		existingBigrams := bigrams(tokenize(content))
		if len(existingBigrams) == 0 {
			continue
		}

		score := jaccardSimilarity(newBigrams, existingBigrams)
		if score < threshold {
			continue
		}

		// Find shared significant terms for human-readable context
		existingTerms := significantTerms(tokenize(content))
		var shared []string
		for term := range newTerms {
			if existingTerms[term] {
				shared = append(shared, term)
			}
		}

		projectName := strings.TrimSuffix(entry.Name(), ".md")
		*matches = append(*matches, overlapMatch{
			Engineer:    engineer,
			Project:     projectName,
			Score:       score,
			SharedTerms: shared,
		})
	}
}

// tokenize splits text into lowercased words, stripping punctuation.
func tokenize(text string) []string {
	var tokens []string
	for _, word := range strings.Fields(text) {
		cleaned := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return unicode.ToLower(r)
			}
			return -1
		}, word)
		if len(cleaned) > 1 {
			tokens = append(tokens, cleaned)
		}
	}
	return tokens
}

// bigrams produces the set of character bigrams from a token list.
func bigrams(tokens []string) map[string]bool {
	set := make(map[string]bool)
	for _, token := range tokens {
		if isStopWord(token) {
			continue
		}
		for i := 0; i < len(token)-1; i++ {
			set[token[i:i+2]] = true
		}
	}
	return set
}

// jaccardSimilarity computes |A ∩ B| / |A ∪ B|.
func jaccardSimilarity(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	intersection := 0
	for k := range a {
		if b[k] {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// significantTerms returns the set of non-stop-word terms (for shared-term reporting).
func significantTerms(tokens []string) map[string]bool {
	terms := make(map[string]bool)
	for _, t := range tokens {
		if !isStopWord(t) && len(t) > 2 {
			terms[t] = true
		}
	}
	return terms
}

// isStopWord returns true for common English words that cause false positives.
func isStopWord(w string) bool {
	switch w {
	case "the", "and", "for", "are", "but", "not", "you", "all", "can",
		"had", "her", "was", "one", "our", "out", "has", "his", "how",
		"its", "may", "new", "now", "old", "see", "way", "who", "did",
		"get", "let", "say", "she", "too", "use", "with", "this", "that",
		"from", "have", "been", "will", "more", "when", "some", "what",
		"into", "them", "than", "each", "make", "like", "just", "over",
		"such", "take", "also", "back", "after", "year", "only", "come",
		"could", "would", "about", "which", "their", "there", "other",
		"should", "project", "finding", "audit", "details", "description":
		return true
	}
	return false
}
