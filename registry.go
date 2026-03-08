package jack

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

// RegistryEntry records a cloned project for a team.
type RegistryEntry struct {
	ClonedAt time.Time `yaml:"cloned_at"`
	Team     string    `yaml:"team"`
	Repo     string    `yaml:"repo"`
	URL      string    `yaml:"url"`
}

// Registry tracks which repos have been cloned for which teams.
type Registry struct {
	Projects []RegistryEntry `yaml:"projects"`
}

// RegistryLoader loads the registry from disk.
type RegistryLoader func() (*Registry, error)

// RegistrySaver persists the registry to disk.
type RegistrySaver func(*Registry) error

// Add records a new project entry, replacing any existing entry for the same team+repo.
func (r *Registry) Add(team, repo, url string) {
	r.Remove(team, repo)
	r.Projects = append(r.Projects, RegistryEntry{
		Team:     team,
		Repo:     repo,
		URL:      url,
		ClonedAt: time.Now().UTC().Truncate(time.Second),
	})
}

// Remove deletes the entry for a given team+repo if it exists.
func (r *Registry) Remove(team, repo string) {
	filtered := r.Projects[:0]
	for _, p := range r.Projects {
		if p.Team != team || p.Repo != repo {
			filtered = append(filtered, p)
		}
	}
	r.Projects = filtered
}

// Find returns the entry for a given team+repo, or nil if not found.
func (r *Registry) Find(team, repo string) *RegistryEntry {
	for i := range r.Projects {
		if r.Projects[i].Team == team && r.Projects[i].Repo == repo {
			return &r.Projects[i]
		}
	}
	return nil
}

// ForTeam returns all entries for the given team, sorted by repo name.
func (r *Registry) ForTeam(team string) []RegistryEntry {
	var entries []RegistryEntry
	for _, p := range r.Projects {
		if p.Team == team {
			entries = append(entries, p)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Repo < entries[j].Repo
	})
	return entries
}

// Teams returns a sorted list of unique team names in the registry.
func (r *Registry) Teams() []string {
	seen := make(map[string]bool)
	for _, p := range r.Projects {
		seen[p.Team] = true
	}
	teams := make([]string, 0, len(seen))
	for t := range seen {
		teams = append(teams, t)
	}
	sort.Strings(teams)
	return teams
}

// TeamsForRepo returns a sorted list of unique team names that have cloned
// the given repo.
func (r *Registry) TeamsForRepo(repo string) []string {
	seen := make(map[string]bool)
	for _, p := range r.Projects {
		if p.Repo == repo {
			seen[p.Team] = true
		}
	}
	teams := make([]string, 0, len(seen))
	for t := range seen {
		teams = append(teams, t)
	}
	sort.Strings(teams)
	return teams
}

// ReposForTeam returns a sorted list of repo names for the given team.
func (r *Registry) ReposForTeam(team string) []string {
	var repos []string
	for _, p := range r.Projects {
		if p.Team == team {
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
