package jack

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

// RegistryEntry records a cloned project for an agent.
type RegistryEntry struct {
	ClonedAt time.Time `yaml:"cloned_at"`
	Agent    string    `yaml:"agent"`
	Repo     string    `yaml:"repo"`
	URL      string    `yaml:"url"`
}

// Registry tracks which repos have been cloned for which agents.
type Registry struct {
	Projects []RegistryEntry `yaml:"projects"`
}

// RegistryLoader loads the registry from disk.
type RegistryLoader func() (*Registry, error)

// RegistrySaver persists the registry to disk.
type RegistrySaver func(*Registry) error

// Add records a new project entry, replacing any existing entry for the same agent+repo.
func (r *Registry) Add(agent, repo, url string) {
	r.Remove(agent, repo)
	r.Projects = append(r.Projects, RegistryEntry{
		Agent:    agent,
		Repo:     repo,
		URL:      url,
		ClonedAt: time.Now().UTC().Truncate(time.Second),
	})
}

// Remove deletes the entry for a given agent+repo if it exists.
func (r *Registry) Remove(agent, repo string) {
	filtered := r.Projects[:0]
	for _, p := range r.Projects {
		if p.Agent != agent || p.Repo != repo {
			filtered = append(filtered, p)
		}
	}
	r.Projects = filtered
}

// Find returns the entry for a given agent+repo, or nil if not found.
func (r *Registry) Find(agent, repo string) *RegistryEntry {
	for i := range r.Projects {
		if r.Projects[i].Agent == agent && r.Projects[i].Repo == repo {
			return &r.Projects[i]
		}
	}
	return nil
}

// ForAgent returns all entries for the given agent, sorted by repo name.
func (r *Registry) ForAgent(agent string) []RegistryEntry {
	var entries []RegistryEntry
	for _, p := range r.Projects {
		if p.Agent == agent {
			entries = append(entries, p)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Repo < entries[j].Repo
	})
	return entries
}

// Agents returns a sorted list of unique agent names in the registry.
func (r *Registry) Agents() []string {
	seen := make(map[string]bool)
	for _, p := range r.Projects {
		seen[p.Agent] = true
	}
	agents := make([]string, 0, len(seen))
	for a := range seen {
		agents = append(agents, a)
	}
	sort.Strings(agents)
	return agents
}

// AgentsForRepo returns a sorted list of unique agent names that have cloned
// the given repo.
func (r *Registry) AgentsForRepo(repo string) []string {
	seen := make(map[string]bool)
	for _, p := range r.Projects {
		if p.Repo == repo {
			seen[p.Agent] = true
		}
	}
	agents := make([]string, 0, len(seen))
	for a := range seen {
		agents = append(agents, a)
	}
	sort.Strings(agents)
	return agents
}

// ReposForAgent returns a sorted list of repo names for the given agent.
func (r *Registry) ReposForAgent(agent string) []string {
	var repos []string
	for _, p := range r.Projects {
		if p.Agent == agent {
			repos = append(repos, p.Repo)
		}
	}
	sort.Strings(repos)
	return repos
}

func registryPath() string {
	return filepath.Join(env.dataDir(), "registry.yaml")
}

func loadRegistry() (*Registry, error) {
	data, err := os.ReadFile(registryPath())
	if os.IsNotExist(err) {
		return &Registry{}, nil
	}
	if err != nil {
		return nil, err
	}
	var reg Registry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, err
	}
	return &reg, nil
}

func saveRegistry(reg *Registry) error {
	data, err := yaml.Marshal(reg)
	if err != nil {
		return err
	}
	dir := filepath.Dir(registryPath())
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	return os.WriteFile(registryPath(), data, 0o600)
}
