package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
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
	containerUpdateFunc  func(cfg container.UpdateConfig) error
	containerTopFunc     func() error
	containerTopRespFunc func() (container.TopResponse, error)
	containerLogsFunc    func(opts container.LogsOptions) (io.ReadCloser, error)
	containerWaitFunc    func() (<-chan container.WaitResponse, <-chan error)
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
		return container.UpdateResponse{}, m.containerUpdateFunc(updateConfig)
	}
	return container.UpdateResponse{}, nil
}

func (m *mockDockerClient) ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
	if m.containerWaitFunc != nil {
		return m.containerWaitFunc()
	}
	okCh := make(chan container.WaitResponse, 1)
	okCh <- container.WaitResponse{}
	errCh := make(chan error, 1)
	return okCh, errCh
}

func (m *mockDockerClient) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	if m.containerLogsFunc != nil {
		return m.containerLogsFunc(options)
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *mockDockerClient) ContainerTop(ctx context.Context, containerID string, arguments []string) (container.TopResponse, error) {
	if m.containerTopRespFunc != nil {
		return m.containerTopRespFunc()
	}
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

			// Check environment variables
			expectedEnv := map[string]bool{
				"REPO_URL=https://github.com/test/repo":            true,
				"RUNNER_NAME=test":                                 true,
				"RUNNER_TOKEN=token":                               true,
				"CONFIGURED_ACTIONS_RUNNER_FILES_DIR=/runner/data": true,
				"DISABLE_AUTOMATIC_DEREGISTRATION=true":            true,
				"CONFIG_OPTS=--replace":                            true,
			}
			for _, env := range c.Env {
				delete(expectedEnv, env)
			}
			if len(expectedEnv) > 0 {
				t.Errorf("missing environment variables: %v", expectedEnv)
			}

			// Check binds
			foundDataBind := false
			for _, bind := range h.Binds {
				if strings.Contains(bind, ":/runner/data") {
					foundDataBind = true
					break
				}
			}
			if !foundDataBind {
				t.Error("missing runner data bind")
			}

			if h.RestartPolicy.Name != container.RestartPolicyUnlessStopped {
				t.Errorf("expected RestartPolicy unless-stopped, got %q", h.RestartPolicy.Name)
			}

			return container.CreateResponse{ID: "test-id"}, nil
		},
	}

	mgr := &Manager{cli: mock}
	runner := &config.Runner{
		Name:        "test",
		URL:         "https://github.com/test/repo",
		Token:       "token",
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
		containerUpdateFunc: func(container.UpdateConfig) error { return fmt.Errorf("update fail") },
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

	// 2. Start fails — the created-but-never-started container must be
	// force-removed so its name doesn't linger.
	removeCalls := []string{}
	mockStartFail := &mockDockerClient{
		containerCreateFunc: func(c *container.Config, h *container.HostConfig) (container.CreateResponse, error) {
			return container.CreateResponse{ID: "create-id"}, nil
		},
		containerStartFunc: func(id string) error {
			return fmt.Errorf("start error")
		},
		containerRemoveFunc: func(id string) error {
			removeCalls = append(removeCalls, id)
			return nil
		},
	}
	mgrStartFail := &Manager{cli: mockStartFail}
	if err := mgrStartFail.StartRunner(ctx, runner); err == nil {
		t.Error("expected error when start fails")
	}
	if !slices.Contains(removeCalls, "create-id") {
		t.Errorf("expected cleanup of created container 'create-id' after start fail; remove calls = %v", removeCalls)
	}
}

