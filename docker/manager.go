package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/moby/term"
	"github.com/runners/config"
)

// RunnerImage is the docker image used for GitHub runners.
const RunnerImage = "myoung34/github-runner:latest"

// Manager handles docker operations for GitHub runners.
type Manager struct {
	cli client.APIClient
}

// NewManager creates a new Docker manager.
func NewManager() (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Manager{cli: cli}, nil
}

// PullImage downloads the runner image from the registry.
func (m *Manager) PullImage(ctx context.Context) error {
	fmt.Printf("Pulling image %s...\n", RunnerImage)
	reader, err := m.cli.ImagePull(ctx, RunnerImage, image.PullOptions{})
	if err != nil {
		return err
	}
	defer func() {
		_ = reader.Close()
	}()

	termFd, isTerm := term.GetFdInfo(os.Stdout)
	return jsonmessage.DisplayJSONMessagesStream(reader, os.Stdout, termFd, isTerm, nil)
}

// StartRunner creates and starts a new runner container.
func (m *Manager) StartRunner(ctx context.Context, runner *config.Runner) error {
	// Prepare persistent storage directory
	dataDir := config.DataDir(runner.Name)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	env := []string{
		fmt.Sprintf("REPO_URL=%s", runner.URL),
		fmt.Sprintf("RUNNER_NAME=%s", runner.Name),
		fmt.Sprintf("RUNNER_TOKEN=%s", runner.Token),
		"CONFIGURED_ACTIONS_RUNNER_FILES_DIR=/runner/data",
		"DISABLE_AUTOMATIC_DEREGISTRATION=true",
		"CONFIG_OPTS=--replace",
	}

	if runner.Labels != "" {
		env = append(env, fmt.Sprintf("RUNNER_LABELS=%s", runner.Labels))
	}

	containerName := fmt.Sprintf("gh-runner-%s", runner.Name)

	// Proactively remove any container with the same name left over from a
	// previous failed run. Without this, ContainerCreate below fails with
	// "name already in use" and the tool looks broken on retry.
	if err := m.cli.ContainerRemove(ctx, containerName, container.RemoveOptions{Force: true}); err != nil &&
		!strings.Contains(err.Error(), "No such container") {
		// Not fatal — surface the underlying create error if there is one below.
		fmt.Printf("Warning: could not clean stale container %q: %v\n", containerName, err)
	}

	hostConfig := &container.HostConfig{
		AutoRemove: false,
		Privileged: true, // Required for some DinD operations
		Binds: []string{
			"/var/run/docker.sock:/var/run/docker.sock", // Mount host docker socket
			fmt.Sprintf("%s:/runner/data", dataDir),     // Persist runner configuration
		},
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyUnlessStopped,
		},
	}

	if runner.CPULimit > 0 {
		hostConfig.NanoCPUs = int64(runner.CPULimit * 1e9)
	}
	if runner.MemoryLimit > 0 {
		memBytes := runner.MemoryLimit * 1024 * 1024
		hostConfig.Memory = memBytes
		hostConfig.MemorySwap = memBytes
	}

	resp, err := m.cli.ContainerCreate(ctx, &container.Config{
		Image: RunnerImage,
		Env:   env,
		Cmd:   []string{"./run.sh"},
	}, hostConfig, nil, nil, containerName)

	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	if err := m.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		// Remove the created-but-never-started container, otherwise its name
		// stays taken and future StartRunner calls fail with a name conflict.
		_ = m.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return fmt.Errorf("failed to start container: %w", err)
	}

	runner.ContainerID = resp.ID
	return nil
}

// StopRunner stops a running container.
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

// RemoveRunner removes a container by ID.
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

// RemoveContainerByName force-removes a container by its runner-derived name
// (gh-runner-<name>). Swallows "No such container" so callers can use it as a
// best-effort cleanup for orphans.
func (m *Manager) RemoveContainerByName(ctx context.Context, name string) error {
	containerName := fmt.Sprintf("gh-runner-%s", name)
	err := m.cli.ContainerRemove(ctx, containerName, container.RemoveOptions{Force: true})
	if err != nil && !strings.Contains(err.Error(), "No such container") {
		return fmt.Errorf("failed to remove container %q: %w", containerName, err)
	}
	return nil
}

// IsRunning checks if a container is currently running.
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

// ResumeRunner starts an existing container.
func (m *Manager) ResumeRunner(ctx context.Context, containerID string) error {
	return m.cli.ContainerStart(ctx, containerID, container.StartOptions{})
}

// RunnerInfo contains detailed information about a runner's status.
type RunnerInfo struct {
	IsRunning      bool
	Uptime         string
	ExitCode       int
	InternalStatus string // "Idle", "Working", "Not Connected", or "-"
}

