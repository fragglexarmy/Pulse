package dockeragent

import (
	"context"
	"io"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/network"
	swarmtypes "github.com/moby/moby/api/types/swarm"
	systemtypes "github.com/moby/moby/api/types/system"
	mobyclient "github.com/moby/moby/client"
	"github.com/opencontainers/image-spec/specs-go/v1"
)

type dockerFilters map[string]map[string]bool

func newDockerFilters() dockerFilters {
	return make(dockerFilters)
}

func (f dockerFilters) Add(term string, values ...string) dockerFilters {
	if f == nil {
		return newDockerFilters().Add(term, values...)
	}
	if _, ok := f[term]; !ok {
		f[term] = make(map[string]bool)
	}
	for _, value := range values {
		f[term][value] = true
	}
	return f
}

func (f dockerFilters) Get(term string) []string {
	values := f[term]
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	return out
}

func (f dockerFilters) Len() int {
	total := 0
	for _, values := range f {
		total += len(values)
	}
	return total
}

func (f dockerFilters) toMoby() mobyclient.Filters {
	if len(f) == 0 {
		return nil
	}
	out := make(mobyclient.Filters, len(f))
	for term, values := range f {
		inner := make(map[string]bool, len(values))
		for value, ok := range values {
			inner[value] = ok
		}
		out[term] = inner
	}
	return out
}

type containerListOptions struct {
	All     bool
	Filters dockerFilters
}

type serviceListOptions struct {
	Status bool
}

type taskListOptions struct {
	Filters dockerFilters
}

type containerStatsResult = mobyclient.ContainerStatsResult

type imagePullOptions = mobyclient.ImagePullOptions

type containerStopOptions = mobyclient.ContainerStopOptions

type containerStartOptions = mobyclient.ContainerStartOptions

type containerRemoveOptions = mobyclient.ContainerRemoveOptions

type dockerClient interface {
	Info(ctx context.Context) (systemtypes.Info, error)
	DaemonHost() string
	ContainerList(ctx context.Context, options containerListOptions) ([]containertypes.Summary, error)
	ContainerInspectWithRaw(ctx context.Context, containerID string, size bool) (containertypes.InspectResponse, []byte, error)
	ContainerStatsOneShot(ctx context.Context, containerID string) (containerStatsResult, error)
	ContainerInspect(ctx context.Context, containerID string) (containertypes.InspectResponse, error)
	ImagePull(ctx context.Context, ref string, options imagePullOptions) (io.ReadCloser, error)
	ContainerStop(ctx context.Context, containerID string, options containerStopOptions) error
	ContainerRename(ctx context.Context, containerID, newName string) error
	ContainerCreate(ctx context.Context, config *containertypes.Config, hostConfig *containertypes.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (containertypes.CreateResponse, error)
	NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error
	ContainerStart(ctx context.Context, containerID string, options containerStartOptions) error
	ContainerRemove(ctx context.Context, containerID string, options containerRemoveOptions) error
	ServiceList(ctx context.Context, options serviceListOptions) ([]swarmtypes.Service, error)
	TaskList(ctx context.Context, options taskListOptions) ([]swarmtypes.Task, error)
	ImageInspectWithRaw(ctx context.Context, imageID string) (image.InspectResponse, []byte, error)
	Close() error
}