func TestRemoveContainerByName(t *testing.T) {
	var lastID string
	mock := &mockDockerClient{
		containerRemoveFunc: func(id string) error {
			lastID = id
			return nil
		},
	}
	mgr := &Manager{cli: mock}
	if err := mgr.RemoveContainerByName(context.Background(), "foo"); err != nil {
		t.Fatalf("RemoveContainerByName: %v", err)
	}
	if lastID != "gh-runner-foo" {
		t.Errorf("expected gh-runner-foo, got %q", lastID)
	}

	// "No such container" is swallowed.
	mockMissing := &mockDockerClient{
		containerRemoveFunc: func(id string) error {
			return fmt.Errorf("Error: No such container: gh-runner-foo")
		},
	}
	if err := (&Manager{cli: mockMissing}).RemoveContainerByName(context.Background(), "foo"); err != nil {
		t.Errorf("missing container should be swallowed, got %v", err)
	}
}

func TestGetRunnerInfoNotConnected(t *testing.T) {
	// Container running but neither Runner.Listener nor Runner.Worker present
	// means the runner is not actually connected to GitHub (e.g. stale creds
	// after the runner was removed from the GitHub side).
	mock := &mockDockerClient{
		containerInspectFunc: func(id string) (container.InspectResponse, error) {
			return container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{
				State: &container.State{Running: true, StartedAt: time.Now().Format(time.RFC3339Nano)},
			}}, nil
		},
		containerTopRespFunc: func() (container.TopResponse, error) {
			return container.TopResponse{Processes: [][]string{{"1", "sh"}, {"2", "some-other-process"}}}, nil
		},
	}
	mgr := &Manager{cli: mock}
	info, err := mgr.GetRunnerInfo(context.Background(), "id")
	if err != nil {
		t.Fatalf("GetRunnerInfo failed: %v", err)
	}
	if info.InternalStatus != "Not Connected" {
		t.Errorf("expected 'Not Connected', got %q", info.InternalStatus)
	}
}

func TestGetRunnerInfoIdle(t *testing.T) {
	// Runner.Listener running but no Worker = Idle.
	mock := &mockDockerClient{
		containerInspectFunc: func(id string) (container.InspectResponse, error) {
			return container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{
				State: &container.State{Running: true, StartedAt: time.Now().Format(time.RFC3339Nano)},
			}}, nil
		},
		containerTopRespFunc: func() (container.TopResponse, error) {
			return container.TopResponse{Processes: [][]string{{"1", "Runner.Listener"}}}, nil
		},
	}
	mgr := &Manager{cli: mock}
	info, _ := mgr.GetRunnerInfo(context.Background(), "id")
	if info.InternalStatus != "Idle" {
		t.Errorf("expected 'Idle', got %q", info.InternalStatus)
	}
}

func TestEnsureRestartPolicy(t *testing.T) {
	var gotPolicy string
	mock := &mockDockerClient{
		containerUpdateFunc: func(cfg container.UpdateConfig) error {
			gotPolicy = string(cfg.RestartPolicy.Name)
			return nil
		},
	}
	mgr := &Manager{cli: mock}
	if err := mgr.EnsureRestartPolicy(context.Background(), "id"); err != nil {
		t.Fatalf("EnsureRestartPolicy failed: %v", err)
	}
	if gotPolicy != string(container.RestartPolicyUnlessStopped) {
		t.Errorf("expected unless-stopped, got %q", gotPolicy)
	}

	// Empty ID is a no-op.
	if err := mgr.EnsureRestartPolicy(context.Background(), ""); err != nil {
		t.Errorf("expected no error for empty id, got %v", err)
	}

	// "No such container" is swallowed (cleanup path).
	mockMissing := &mockDockerClient{
		containerUpdateFunc: func(cfg container.UpdateConfig) error {
			return fmt.Errorf("Error response from daemon: No such container: id")
		},
	}
	mgrMissing := &Manager{cli: mockMissing}
	if err := mgrMissing.EnsureRestartPolicy(context.Background(), "id"); err != nil {
		t.Errorf("expected missing container error to be swallowed, got %v", err)
	}

	// Other errors propagate.
	mockErr := &mockDockerClient{
		containerUpdateFunc: func(cfg container.UpdateConfig) error {
			return fmt.Errorf("boom")
		},
	}
	mgrErr := &Manager{cli: mockErr}
	if err := mgrErr.EnsureRestartPolicy(context.Background(), "id"); err == nil {
		t.Error("expected error to propagate")
	}
}

