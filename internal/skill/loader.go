package skill

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/penney-101/ai-novel-agent/internal/storage"
)

// Loader reads skill definitions from the .novelAgent/skills/ directory.
type Loader struct {
	root string
}

// NewLoader creates a Loader for the given .novelAgent root directory.
func NewLoader(root string) *Loader {
	return &Loader{root: root}
}

// Load reads and validates a single skill by name.
func (l *Loader) Load(name string) (*Skill, error) {
	raw, err := storage.ReadSkill(l.root, name)
	if err != nil {
		return nil, fmt.Errorf("loader: read skill %q: %w", name, err)
	}
	return parseSkill(raw, name)
}

// LoadAll reads all installed skills and returns them keyed by name.
// Invalid skills are skipped with a log-like diagnostic appended to diags.
func (l *Loader) LoadAll() (map[string]*Skill, []string) {
	names, err := storage.ListSkills(l.root)
	if err != nil {
		return nil, []string{fmt.Sprintf("loader: list skills: %v", err)}
	}

	skills := make(map[string]*Skill, len(names))
	var diags []string

	for _, name := range names {
		s, err := l.Load(name)
		if err != nil {
			diags = append(diags, fmt.Sprintf("skip %q: %v", name, err))
			continue
		}
		skills[name] = s
	}
	return skills, diags
}

// parseSkill unmarshals raw YAML map into a Skill and validates it.
func parseSkill(raw map[string]any, name string) (*Skill, error) {
	// re-marshal to YAML then unmarshal into struct (robust YAML→struct path)
	data, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("parse skill %q: re-marshal: %w", name, err)
	}
	var s Skill
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse skill %q: %w", name, err)
	}
	if s.Name == "" {
		s.Name = name
	}
	if errs := s.Validate(); len(errs) > 0 {
		msg := fmt.Sprintf("parse skill %q:", name)
		for _, e := range errs {
			msg += "\n  - " + e.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return &s, nil
}

// Manager provides thread-safe access to loaded skills with hot-reload capability.
type Manager struct {
	mu     sync.RWMutex
	loader *Loader
	skills map[string]*Skill // name → parsed skill
}

// NewManager creates a Manager that loads all skills from the given root.
func NewManager(root string) (*Manager, error) {
	m := &Manager{
		loader: NewLoader(root),
	}
	if err := m.Reload(); err != nil {
		return nil, err
	}
	return m, nil
}

// Get returns the skill by name, or nil if not found.
func (m *Manager) Get(name string) *Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.skills[name]
}

// List returns the names of all loaded skills.
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.skills))
	for n := range m.skills {
		names = append(names, n)
	}
	return names
}

// Reload re-reads all skill definitions from disk (hot-reload support).
func (m *Manager) Reload() error {
	skills, diags := m.loader.LoadAll()
	if len(diags) > 0 {
		for _, d := range diags {
			fmt.Fprintln(os.Stderr, "[skill]", d)
		}
	}
	m.mu.Lock()
	m.skills = skills
	m.mu.Unlock()
	return nil
}
