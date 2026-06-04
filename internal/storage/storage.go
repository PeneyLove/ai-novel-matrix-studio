// Package storage provides read/write access to the .novelAgent/ directory.
//
// All operations are sandboxed — paths containing ".." are rejected.
// No external database is required; everything lives on the local filesystem.
package storage

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Dir is the standard directory name.
const Dir = ".novelAgent"

// --- Path helpers with sandbox enforcement ---

// resolve ensures the given path is within the agent root and contains no ".." escapes.
func resolve(root, sub string) (string, error) {
	if strings.Contains(sub, "..") {
		return "", fmt.Errorf("storage: path traversal rejected: %q", sub)
	}
	clean := filepath.Clean(filepath.Join(root, sub))
	// Double-check after cleaning
	rel, err := filepath.Rel(root, clean)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("storage: path escapes root: %q", sub)
	}
	return clean, nil
}

// ensureDir creates parent directories for a file path.
func ensureDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

// --- Types ---

// TraceRecord is a single copyright-trace entry written as one JSONL line.
type TraceRecord struct {
	TaskID     string `json:"task_id"`
	Stage      string `json:"stage"`
	PromptHash string `json:"prompt_hash"` // SHA256 of the prompt
	DraftHash  string `json:"draft_hash"`  // SHA256 of the AI draft
	FinalHash  string `json:"final_hash"`  // SHA256 after human edits
	Timestamp  string `json:"timestamp"`   // RFC3339
}

// --- Config ---

// ReadConfig reads .novelAgent/config.yaml.
func ReadConfig(root string) (map[string]any, error) {
	path, err := resolve(root, "config.yaml")
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("storage: read config: %w", err)
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("storage: parse config: %w", err)
	}
	return cfg, nil
}

// WriteConfig writes .novelAgent/config.yaml.
func WriteConfig(root string, cfg map[string]any) error {
	path, err := resolve(root, "config.yaml")
	if err != nil {
		return err
	}
	if err := ensureDir(path); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("storage: marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0o600) // owner-only for API keys
}

// --- Skills ---

// ReadSkill reads .novelAgent/skills/<name>/skill.yaml.
func ReadSkill(root, name string) (map[string]any, error) {
	sub := filepath.Join("skills", name, "skill.yaml")
	path, err := resolve(root, sub)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("storage: read skill %q: %w", name, err)
	}
	var skill map[string]any
	if err := yaml.Unmarshal(data, &skill); err != nil {
		return nil, fmt.Errorf("storage: parse skill %q: %w", name, err)
	}
	return skill, nil
}

// WriteSkill writes .novelAgent/skills/<name>/skill.yaml.
func WriteSkill(root, name string, skill map[string]any) error {
	sub := filepath.Join("skills", name, "skill.yaml")
	path, err := resolve(root, sub)
	if err != nil {
		return err
	}
	if err := ensureDir(path); err != nil {
		return err
	}
	data, err := yaml.Marshal(skill)
	if err != nil {
		return fmt.Errorf("storage: marshal skill: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// ListSkills returns names of installed skills.
func ListSkills(root string) ([]string, error) {
	skillsDir, err := resolve(root, "skills")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(skillsDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("storage: list skills: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// --- Corpus ---

// WriteCorpus writes a corpus file to .novelAgent/corpus/<category>.txt (appends).
func WriteCorpus(root, category, content string) error {
	sub := filepath.Join("corpus", category+".txt")
	path, err := resolve(root, sub)
	if err != nil {
		return err
	}
	if err := ensureDir(path); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("storage: write corpus: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(content + "\n"); err != nil {
		return fmt.Errorf("storage: write corpus: %w", err)
	}
	return nil
}

// ReadCorpus reads all lines from .novelAgent/corpus/<category>.txt.
func ReadCorpus(root, category string) ([]string, error) {
	sub := filepath.Join("corpus", category+".txt")
	path, err := resolve(root, sub)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("storage: read corpus: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var result []string
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			result = append(result, l)
		}
	}
	return result, nil
}

// --- Outputs ---

// WriteOutput writes generated content to .novelAgent/outputs/<task_id>/<filename>.
func WriteOutput(root, taskID, filename, content string) error {
	sub := filepath.Join("outputs", taskID, filename)
	path, err := resolve(root, sub)
	if err != nil {
		return err
	}
	if err := ensureDir(path); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// ReadOutput reads generated content from .novelAgent/outputs/<task_id>/<filename>.
func ReadOutput(root, taskID, filename string) (string, error) {
	sub := filepath.Join("outputs", taskID, filename)
	path, err := resolve(root, sub)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("storage: read output: %w", err)
	}
	return string(data), nil
}

// TaskExists checks if outputs/<taskID>/ directory already exists (idempotency guard).
func TaskExists(root, taskID string) bool {
	sub := filepath.Join("outputs", taskID)
	path, err := resolve(root, sub)
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// --- Traces ---

// AppendTrace appends a JSONL line to .novelAgent/traces/<task_id>.jsonl.
func AppendTrace(root string, tr TraceRecord) error {
	sub := filepath.Join("traces", tr.TaskID+".jsonl")
	path, err := resolve(root, sub)
	if err != nil {
		return err
	}
	if err := ensureDir(path); err != nil {
		return err
	}
	line, err := json.Marshal(tr)
	if err != nil {
		return fmt.Errorf("storage: marshal trace: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("storage: append trace: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("storage: write trace: %w", err)
	}
	if _, err := f.WriteString("\n"); err != nil {
		return fmt.Errorf("storage: write trace newline: %w", err)
	}
	return nil
}

// HashSHA256 returns the hex-encoded SHA256 hash of the input string.
func HashSHA256(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}
