// Package project manages the per-novel project directory structure.
//
//	.novelAgent/projects/<name>/
//	  world.yaml        — worldbuilding, power system, factions
//	  outline.yaml      — outline versions + hook ledger
//	  characters/       — one YAML per character (profile + arc)
//	  relationships.yaml — relationship graph across all characters
//	  output/           — generated chapter .txt files (ready to upload)
//	  traces/           — copyright retention hashes
package project

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Dir returns the project directory path under .novelAgent.
func Dir(root, name string) string {
	return filepath.Join(root, "projects", name)
}

// Init creates the full project directory skeleton.
func Init(root, name string) error {
	dir := Dir(root, name)
	dirs := []string{
		dir,
		filepath.Join(dir, "characters"),
		filepath.Join(dir, "output"),
		filepath.Join(dir, "traces"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("project: create dir %s: %w", d, err)
		}
	}

	// Write default world.yaml
	world := map[string]any{
		"name":        name,
		"genre":       "",
		"sub_genre":   "",
		"power_system": "",
		"factions":    []string{},
		"rules":       []string{},
	}
	if err := writeYAML(filepath.Join(dir, "world.yaml"), world); err != nil {
		return err
	}

	// Write default outline.yaml
	outline := map[string]any{
		"version":     0,
		"content":     "",
		"finalized":   false,
		"volumes":     []any{},
		"hooks":       []any{},
		"chapter_count": 0,
	}
	if err := writeYAML(filepath.Join(dir, "outline.yaml"), outline); err != nil {
		return err
	}

	// Write default relationships.yaml
	rels := map[string]any{
		"pairs": []any{},
		"deactivated_characters": []any{},
	}
	if err := writeYAML(filepath.Join(dir, "relationships.yaml"), rels); err != nil {
		return err
	}

	return nil
}

// ---- Character operations ----

// CharacterProfile is a single character's full profile.
type CharacterProfile struct {
	ID           string `yaml:"id" json:"id"`
	Name         string `yaml:"name" json:"name"`
	Role         string `yaml:"role"`   // 主角 / 配角 / 反派 / 路人
	Status       string `yaml:"status"` // active / deactivated
	Personality  string `yaml:"personality"`
	Background   string `yaml:"background"`
	Motivation   string `yaml:"motivation"`
	Arc          string `yaml:"arc"`
	Abilities    string `yaml:"abilities"`
	Appearance   string `yaml:"appearance"`
	Notes        string `yaml:"notes"`

	// Evolution records the character's growth across chapters.
	// Each step captures a state change (level-up, personality shift, death, etc.)
	Evolution []EvolutionStep `yaml:"evolution"`

	// EvolvePrompt is the AI-refined system prompt for this specific character.
	// Updated via /char <name> evolve or automatically after chapter generation.
	EvolvePrompt string `yaml:"evolve_prompt"`

	CreatedAt    string `yaml:"created_at"`
	UpdatedAt    string `yaml:"updated_at"`
	DeactivatedAt string `yaml:"deactivated_at,omitempty"`
	DeactivatedBy string `yaml:"deactivated_by,omitempty"`
}

// WriteCharacter saves a character profile to characters/<id>.yaml.
func WriteCharacter(root, projectName string, ch CharacterProfile) error {
	ch.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if ch.CreatedAt == "" {
		ch.CreatedAt = ch.UpdatedAt
	}
	if ch.ID == "" {
		ch.ID = sanitizeFilename(ch.Name)
	}
	path := filepath.Join(Dir(root, projectName), "characters", ch.ID+".yaml")
	return writeYAML(path, ch)
}

// ReadCharacter loads a character by ID.
func ReadCharacter(root, projectName, id string) (*CharacterProfile, error) {
	path := filepath.Join(Dir(root, projectName), "characters", id+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("project: read character %s: %w", id, err)
	}
	var ch CharacterProfile
	if err := yaml.Unmarshal(data, &ch); err != nil {
		return nil, fmt.Errorf("project: parse character %s: %w", id, err)
	}
	return &ch, nil
}

// ListCharacters returns all active characters in the project.
func ListCharacters(root, projectName string) ([]CharacterProfile, error) {
	dir := filepath.Join(Dir(root, projectName), "characters")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var chars []CharacterProfile
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		ch, err := ReadCharacter(root, projectName, e.Name()[:len(e.Name())-5])
		if err != nil {
			continue
		}
		chars = append(chars, *ch)
	}
	return chars, nil
}

