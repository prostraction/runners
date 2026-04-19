package docker

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/runners/config"
)

type mockDockerClient struct {
	client.APIClient
	containerCreateFunc  func(config *container.Config, hostConfig *container.HostConfig) (container.CreateResponse, error)
	containerStartFunc   func(containerID string) error
	containerStopFunc    func(containerID string) error
	containerRemoveFunc  func(containerID string) error
	containerInspectFunc func(containerID string) (container.InspectResponse, error)
	imagePullFunc        func(ref string) (io.ReadCloser, error)
	containerUpdateFunc  func() error
	containerTopFunc     func() error
}

func (m *mockDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	if m.containerCreateFunc != nil {
		return m.containerCreateFunc(config, hostConfig)
	}
	return container.CreateResponse{ID: "default-id"}, nil
}

func (m *mockDockerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	if m.containerStartFunc != nil {
		return m.containerStartFunc(containerID)
	}
	return nil
}

func (m *mockDockerClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	if m.containerStopFunc != nil {
		return m.containerStopFunc(containerID)
	}
	return nil
}

func (m *mockDockerClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	if m.containerRemoveFunc != nil {
		return m.containerRemoveFunc(containerID)
	}
	return nil
}

func (m *mockDockerClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	if m.containerInspectFunc != nil {
		return m.containerInspectFunc(containerID)
	}
	return container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{State: &container.State{Running: true}}}, nil
}

func (m *mockDockerClient) ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	if m.imagePullFunc != nil {
		return m.imagePullFunc(ref)
	}
	return io.NopCloser(strings.NewReader(`{"status":"Done"}`)), nil
}

func (m *mockDockerClient) ContainerUpdate(ctx context.Context, containerID string, updateConfig container.UpdateConfig) (container.UpdateResponse, error) {
	if m.containerUpdateFunc != nil {
		return container.UpdateResponse{}, m.containerUpdateFunc()
	}
	return container.UpdateResponse{}, nil
}

func (m *mockDockerClient) ContainerTop(ctx context.Context, containerID string, arguments []string) (container.TopResponse, error) {
	if m.containerTopFunc != nil {
		return container.TopResponse{}, m.containerTopFunc()
	}
	return container.TopResponse{Processes: [][]string{{"123", "Runner.Worker"}}}, nil
}

func TestPullImage(t *testing.T) {
	// 1. Success
	mock := &mockDockerClient{
		imagePullFunc: func(ref string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`{"status":"Done"}`)), nil
		},
	}
	mgr := &Manager{cli: mock}
	if err := mgr.PullImage(context.Background()); err != nil {
		t.Errorf("PullImage failed: %v", err)
	}

	// 2. Failure
	mockErr := &mockDockerClient{
		imagePullFunc: func(ref string) (io.ReadCloser, error) {
			return nil, fmt.Errorf("pull error")
		},
	}
	mgrErr := &Manager{cli: mockErr}
	if err := mgrErr.PullImage(context.Background()); err == nil {
		t.Error("expected error when ImagePull fails")
	}
}

