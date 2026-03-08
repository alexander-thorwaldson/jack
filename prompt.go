package jack

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

// TeamSelector prompts the user to select a team.
type TeamSelector func(teams []string) (string, error)

// ProjectSelector prompts the user to select a project for a team.
type ProjectSelector func(team string, repos []string) (string, error)

func selectTeam(teams []string) (string, error) {
	var team string
	err := huh.NewSelect[string]().
		Title("Select a team").
		Options(huh.NewOptions(teams...)...).
		Value(&team).
		Run()
	if err != nil {
		return "", fmt.Errorf("selecting team: %w", err)
	}
	return team, nil
}

func selectProject(team string, repos []string) (string, error) {
	var repo string
	err := huh.NewSelect[string]().
		Title(fmt.Sprintf("Select a project for %s", team)).
		Options(huh.NewOptions(repos...)...).
		Value(&repo).
		Run()
	if err != nil {
		return "", fmt.Errorf("selecting project: %w", err)
	}
	return repo, nil
}