func TestVerifyStartup(t *testing.T) {
	// Success: listener shows up on first poll.
	mockOK := &mockDockerClient{
		containerInspectFunc: func(id string) (container.InspectResponse, error) {
			return container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{
				State: &container.State{Running: true},
			}}, nil
		},
		containerTopRespFunc: func() (container.TopResponse, error) {
			return container.TopResponse{Processes: [][]string{{"1", "Runner.Listener"}}}, nil
		},
	}
	mgrOK := &Manager{cli: mockOK}
	if err := mgrOK.VerifyStartup(context.Background(), "id", 2*time.Second); err != nil {
		t.Errorf("VerifyStartup should succeed when Listener is present: %v", err)
	}

	// Failure: container crashed (not running, non-zero exit).
	mockCrash := &mockDockerClient{
		containerInspectFunc: func(id string) (container.InspectResponse, error) {
			return container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{
				State: &container.State{Running: false, ExitCode: 1},
			}}, nil
		},
	}
	mgrCrash := &Manager{cli: mockCrash}
	if err := mgrCrash.VerifyStartup(context.Background(), "id", 2*time.Second); err == nil {
		t.Error("expected error when container exited")
	}

	// Failure: running but no Listener within timeout.
	mockNoListener := &mockDockerClient{
		containerInspectFunc: func(id string) (container.InspectResponse, error) {
			return container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{
				State: &container.State{Running: true},
			}}, nil
		},
		containerTopRespFunc: func() (container.TopResponse, error) {
			return container.TopResponse{Processes: [][]string{{"1", "sh"}}}, nil
		},
	}
	mgrNoListener := &Manager{cli: mockNoListener}
	if err := mgrNoListener.VerifyStartup(context.Background(), "id", 100*time.Millisecond); err == nil {
		t.Error("expected timeout error when Listener never appears")
	}

	// Failure: empty container ID.
	if err := (&Manager{cli: &mockDockerClient{}}).VerifyStartup(context.Background(), "", time.Second); err == nil {
		t.Error("expected error for empty container ID")
	}
}

