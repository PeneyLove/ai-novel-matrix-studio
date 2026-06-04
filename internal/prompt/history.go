// Package prompt manages skill prompt templates lifecycle:
//   - Versioned snapshots (every mutation saved to history)
//   - CLI editing (novel-agent prompt edit)
//   - Conversational auto-optimization (model refines its own prompts)
package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// HistoryDir is the subdirectory under .novelAgent/ for prompt version snapshots.
const HistoryDir = "prompts_history"

// Snapshot saves the current prompt content as a versioned backup before mutation.
// Returns the snapshot path or empty string if snapshotting is suppressed.
func Snapshot(root, genre, stage, currentContent string) (string, error) {
	if currentContent == "" {
		return "", nil
	}
	dir := filepath.Join(root, HistoryDir, genre, stage)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("prompt: create history dir: %w", err)
	}

	// Find the next version number
	entries, _ := os.ReadDir(dir)
	nextVer := len(entries) + 1

	filename := fmt.Sprintf("v%03d-%s.yaml", nextVer, time.Now().UTC().Format("20060102-150405"))
	path := filepath.Join(dir, filename)

	if err := os.WriteFile(path, []byte(currentContent), 0o644); err != nil {
		return "", fmt.Errorf("prompt: write snapshot: %w", err)
	}
	return path, nil
}

// ListHistory returns all snapshots for a given skill/stage, sorted by filename.
func ListHistory(root, genre, stage string) ([]string, error) {
	dir := filepath.Join(root, HistoryDir, genre, stage)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("prompt: list history: %w", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files, nil
}

// ReadHistory reads a specific snapshot file.
func ReadHistory(root, genre, stage, filename string) (string, error) {
	path := filepath.Join(root, HistoryDir, genre, stage, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("prompt: read history: %w", err)
	}
	return string(data), nil
}

// Diff returns a simple line-by-line diff summary between the current prompt
// and the most recent snapshot. Returns empty string if no history or identical.
func Diff(root, genre, stage, currentContent string) string {
	files, err := ListHistory(root, genre, stage)
	if err != nil || len(files) == 0 {
		return "(no previous version to diff against)"
	}
	last := files[len(files)-1]
	prev, err := os.ReadFile(last)
	if err != nil {
		return fmt.Sprintf("(error reading previous: %v)", err)
	}
	if string(prev) == currentContent {
		return "(unchanged)"
	}

	prevLines := strings.Split(string(prev), "\n")
	currLines := strings.Split(currentContent, "\n")

	var diff strings.Builder
	diff.WriteString(fmt.Sprintf("Diff vs %s:\n", filepath.Base(last)))

	max := len(prevLines)
	if len(currLines) > max {
		max = len(currLines)
	}
	for i := 0; i < max; i++ {
		if i < len(prevLines) && i < len(currLines) {
			if prevLines[i] != currLines[i] {
				diff.WriteString(fmt.Sprintf("  L%d: -%s\n", i+1, prevLines[i]))
				diff.WriteString(fmt.Sprintf("  L%d: +%s\n", i+1, currLines[i]))
			}
		} else if i >= len(prevLines) {
			diff.WriteString(fmt.Sprintf("  L%d: +%s\n", i+1, currLines[i]))
		} else {
			diff.WriteString(fmt.Sprintf("  L%d: -%s\n", i+1, prevLines[i]))
		}
	}
	return diff.String()
}

// Rollback restores the prompt to a specific snapshot version.
func Rollback(root, genre, stage, snapshotFile string) (string, error) {
	content, err := ReadHistory(root, genre, stage, snapshotFile)
	if err != nil {
		return "", err
	}
	// The actual write-back is done by the caller who holds the skill YAML reference
	return content, nil
}
