package docker

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"github.com/flexkube/libflexkube/pkg/container/runtime"
	"github.com/flexkube/libflexkube/pkg/container/types"
	"github.com/flexkube/libflexkube/pkg/defaults"
)

// Config struct represents Docker container runtime configuration
type Config struct {
	Host string `json:"host,omitempty" yaml:"host,omitempty"`
}

// docker struct is a struct, which can be used to manage Docker containers
type docker struct {
	ctx context.Context
	cli *client.Client
}

// SetAddress sets runtime config address where it should connect
func (c *Config) SetAddress(s string) {
	c.Host = s
}

// GetAddress returns configured container runtime address
func (c *Config) GetAddress() string {
	if c != nil && c.Host != "" {
		return c.Host
	}

	return client.DefaultDockerHost
}

// New validates Docker runtime configuration and returns configured
// runtime client.
func (c *Config) New() (runtime.Runtime, error) {
	opts := []client.Opt{
		client.WithVersion(defaults.DockerAPIVersion),
	}

	if c != nil && c.Host != "" {
		opts = append(opts, client.WithHost(c.Host))
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating Docker client: %w", err)
	}

	return &docker{
		ctx: context.Background(),
		cli: cli,
	}, nil
}

// Start starts Docker container
func (d *docker) Create(config *types.ContainerConfig) (string, error) {
	// Pull image to make sure it's available.
	// TODO make it configurable?
	out, err := d.cli.ImagePull(d.ctx, config.Image, dockertypes.ImagePullOptions{})
	if err != nil {
		return "", fmt.Errorf("pulling image: %w", err)
	}

	defer out.Close()

	if _, err := io.Copy(ioutil.Discard, out); err != nil {
		return "", fmt.Errorf("failed to pull image: %w", err)
	}

	// TODO That should be validated at ContainerConfig level!
	portBindings := nat.PortMap{}
	exposedPorts := nat.PortSet{}

	for _, ip := range config.Ports {
		port, err := nat.NewPort(ip.Protocol, strconv.Itoa(ip.Port))
		if err != nil {
			return "", fmt.Errorf("failed mapping ports: %w", err)
		}

		if _, exists := portBindings[port]; !exists {
			portBindings[port] = []nat.PortBinding{}
		}

		portBindings[port] = append(portBindings[port], nat.PortBinding{
			HostIP:   ip.IP,
			HostPort: strconv.Itoa(ip.Port),
		})
		exposedPorts[port] = struct{}{}
	}

	// TODO validate that
	mounts := []mount.Mount{}
	for _, m := range config.Mounts {
		mounts = append(mounts, mount.Mount{
			Type:   "bind",
			Source: m.Source,
			Target: m.Target,
			// TODO validate!
			BindOptions: &mount.BindOptions{
				Propagation: mount.Propagation(m.Propagation),
			},
		})
	}

	// Just structs required for starting container.
	dockerConfig := containertypes.Config{
		Image:        config.Image,
		Cmd:          config.Args,
		Entrypoint:   config.Entrypoint,
		ExposedPorts: exposedPorts,
	}
	hostConfig := containertypes.HostConfig{
		Mounts:       mounts,
		PortBindings: portBindings,
		Privileged:   config.Privileged,
		NetworkMode:  containertypes.NetworkMode(config.NetworkMode),
		PidMode:      containertypes.PidMode(config.PidMode),
		IpcMode:      containertypes.IpcMode(config.IpcMode),
	}

	// Create container
	c, err := d.cli.ContainerCreate(d.ctx, &dockerConfig, &hostConfig, &network.NetworkingConfig{}, config.Name)
	if err != nil {
		return "", fmt.Errorf("creating container: %w", err)
	}

	return c.ID, nil
}

// Start starts Docker container
func (d *docker) Start(id string) error {
	return d.cli.ContainerStart(d.ctx, id, dockertypes.ContainerStartOptions{})
}

// Stop stops Docker container
func (d *docker) Stop(id string) error {
	// TODO make this configurable?
	timeout := time.Duration(30) * time.Second
	return d.cli.ContainerStop(d.ctx, id, &timeout)
}

// Status returns container status
func (d *docker) Status(id string) (*types.ContainerStatus, error) {
	status, err := d.cli.ContainerInspect(d.ctx, id)
	if err != nil {
		// If container is missing, return no status
		if client.IsErrNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("inspecting container failed: %w", err)
	}

	return &types.ContainerStatus{
		Image:  status.Image,
		ID:     id,
		Name:   status.Name,
		Status: status.State.Status,
	}, nil
}

// Delete removes the container
func (d *docker) Delete(id string) error {
	return d.cli.ContainerRemove(d.ctx, id, dockertypes.ContainerRemoveOptions{})
}

// Copy copies and extracts TAR archive into container
func (d *docker) Copy(id string, dstPath string, content io.Reader) error {
	return d.cli.CopyToContainer(d.ctx, id, dstPath, content, dockertypes.CopyToContainerOptions{})
}

// Stat check if given path exists on the container. If it is missing, FileMode will be nil
func (d *docker) Stat(id string, path string) (*os.FileMode, error) {
	s, err := d.cli.ContainerStatPath(d.ctx, id, path)
	if err != nil {
		if client.IsErrNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return &s.Mode, nil
}

// Read reads single file from the container and returns it in TAR format
func (d *docker) Read(id string, srcPath string) (io.ReadCloser, error) {
	// TODO check if we should return stat info here
	rc, _, err := d.cli.CopyFromContainer(d.ctx, id, srcPath)
	if err != nil {
		if client.IsErrNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed copying from container: %w", err)
	}

	return rc, nil
}