func TestStartRunner(t *testing.T) {
	mock := &mockDockerClient{
		containerCreateFunc: func(c *container.Config, h *container.HostConfig) (container.CreateResponse, error) {
			if h.Resources.Memory != 512*1024*1024 {
				t.Errorf("expected 512MB memory limit, got %d", h.Resources.Memory)
			}
			return container.CreateResponse{ID: "test-id"}, nil
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
	err := mgr.StopRunner(context.Background(), "id")
	if err != nil {
		t.Fatalf("StopRunner failed: %v", err)
	}
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
	err := mgr.RemoveRunner(context.Background(), "id")
	if err != nil {
		t.Fatalf("RemoveRunner failed: %v", err)
	}
	if !called {
		t.Error("ContainerRemove was not called")
	}
}

func TestResumeRunner(t *testing.T) {
	called := false
	mock := &mockDockerClient{
		containerStartFunc: func(id string) error {
			called = true
			return nil
		},
	}
	mgr := &Manager{cli: mock}
	err := mgr.ResumeRunner(context.Background(), "id")
	if err != nil {
		t.Fatalf("ResumeRunner failed: %v", err)
	}
	if !called {
		t.Error("ResumeRunner (ContainerStart) was not called")
	}
}

func TestIsRunning(t *testing.T) {
	mgr := &Manager{cli: &mockDockerClient{}}
	running, _ := mgr.IsRunning(context.Background(), "id")
	if !running {
		t.Error("Expected IsRunning to be true")
	}
}

func TestUpdateResources(t *testing.T) {
	// 1. Success
	mgr := &Manager{cli: &mockDockerClient{}}
	err := mgr.UpdateResources(context.Background(), "id", 0.5, 1024)
	if err != nil {
		t.Errorf("UpdateResources failed: %v", err)
	}

	// 2. Failure
	mockErr := &mockDockerClient{
		containerUpdateFunc: func() error { return fmt.Errorf("update fail") },
	}
	mgrErr := &Manager{cli: mockErr}
	if err := mgrErr.UpdateResources(context.Background(), "id", 1, 1); err == nil {
		t.Error("expected error when ContainerUpdate fails")
	}
}

func TestGetRunnerInfo(t *testing.T) {
	mock := &mockDockerClient{
		containerInspectFunc: func(id string) (container.InspectResponse, error) {
			return container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{
				State: &container.State{
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

func TestGetRunnerInfoExtra(t *testing.T) {
	// Test when ContainerTop fails
	mock := &mockDockerClient{
		containerInspectFunc: func(id string) (container.InspectResponse, error) {
			return container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{
				State: &container.State{Running: true, StartedAt: time.Now().Format(time.RFC3339Nano)},
			}}, nil
		},
		containerTopFunc: func() error { return fmt.Errorf("top error") },
	}
	mgr := &Manager{cli: mock}
	info, _ := mgr.GetRunnerInfo(context.Background(), "id")
	if info.InternalStatus != "-" {
		t.Errorf("expected status '-' when top fails, got '%s'", info.InternalStatus)
	}
}

func TestStartRunnerFailures(t *testing.T) {
	ctx := context.Background()
	runner := &config.Runner{Name: "fail"}

	// 1. Create fails
	mock := &mockDockerClient{
		containerCreateFunc: func(c *container.Config, h *container.HostConfig) (container.CreateResponse, error) {
			return container.CreateResponse{}, fmt.Errorf("creation error")
		},
	}
	mgr := &Manager{cli: mock}
	if err := mgr.StartRunner(ctx, runner); err == nil {
		t.Error("expected error when creation fails")
	}

	// 2. Start fails
	mockStartFail := &mockDockerClient{
		containerCreateFunc: func(c *container.Config, h *container.HostConfig) (container.CreateResponse, error) {
			return container.CreateResponse{ID: "id"}, nil
		},
		containerStartFunc: func(id string) error {
			return fmt.Errorf("start error")
		},
	}
	mgrStartFail := &Manager{cli: mockStartFail}
	if err := mgrStartFail.StartRunner(ctx, runner); err == nil {
		t.Error("expected error when start fails")
	}
}

func TestDockerErrors(t *testing.T) {
	mgr := &Manager{cli: &mockDockerClient{
		containerInspectFunc: func(id string) (container.InspectResponse, error) {
			return container.InspectResponse{}, fmt.Errorf("Error response from daemon: No such container")
		},
	}}

	// Test IsRunning with missing container
	running, _ := mgr.IsRunning(context.Background(), "id")
	if running {
		t.Error("expected running to be false for non-existent container")
	}

	// Test GetRunnerInfo with missing container
	info, _ := mgr.GetRunnerInfo(context.Background(), "id")
	if info.IsRunning {
		t.Error("expected info.IsRunning to be false")
	}

	// Test empty container IDs
	_ = mgr.StopRunner(context.Background(), "")
	_ = mgr.RemoveRunner(context.Background(), "")
	running, _ = mgr.IsRunning(context.Background(), "")
	if running {
		t.Error("empty ID should not be running")
	}

	// Test StopRunner error (other than not found)
	mockStopFail := &mockDockerClient{
		containerStopFunc: func(id string) error {
			return fmt.Errorf("stop error")
		},
	}
	mgrStopFail := &Manager{cli: mockStopFail}
	if err := mgrStopFail.StopRunner(context.Background(), "id"); err == nil {
		t.Error("expected error when stop fails")
	}

	// Test RemoveRunner error (other than not found)
	mockRemoveFail := &mockDockerClient{
		containerRemoveFunc: func(id string) error {
			return fmt.Errorf("remove error")
		},
	}
	mgrRemoveFail := &Manager{cli: mockRemoveFail}
	if err := mgrRemoveFail.RemoveRunner(context.Background(), "id"); err == nil {
		t.Error("expected error when remove fails")
	}
}