// DeactivateCharacter marks a character as deactivated and moves their
// memory to the relationships.yaml deactivated section.
// Returns an audit record describing what was affected.
func DeactivateCharacter(root, projectName, id, reason string) (string, error) {
	ch, err := ReadCharacter(root, projectName, id)
	if err != nil {
		return "", err
	}
	if ch.Status == "deactivated" {
		return "角色已处于下线状态", nil
	}

	// Mark as deactivated
	ch.Status = "deactivated"
	ch.DeactivatedAt = time.Now().UTC().Format(time.RFC3339)
	ch.DeactivatedBy = reason
	if err := WriteCharacter(root, projectName, *ch); err != nil {
		return "", err
	}

	// Append to relationships deactivated list
	relsPath := filepath.Join(Dir(root, projectName), "relationships.yaml")
	relsData, _ := os.ReadFile(relsPath)
	var rels map[string]any
	yaml.Unmarshal(relsData, &rels)
	deactivated, _ := rels["deactivated_characters"].([]any)
	deactivated = append(deactivated, map[string]any{
		"id":     ch.ID,
		"name":   ch.Name,
		"role":   ch.Role,
		"reason": reason,
		"at":     ch.DeactivatedAt,
	})
	rels["deactivated_characters"] = deactivated
	writeYAML(relsPath, rels)

	summary := fmt.Sprintf(
		"角色「%s」(%s) 已下线\n原因: %s\n影响: 该角色从此仅在回忆/闪回中出现，关系网保留其历史关联",
		ch.Name, ch.Role, reason,
	)
	return summary, nil
}

// ---- Output operations ----

// WriteChapter saves a generated chapter to output/ch<num>.txt.
func WriteChapter(root, projectName string, chapterNo int, content string) error {
	filename := fmt.Sprintf("ch%03d.txt", chapterNo)
	path := filepath.Join(Dir(root, projectName), "output", filename)
	return os.WriteFile(path, []byte(content), 0o644)
}

// ExportChapter reads and returns a generated chapter.
func ExportChapter(root, projectName string, chapterNo int) (string, error) {
	filename := fmt.Sprintf("ch%03d.txt", chapterNo)
	path := filepath.Join(Dir(root, projectName), "output", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ExportAll concatenates all chapters into a single book-ready txt.
func ExportAll(root, projectName string) (string, error) {
	dir := filepath.Join(Dir(root, projectName), "output")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var all string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".txt" {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		all += string(data) + "\n\n━━━━━━━━━━━━━━━━━━━━\n\n"
	}
	return all, nil
}

// ---- World / Outline helpers ----

// ReadWorld loads the world.yaml.
func ReadWorld(root, projectName string) (map[string]any, error) {
	return readYAML(filepath.Join(Dir(root, projectName), "world.yaml"))
}

// WriteWorld saves the world.yaml.
func WriteWorld(root, projectName string, data map[string]any) error {
	return writeYAML(filepath.Join(Dir(root, projectName), "world.yaml"), data)
}

// ReadOutline loads the outline.yaml.
func ReadOutline(root, projectName string) (map[string]any, error) {
	return readYAML(filepath.Join(Dir(root, projectName), "outline.yaml"))
}

// WriteOutline saves the outline.yaml.
func WriteOutline(root, projectName string, data map[string]any) error {
	return writeYAML(filepath.Join(Dir(root, projectName), "outline.yaml"), data)
}

// ReadRelationships loads relationships.yaml.
func ReadRelationships(root, projectName string) (map[string]any, error) {
	return readYAML(filepath.Join(Dir(root, projectName), "relationships.yaml"))
}

// WriteRelationships saves relationships.yaml.
func WriteRelationships(root, projectName string, data map[string]any) error {
	return writeYAML(filepath.Join(Dir(root, projectName), "relationships.yaml"), data)
}

// ListProjects returns names of all project directories.
func ListProjects(root string) ([]string, error) {
	dir := filepath.Join(root, "projects")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// --- helpers ---

func writeYAML(path string, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("project: marshal yaml: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

func readYAML(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("project: read %s: %w", path, err)
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("project: parse %s: %w", path, err)
	}
	return m, nil
}

func sanitizeFilename(s string) string {
	var out []rune
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			out = append(out, r)
		} else if r >= 0x4E00 && r <= 0x9FFF {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return "character"
	}
	return string(out)
}

// CharacterID returns the file-safe ID for a character name.
func CharacterID(name string) string {
	return sanitizeFilename(name)
}