// GetRunnerInfo retrieves detailed status for a runner.
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
		startedAt, err := time.Parse(time.RFC3339Nano, inspect.State.StartedAt)
		if err == nil {
			duration := time.Since(startedAt).Round(time.Second)
			info.Uptime = duration.String()
		} else {
			info.Uptime = "Unknown"
		}

		// Check internal status by looking at processes.
		// Runner.Worker = a job is actively running.
		// Runner.Listener = the runner is connected to GitHub and waiting for jobs.
		// Neither = the runner container is up but not registered/connected (e.g. stale
		// credentials, or runner was deleted from GitHub).
		top, err := m.cli.ContainerTop(ctx, containerID, nil)
		if err == nil {
			hasListener := false
			hasWorker := false
			for _, proc := range top.Processes {
				for _, field := range proc {
					if strings.Contains(field, "Runner.Worker") {
						hasWorker = true
					}
					if strings.Contains(field, "Runner.Listener") {
						hasListener = true
					}
				}
			}
			switch {
			case hasWorker:
				info.InternalStatus = "Working"
			case hasListener:
				info.InternalStatus = "Idle"
			default:
				info.InternalStatus = "Not Connected"
			}
		}
	} else {
		info.Uptime = "Stopped"
	}

	return info, nil
}

// EnsureRestartPolicy applies the unless-stopped restart policy to an existing
// container. This lets Docker auto-restart the runner after a host reboot or
// daemon restart, so runners do not get stuck in "Exited (143)" after the PC
// is turned off and on again.
func (m *Manager) EnsureRestartPolicy(ctx context.Context, containerID string) error {
	if containerID == "" {
		return nil
	}
	_, err := m.cli.ContainerUpdate(ctx, containerID, container.UpdateConfig{
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyUnlessStopped,
		},
	})
	if err != nil && strings.Contains(err.Error(), "No such container") {
		return nil
	}
	return err
}

// VerifyStartup waits until the runner container has a live Runner.Listener
// process, meaning registration with GitHub succeeded and the runner is ready
// to accept jobs. Returns an error if the container crashes or fails to
// connect within the given timeout.
func (m *Manager) VerifyStartup(ctx context.Context, containerID string, timeout time.Duration) error {
	if containerID == "" {
		return fmt.Errorf("empty container ID")
	}

	deadline := time.Now().Add(timeout)
	for {
		inspect, err := m.cli.ContainerInspect(ctx, containerID)
		if err != nil {
			return fmt.Errorf("failed to inspect container: %w", err)
		}
		if !inspect.State.Running {
			return fmt.Errorf("container exited with code %d", inspect.State.ExitCode)
		}

		top, err := m.cli.ContainerTop(ctx, containerID, nil)
		if err == nil {
			for _, proc := range top.Processes {
				for _, field := range proc {
					if strings.Contains(field, "Runner.Listener") {
						return nil
					}
				}
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("runner did not connect to GitHub within %s (check token and registration)", timeout)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
}

// purgeDataDirTimeout bounds cleanup container lifetime so rm/add can't hang
// indefinitely if the docker daemon wedges.
const purgeDataDirTimeout = 30 * time.Second

// PurgeDataDir removes a runner's persistent data directory. Files inside
// were created by the runner container running as root, so a plain
// os.RemoveAll from the host user fails with "permission denied" on bind
// mounts. Instead we spin up a short-lived container that shares root's UID
// namespace, have it wipe /data from the inside, then drop the now-empty
// directory from the host.
func (m *Manager) PurgeDataDir(ctx context.Context, name string) error {
	dataDir := config.DataDir(name)
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, purgeDataDirTimeout)
	defer cancel()

	cfg := &container.Config{
		Image:      RunnerImage,
		Entrypoint: []string{"sh", "-c"},
		Cmd:        []string{"find /data -mindepth 1 -delete"},
	}
	hostCfg := &container.HostConfig{
		AutoRemove: true,
		Binds:      []string{fmt.Sprintf("%s:/data", dataDir)},
	}

	resp, createErr := m.cli.ContainerCreate(ctx, cfg, hostCfg, nil, nil, "")
	if createErr != nil {
		// Fall back to host-side removal. It will usually fail on root-owned
		// files, but try anyway so we at least propagate a clear error.
		if err := os.RemoveAll(dataDir); err != nil {
			return fmt.Errorf("failed to create cleanup container (%v); host removal also failed: %w", createErr, err)
		}
		return nil
	}

	if err := m.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = m.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return fmt.Errorf("failed to start cleanup container: %w", err)
	}

	waitCh, errCh := m.cli.ContainerWait(ctx, resp.ID, container.WaitConditionRemoved)
	select {
	case <-waitCh:
	case err := <-errCh:
		// AutoRemove can race with Wait and yield "No such container" — that's
		// actually the success path, so swallow it.
		if err != nil && !strings.Contains(err.Error(), "No such container") {
			return fmt.Errorf("cleanup container wait failed: %w", err)
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	return os.RemoveAll(dataDir)
}

// StreamLogs writes container logs to the given writers. If follow is true,
// the call blocks until the context is cancelled or the container stops. The
// tail argument is a line count or "all".
func (m *Manager) StreamLogs(ctx context.Context, containerID string, follow bool, tail string, stdout, stderr io.Writer) error {
	if containerID == "" {
		return fmt.Errorf("runner has no container id (not yet started)")
	}
	reader, err := m.cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Tail:       tail,
		Timestamps: false,
	})
	if err != nil {
		return fmt.Errorf("failed to fetch container logs: %w", err)
	}
	defer func() {
		_ = reader.Close()
	}()
	if _, err := stdcopy.StdCopy(stdout, stderr, reader); err != nil && err != io.EOF {
		return err
	}
	return nil
}

// UpdateResources dynamically updates container resource limits.
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
