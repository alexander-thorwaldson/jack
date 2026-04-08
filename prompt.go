package jack

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

// AgentSelector prompts the user to select an agent.
type AgentSelector func(agents []string) (string, error)

// ProjectSelector prompts the user to select a project for an agent.
type ProjectSelector func(agent string, repos []string) (string, error)

func selectAgent(agents []string) (string, error) {
	var agent string
	err := huh.NewSelect[string]().
		Title("Select an agent").
		Options(huh.NewOptions(agents...)...).
		Value(&agent).
		Run()
	if err != nil {
		return "", fmt.Errorf("selecting agent: %w", err)
	}
	return agent, nil
}

func selectProject(agent string, repos []string) (string, error) {
	var repo string
	err := huh.NewSelect[string]().
		Title(fmt.Sprintf("Select a project for %s", agent)).
		Options(huh.NewOptions(repos...)...).
		Value(&repo).
		Run()
	if err != nil {
		return "", fmt.Errorf("selecting project: %w", err)
	}
	return repo, nil
}