func TestPurgeDataDir(t *testing.T) {
	// Missing data dir: no-op, no container API is called.
	tmpDir, err := os.MkdirTemp("", "runners-purge")
	if err != nil {
		t.Fatalf("mkdirtemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	origConfigDir := config.ConfigDir
	config.ConfigDir = tmpDir
	defer func() { config.ConfigDir = origConfigDir }()

	createCalled := false
	mock := &mockDockerClient{
		containerCreateFunc: func(c *container.Config, h *container.HostConfig) (container.CreateResponse, error) {
			createCalled = true
			return container.CreateResponse{ID: "cleanup-id"}, nil
		},
	}
	mgr := &Manager{cli: mock}
	if err := mgr.PurgeDataDir(context.Background(), "nope"); err != nil {
		t.Errorf("expected no error for missing data dir, got %v", err)
	}
	if createCalled {
		t.Error("did not expect cleanup container when data dir is missing")
	}

	// Existing data dir: create cleanup container with the right bind + entrypoint,
	// then host-side remove the (now-empty) dir.
	dataDir := config.DataDir("rn")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	var gotBinds []string
	var gotEntrypoint []string
	var gotCmd []string
	mock2 := &mockDockerClient{
		containerCreateFunc: func(c *container.Config, h *container.HostConfig) (container.CreateResponse, error) {
			gotBinds = h.Binds
			gotEntrypoint = c.Entrypoint
			gotCmd = c.Cmd
			if !h.AutoRemove {
				t.Error("expected AutoRemove=true on cleanup container")
			}
			return container.CreateResponse{ID: "cleanup-id"}, nil
		},
	}
	mgr2 := &Manager{cli: mock2}
	if err := mgr2.PurgeDataDir(context.Background(), "rn"); err != nil {
		t.Fatalf("PurgeDataDir failed: %v", err)
	}

	if len(gotBinds) != 1 || !strings.Contains(gotBinds[0], ":/data") {
		t.Errorf("unexpected binds: %v", gotBinds)
	}
	if len(gotEntrypoint) == 0 || gotEntrypoint[0] != "sh" {
		t.Errorf("expected sh entrypoint, got %v", gotEntrypoint)
	}
	if len(gotCmd) == 0 || !strings.Contains(gotCmd[0], "find /data") {
		t.Errorf("expected find-based cleanup, got %v", gotCmd)
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Errorf("expected dataDir to be removed, stat err = %v", err)
	}

	// ContainerCreate error falls back to host removal.
	dataDir2 := config.DataDir("fallback")
	if err := os.MkdirAll(dataDir2, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}
	mockFail := &mockDockerClient{
		containerCreateFunc: func(c *container.Config, h *container.HostConfig) (container.CreateResponse, error) {
			return container.CreateResponse{}, fmt.Errorf("create fail")
		},
	}
	mgrFail := &Manager{cli: mockFail}
	if err := mgrFail.PurgeDataDir(context.Background(), "fallback"); err != nil {
		t.Errorf("expected host-side fallback to succeed: %v", err)
	}
	if _, err := os.Stat(dataDir2); !os.IsNotExist(err) {
		t.Errorf("expected fallback dataDir to be removed, stat err = %v", err)
	}

	// "No such container" from Wait (AutoRemove race) is treated as success.
	dataDir3 := config.DataDir("race")
	if err := os.MkdirAll(dataDir3, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}
	mockRace := &mockDockerClient{
		containerWaitFunc: func() (<-chan container.WaitResponse, <-chan error) {
			errCh := make(chan error, 1)
			errCh <- fmt.Errorf("Error response from daemon: No such container: cleanup-id")
			okCh := make(chan container.WaitResponse)
			return okCh, errCh
		},
	}
	mgrRace := &Manager{cli: mockRace}
	if err := mgrRace.PurgeDataDir(context.Background(), "race"); err != nil {
		t.Errorf("AutoRemove-race should be swallowed, got: %v", err)
	}
}

func TestStreamLogs(t *testing.T) {
	// Empty container ID is rejected.
	mgr := &Manager{cli: &mockDockerClient{}}
	if err := mgr.StreamLogs(context.Background(), "", false, "100", io.Discard, io.Discard); err == nil {
		t.Error("expected error for empty container id")
	}

	// Options are forwarded and an empty stream returns no error.
	var gotOpts container.LogsOptions
	mock := &mockDockerClient{
		containerLogsFunc: func(opts container.LogsOptions) (io.ReadCloser, error) {
			gotOpts = opts
			return io.NopCloser(strings.NewReader("")), nil
		},
	}
	mgrOK := &Manager{cli: mock}
	if err := mgrOK.StreamLogs(context.Background(), "id", true, "50", io.Discard, io.Discard); err != nil {
		t.Fatalf("StreamLogs failed: %v", err)
	}
	if !gotOpts.ShowStdout || !gotOpts.ShowStderr || !gotOpts.Follow || gotOpts.Tail != "50" {
		t.Errorf("unexpected options forwarded: %+v", gotOpts)
	}

	// API error propagates.
	mockErr := &mockDockerClient{
		containerLogsFunc: func(opts container.LogsOptions) (io.ReadCloser, error) {
			return nil, fmt.Errorf("logs error")
		},
	}
	mgrErr := &Manager{cli: mockErr}
	if err := mgrErr.StreamLogs(context.Background(), "id", false, "100", io.Discard, io.Discard); err == nil {
		t.Error("expected error when ContainerLogs fails")
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
