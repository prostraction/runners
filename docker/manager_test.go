package docker

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/runners/config"
)

type mockDockerClient struct {
	client.CommonAPIClient
	containerCreateFunc  func(config *container.Config, hostConfig *container.HostConfig) (container.CreateResponse, error)
	containerStartFunc   func(containerID string) error
	containerStopFunc    func(containerID string) error
	containerRemoveFunc  func(containerID string) error
	containerInspectFunc func(containerID string) (types.ContainerJSON, error)
}

func (m *mockDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	return m.containerCreateFunc(config, hostConfig)
}

func (m *mockDockerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	return m.containerStartFunc(containerID)
}

func (m *mockDockerClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	return m.containerStopFunc(containerID)
}

func (m *mockDockerClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	return m.containerRemoveFunc(containerID)
}

func (m *mockDockerClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	return m.containerInspectFunc(containerID)
}

func (m *mockDockerClient) ContainerUpdate(ctx context.Context, containerID string, updateConfig container.UpdateConfig) (container.ContainerUpdateOKBody, error) {
	return container.ContainerUpdateOKBody{}, nil
}

func (m *mockDockerClient) ContainerTop(ctx context.Context, containerID string, arguments []string) (container.ContainerTopOKBody, error) {
	return container.ContainerTopOKBody{Processes: [][]string{{"123", "Runner.Worker"}}}, nil
}

func TestStartRunner(t *testing.T) {
	mock := &mockDockerClient{
		containerCreateFunc: func(c *container.Config, h *container.HostConfig) (container.CreateResponse, error) {
			if h.Resources.Memory != 512*1024*1024 {
				t.Errorf("expected 512MB memory limit, got %d", h.Resources.Memory)
			}
			return container.CreateResponse{ID: "test-id"}, nil
		},
		containerStartFunc: func(id string) error {
			return nil
		},
	}

	mgr := &Manager{cli: mock}
	runner := &config.Runner{
		Name:        "test",
		MemoryLimit: 512,
	}

	err := mgr.StartRunner(context.Background(), runner)
	if err != nil {
		t.Fatalf("StartRunner failed: %v", err)
	}

	if runner.ContainerID != "test-id" {
		t.Errorf("expected container ID 'test-id', got '%s'", runner.ContainerID)
	}
}

func TestStopRunner(t *testing.T) {
	called := false
	mock := &mockDockerClient{
		containerStopFunc: func(id string) error {
			called = true
			return nil
		},
	}
	mgr := &Manager{cli: mock}
	mgr.StopRunner(context.Background(), "id")
	if !called {
		t.Error("ContainerStop was not called")
	}
}

func TestRemoveRunner(t *testing.T) {
	called := false
	mock := &mockDockerClient{
		containerRemoveFunc: func(id string) error {
			called = true
			return nil
		},
	}
	mgr := &Manager{cli: mock}
	mgr.RemoveRunner(context.Background(), "id")
	if !called {
		t.Error("ContainerRemove was not called")
	}
}

func TestIsRunning(t *testing.T) {
	mock := &mockDockerClient{
		containerInspectFunc: func(id string) (types.ContainerJSON, error) {
			return types.ContainerJSON{ContainerJSONBase: &types.ContainerJSONBase{State: &types.ContainerState{Running: true}}}, nil
		},
	}
	mgr := &Manager{cli: mock}
	running, _ := mgr.IsRunning(context.Background(), "id")
	if !running {
		t.Error("Expected IsRunning to be true")
	}
}

func TestUpdateResources(t *testing.T) {
	mgr := &Manager{cli: &mockDockerClient{}}
	err := mgr.UpdateResources(context.Background(), "id", 0.5, 1024)
	if err != nil {
		t.Errorf("UpdateResources failed: %v", err)
	}
}

func TestGetRunnerInfo(t *testing.T) {
	mock := &mockDockerClient{
		containerInspectFunc: func(id string) (types.ContainerJSON, error) {
			return types.ContainerJSON{ContainerJSONBase: &types.ContainerJSONBase{
				State: &types.ContainerState{
					Running:   true,
					StartedAt: time.Now().Add(-10 * time.Minute).Format(time.RFC3339Nano),
				},
			}}, nil
		},
	}
	mgr := &Manager{cli: mock}
	info, err := mgr.GetRunnerInfo(context.Background(), "id")
	if err != nil {
		t.Fatalf("GetRunnerInfo failed: %v", err)
	}
	if info.InternalStatus != "Working" {
		t.Errorf("expected status 'Working', got '%s'", info.InternalStatus)
	}
}
