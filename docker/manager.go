package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/runners-manager/config"
)

const RunnerImage = "myoung34/github-runner:latest"

type Manager struct {
	cli *client.Client
}

func NewManager() (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Manager{cli: cli}, nil
}

func (m *Manager) PullImage(ctx context.Context) error {
	fmt.Printf("Pulling image %s...\n", RunnerImage)
	reader, err := m.cli.ImagePull(ctx, RunnerImage, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()
	io.Copy(os.Stdout, reader)
	return nil
}

func (m *Manager) StartRunner(ctx context.Context, runner *config.Runner) error {
	env := []string{
		fmt.Sprintf("REPO_URL=%s", runner.URL),
		fmt.Sprintf("RUNNER_NAME=%s", runner.Name),
		fmt.Sprintf("RUNNER_TOKEN=%s", runner.Token),
	}

	if runner.Labels != "" {
		env = append(env, fmt.Sprintf("RUNNER_LABELS=%s", runner.Labels))
	}

	containerName := fmt.Sprintf("gh-runner-%s", runner.Name)

	resp, err := m.cli.ContainerCreate(ctx, &container.Config{
		Image: RunnerImage,
		Env:   env,
	}, &container.HostConfig{
		AutoRemove: false,
		Privileged: true, // Required for some DinD operations
		Binds: []string{
			"/var/run/docker.sock:/var/run/docker.sock", // Mount host docker socket
		},
	}, nil, nil, containerName)

	if err != nil {
		// If it's a conflict (container with same name exists), we might want to return a specific error
		return fmt.Errorf("failed to create container: %w", err)
	}

	if err := m.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	runner.ContainerID = resp.ID
	return nil
}

func (m *Manager) StopRunner(ctx context.Context, containerID string) error {
	if containerID == "" {
		return nil // Not running
	}

	// Stop container
	err := m.cli.ContainerStop(ctx, containerID, container.StopOptions{})
	if err != nil && !strings.Contains(err.Error(), "No such container") {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	return nil
}

func (m *Manager) RemoveRunner(ctx context.Context, containerID string) error {
	if containerID == "" {
		return nil
	}

	err := m.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: true,
	})
	if err != nil && !strings.Contains(err.Error(), "No such container") {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	return nil
}

func (m *Manager) IsRunning(ctx context.Context, containerID string) (bool, error) {
	if containerID == "" {
		return false, nil
	}
	inspect, err := m.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		if strings.Contains(err.Error(), "No such container") {
			return false, nil
		}
		return false, err
	}
	return inspect.State.Running, nil
}
