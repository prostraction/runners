package docker

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/moby/term"
	"github.com/runners/config"
)

const RunnerImage = "myoung34/github-runner:latest"

type Manager struct {
	cli client.CommonAPIClient
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

	termFd, isTerm := term.GetFdInfo(os.Stdout)
	return jsonmessage.DisplayJSONMessagesStream(reader, os.Stdout, termFd, isTerm, nil)
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

	hostConfig := &container.HostConfig{
		AutoRemove: false,
		Privileged: true, // Required for some DinD operations
		Binds: []string{
			"/var/run/docker.sock:/var/run/docker.sock", // Mount host docker socket
		},
	}

	if runner.CPULimit > 0 {
		hostConfig.Resources.NanoCPUs = int64(runner.CPULimit * 1e9)
	}
	if runner.MemoryLimit > 0 {
		memBytes := runner.MemoryLimit * 1024 * 1024
		hostConfig.Resources.Memory = memBytes
		hostConfig.Resources.MemorySwap = memBytes // Ensure swap limit is equal to memory limit
	}

	resp, err := m.cli.ContainerCreate(ctx, &container.Config{
		Image: RunnerImage,
		Env:   env,
	}, hostConfig, nil, nil, containerName)

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

func (m *Manager) ResumeRunner(ctx context.Context, containerID string) error {
	return m.cli.ContainerStart(ctx, containerID, container.StartOptions{})
}

type RunnerInfo struct {
	IsRunning      bool
	Uptime         string
	ExitCode       int
	InternalStatus string // "Idle" or "Working"
}

func (m *Manager) GetRunnerInfo(ctx context.Context, containerID string) (*RunnerInfo, error) {
	if containerID == "" {
		return &RunnerInfo{IsRunning: false, Uptime: "-", ExitCode: 0, InternalStatus: "-"}, nil
	}
	inspect, err := m.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		if strings.Contains(err.Error(), "No such container") {
			return &RunnerInfo{IsRunning: false, Uptime: "-", ExitCode: 0, InternalStatus: "-"}, nil
		}
		return nil, err
	}

	info := &RunnerInfo{
		IsRunning:      inspect.State.Running,
		ExitCode:       inspect.State.ExitCode,
		InternalStatus: "-",
	}

	if inspect.State.Running {
		startedAt, _ := time.Parse(time.RFC3339Nano, inspect.State.StartedAt)
		duration := time.Since(startedAt).Round(time.Second)
		info.Uptime = duration.String()

		// Check internal status by looking at processes
		top, err := m.cli.ContainerTop(ctx, containerID, nil)
		if err == nil {
			info.InternalStatus = "Idle"
			for _, proc := range top.Processes {
				// The process list contains rows of strings. 
				// We look for "Runner.Worker" in any of the fields (usually the command field)
				for _, field := range proc {
					if strings.Contains(field, "Runner.Worker") {
						info.InternalStatus = "Working"
						break
					}
				}
				if info.InternalStatus == "Working" {
					break
				}
			}
		}
	} else {
		info.Uptime = "Stopped"
	}

	return info, nil
}

func (m *Manager) UpdateResources(ctx context.Context, containerID string, cpu float64, memory int64) error {
	resources := container.Resources{}
	if cpu > 0 {
		resources.NanoCPUs = int64(cpu * 1e9)
	}
	if memory > 0 {
		memBytes := memory * 1024 * 1024
		resources.Memory = memBytes
		resources.MemorySwap = memBytes // Update swap limit to match memory limit
	}

	_, err := m.cli.ContainerUpdate(ctx, containerID, container.UpdateConfig{
		Resources: resources,
	})
	return err
}
