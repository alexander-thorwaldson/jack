package jack

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const jackImage = "jack"

const baseDockerfile = `FROM node:22-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    git openssh-client && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /root/.ssh && chmod 700 /root/.ssh \
    && ssh-keyscan github.com >> /root/.ssh/known_hosts
RUN npm install -g @anthropic-ai/claude-code
COPY msg /usr/local/bin/msg
WORKDIR /workspace
`

// Mount describes a Docker bind mount.
type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

// ImageBuilder builds the jack base Docker image.
type ImageBuilder func(ctx context.Context) error

// ContainerRunner starts an idle container with the given mounts and env.
type ContainerRunner func(name string, mounts []Mount, env map[string]string) error

// ContainerStopper stops and removes a container.
type ContainerStopper func(name string) error

// ContainerExecer runs a command inside a running container.
type ContainerExecer func(name string, cmd []string) error

// ContainerChecker reports whether a container is running and/or exists.
type ContainerChecker func(name string) (running bool, exists bool)

// ContainerName builds the canonical Docker container name for an agent and repo.
func ContainerName(agent, repo string) string {
	return "jack-" + agent + "-" + repo
}

// SessionMounts returns the standard bind mounts for a session container.
// Supporting repos from the agent config are mounted at /repos/<name>.
func SessionMounts(c Config, agent, repoDir string) []Mount {
	home, _ := os.UserHomeDir()
	mounts := []Mount{
		{Source: filepath.Join(home, ".claude"), Target: "/root/.claude", ReadOnly: false},
		{Source: repoDir, Target: "/workspace", ReadOnly: false},
	}
	if ac, ok := c.Agents[agent]; ok {
		for _, repoURL := range ac.Repos {
			name := repoName(repoURL)
			if name == "" {
				continue
			}
			supportDir := filepath.Join(env.dataDir(), agent, name)
			if _, err := os.Stat(supportDir); err == nil {
				mounts = append(mounts, Mount{Source: supportDir, Target: "/repos/" + name, ReadOnly: false})
			}
		}
	}
	return mounts
}

// SessionEnv returns the environment variables for a session container.
func SessionEnv(c Config, agent, token, ghToken string) map[string]string {
	e := make(map[string]string)
	if agent != "" {
		e["JACK_AGENT"] = agent
	}
	if token != "" {
		e["JACK_MSG_TOKEN"] = token
	}
	if ghToken != "" {
		e["GH_TOKEN"] = ghToken
	}
	if c.Matrix.Homeserver != "" {
		e["JACK_HOMESERVER"] = c.Matrix.Homeserver
	}
	if c.Git.Name != "" {
		e["GIT_AUTHOR_NAME"] = c.Git.Name
		e["GIT_COMMITTER_NAME"] = c.Git.Name
	}
	if c.Git.Email != "" {
		e["GIT_AUTHOR_EMAIL"] = c.Git.Email
		e["GIT_COMMITTER_EMAIL"] = c.Git.Email
	}
	return e
}

// DockerBuild cross-compiles the msg binary and builds the jack base image.
func DockerBuild(ctx context.Context) error {
	dir, err := os.MkdirTemp("", "jack-docker-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	// Cross-compile msg for the container (Linux, matching host arch).
	binPath := filepath.Join(dir, "msg")
	goBuild := exec.CommandContext(ctx, "go", "build", "-o", binPath, "./cmd/msg") // #nosec G204 -- args from internal paths
	goBuild.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+runtime.GOARCH, "CGO_ENABLED=0")
	goBuild.Stdout = os.Stdout
	goBuild.Stderr = os.Stderr
	if err := goBuild.Run(); err != nil {
		return fmt.Errorf("building msg: %w", err)
	}

	dockerfilePath := filepath.Join(dir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(baseDockerfile), 0o600); err != nil {
		return fmt.Errorf("writing Dockerfile: %w", err)
	}

	cmd := exec.CommandContext(ctx, "docker", "build", "-t", jackImage, dir) // #nosec G204 -- args from internal constants
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker build: %w", err)
	}
	return nil
}

// DockerRun starts an idle container with the given name, mounts, and env.
func DockerRun(name string, mounts []Mount, envVars map[string]string) error {
	args := make([]string, 0, 6+2*len(mounts)+2*len(envVars)+3)
	args = append(args, "run", "-d", "--name", name, "-w", "/workspace")
	for _, m := range mounts {
		vol := m.Source + ":" + m.Target
		if m.ReadOnly {
			vol += ":ro"
		}
		args = append(args, "-v", vol)
	}
	for k, v := range envVars {
		args = append(args, "-e", k+"="+v)
	}
	args = append(args, jackImage, "sleep", "infinity")

	cmd := exec.CommandContext(context.Background(), "docker", args...) // #nosec G204 -- args from internal config
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker run: %w: %s", err, stderr.String())
	}
	return nil
}

// DockerExec runs a command inside a running container, streaming output.
func DockerExec(name string, cmdArgs []string) error {
	args := make([]string, 0, 2+len(cmdArgs))
	args = append(args, "exec", name)
	args = append(args, cmdArgs...)
	cmd := exec.CommandContext(context.Background(), "docker", args...) // #nosec G204 -- args from internal config
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker exec: %w", err)
	}
	return nil
}

// DockerStop stops and removes a container.
func DockerStop(name string) error {
	cmd := exec.CommandContext(context.Background(), "docker", "rm", "-f", name) // #nosec G204 -- args from internal session name
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker rm: %w: %s", err, stderr.String())
	}
	return nil
}

// DockerCheck reports whether a container is running and whether it exists.
func DockerCheck(name string) (running bool, exists bool) {
	cmd := exec.CommandContext(context.Background(), "docker", "inspect", "--format", "{{.State.Running}}", name) // #nosec G204 -- args from internal session name
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return false, false
	}
	state := strings.TrimSpace(stdout.String())
	return state == "true", true
}

// DockerExecCmd returns the tmux command string that execs into a container.
func DockerExecCmd(container, shellCmd string) string {
	return fmt.Sprintf("docker exec -it %s sh -c %q", container, shellCmd)
}
