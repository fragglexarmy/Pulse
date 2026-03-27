package dockeragent

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/network"
	swarmtypes "github.com/moby/moby/api/types/swarm"
	systemtypes "github.com/moby/moby/api/types/system"
	"github.com/moby/moby/client"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
)

var (
	connectRuntimeFn         = connectRuntime
	hostmetricsCollect       = hostmetrics.Collect
	newTickerFn              = time.NewTicker
	newTimerFn               = time.NewTimer
	randomDurationFn         = randomDuration
	nowFn                    = time.Now
	sleepFn                  = time.Sleep
	jsonMarshalFn            = json.Marshal
	normalizeTargetsFn       = normalizeTargets
	buildRuntimeCandidatesFn = buildRuntimeCandidates
	tryRuntimeCandidateFn    = tryRuntimeCandidate
	randIntFn                = rand.Int
	osExecutableFn           = os.Executable
	osCreateTempFn           = os.CreateTemp
	closeFileFn              = func(f *os.File) error { return f.Close() }
	osRenameFn               = os.Rename
	osChmodFn                = os.Chmod
	osRemoveFn               = os.Remove
	osReadFileFn             = os.ReadFile
	osWriteFileFn            = os.WriteFile
	osStatFn                 = os.Stat
	syscallExecFn            = syscall.Exec
	goArch                   = runtime.GOARCH
	unameMachine             = func() (string, error) {
		out, err := exec.Command("uname", "-m").Output()
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
	machineIDPaths = []string{
		"/etc/machine-id",
		"/var/lib/dbus/machine-id",
	}
	unraidVersionPath       = "/etc/unraid-version"
	unraidPersistPath       = "/boot/config/plugins/pulse-docker-agent/pulse-docker-agent"
	unraidStartupScriptPath = "/boot/config/go.d/pulse-docker-agent.sh"
	agentLogPath            = "/var/log/pulse-docker-agent.log"
	openProcUptime          = func() (io.ReadCloser, error) {
		return os.Open("/proc/uptime")
	}
	newDockerClientFn = func(opts ...client.Opt) (dockerClient, error) {
		raw, err := client.NewClientWithOpts(opts...)
		if err != nil {
			return nil, err
		}
		return &mobyDockerClient{Client: raw}, nil
	}
	selfUpdateFunc = func(a *Agent, ctx context.Context) error {
		return a.selfUpdate(ctx)
	}
	execCommandContextFn = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return exec.CommandContext(ctx, name, arg...)
	}
)

type mobyDockerClient struct {
	*client.Client
}

func (m *mobyDockerClient) Info(ctx context.Context) (systemtypes.Info, error) {
	result, err := m.Client.Info(ctx, client.InfoOptions{})
	if err != nil {
		return systemtypes.Info{}, err
	}
	return result.Info, nil
}

func (m *mobyDockerClient) ContainerStatsOneShot(ctx context.Context, containerID string) (containerStatsResult, error) {
	return m.Client.ContainerStats(ctx, containerID, client.ContainerStatsOptions{})
}

func (m *mobyDockerClient) ContainerInspect(ctx context.Context, containerID string) (containertypes.InspectResponse, error) {
	result, err := m.Client.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return containertypes.InspectResponse{}, err
	}
	return result.Container, nil
}

func (m *mobyDockerClient) ContainerInspectWithRaw(ctx context.Context, containerID string, size bool) (containertypes.InspectResponse, []byte, error) {
	result, err := m.Client.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{Size: size})
	if err != nil {
		return containertypes.InspectResponse{}, nil, err
	}
	return result.Container, result.Raw, nil
}

func (m *mobyDockerClient) ImagePull(ctx context.Context, ref string, options imagePullOptions) (io.ReadCloser, error) {
	return m.Client.ImagePull(ctx, ref, options)
}

func (m *mobyDockerClient) ContainerStop(ctx context.Context, containerID string, options containerStopOptions) error {
	_, err := m.Client.ContainerStop(ctx, containerID, options)
	return err
}

func (m *mobyDockerClient) ContainerStart(ctx context.Context, containerID string, options containerStartOptions) error {
	_, err := m.Client.ContainerStart(ctx, containerID, options)
	return err
}

func (m *mobyDockerClient) ContainerRemove(ctx context.Context, containerID string, options containerRemoveOptions) error {
	_, err := m.Client.ContainerRemove(ctx, containerID, options)
	return err
}

func (m *mobyDockerClient) NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error {
	_, err := m.Client.NetworkConnect(ctx, networkID, client.NetworkConnectOptions{
		Container:      containerID,
		EndpointConfig: config,
	})
	return err
}

func (m *mobyDockerClient) ContainerRename(ctx context.Context, containerID, newName string) error {
	_, err := m.Client.ContainerRename(ctx, containerID, client.ContainerRenameOptions{NewName: newName})
	return err
}

func (m *mobyDockerClient) ImageInspectWithRaw(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
	var raw bytes.Buffer
	result, err := m.Client.ImageInspect(ctx, imageID, client.ImageInspectWithRawResponse(&raw))
	if err != nil {
		return image.InspectResponse{}, nil, err
	}
	return result.InspectResponse, raw.Bytes(), nil
}

func (m *mobyDockerClient) ContainerCreate(ctx context.Context, config *containertypes.Config, hostConfig *containertypes.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (containertypes.CreateResponse, error) {
	result, err := m.Client.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:           config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkingConfig,
		Platform:         platform,
		Name:             containerName,
	})
	if err != nil {
		return containertypes.CreateResponse{}, err
	}
	return containertypes.CreateResponse{ID: result.ID, Warnings: result.Warnings}, nil
}

func (m *mobyDockerClient) ContainerList(ctx context.Context, options containerListOptions) ([]containertypes.Summary, error) {
	result, err := m.Client.ContainerList(ctx, client.ContainerListOptions{
		All:     options.All,
		Filters: options.Filters.toMoby(),
	})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (m *mobyDockerClient) ServiceList(ctx context.Context, options serviceListOptions) ([]swarmtypes.Service, error) {
	result, err := m.Client.ServiceList(ctx, client.ServiceListOptions{
		Status: options.Status,
	})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (m *mobyDockerClient) TaskList(ctx context.Context, options taskListOptions) ([]swarmtypes.Task, error) {
	result, err := m.Client.TaskList(ctx, client.TaskListOptions{
		Filters: options.Filters.toMoby(),
	})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}
