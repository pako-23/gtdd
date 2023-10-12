package compose

import (
	"sync"

	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

type HealthCheck struct {
	Test        []string      `yaml:"test"`
	Interval    time.Duration `yaml:"interval"`
	Timeout     time.Duration `yaml:"timeout"`
	Retries     int           `yaml:"retries"`
	StartPeriod time.Duration `yaml:"start_period"`
}

// A Service represents a container as it is defined inside a Docker Compose
// definition file.
type Service struct {
	// The command to run inside the Docker container.
	Cmd []string `yaml:"command"`
	// Overrides the default ENTRYPOINT for the image needed to run the service.
	Entrypoint []string `yaml:"entrypoint"`
	// Sets the environment variables inside the container tu run the service.
	Env []string `yaml:"env"`
	// The Docker image used to create the container to run the service.
	Image       string       `yaml:"image"`
	HealthCheck *HealthCheck `yaml:"healthcheck"`
}

type serviceInstance struct {
	ServiceName string
	ContainerID string
	Error       error
}

// create creates the Docker container needed to run the Service. The image
// should already be pulled on the host. If there is any error in creating the
// Docker container, it is returned.
func (s *Service) create(ctx context.Context, name string) (string, error) {
	cli := ctx.Value("client").(*client.Client)

	config := container.Config{
		Cmd:        s.Cmd,
		Entrypoint: s.Entrypoint,
		Env:        s.Env,
		Image:      s.Image,
	}
	if s.HealthCheck != nil {
		config.Healthcheck = &container.HealthConfig{
			Test:        s.HealthCheck.Test,
			Interval:    s.HealthCheck.Interval,
			Timeout:     s.HealthCheck.Timeout,
			StartPeriod: s.HealthCheck.StartPeriod,
			Retries:     s.HealthCheck.Retries,
		}
	}

	res, err := cli.ContainerCreate(ctx, &config, nil, nil, nil, name)
	if err != nil {
		return "", fmt.Errorf("container creation with config %v failed: %w", config, err)
	}
	log.Debugf("created container %s with config: %v", res.ID, config)

	return res.ID, nil
}

func (s *Service) run(ctx context.Context, ch chan<- serviceInstance, n *sync.WaitGroup) {
	defer n.Done()
	var (
		config = ctx.Value("start-config").(*StartConfig)
		cli    = ctx.Value("client").(*client.Client)
		name   = ctx.Value("service-name").(string)
	)

	containerID, err := s.create(ctx, config.name(name))
	if err != nil {
		ch <- serviceInstance{ContainerID: "", ServiceName: name, Error: err}

		return
	}

	if err := config.start(ctx, cli, containerID); err != nil {
		cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{})
		ch <- serviceInstance{ContainerID: "", ServiceName: name, Error: err}

		return
	}

	if s.HealthCheck != nil {
		log.Debug("Sleeping for %v", s.HealthCheck.StartPeriod)
		time.Sleep(s.HealthCheck.StartPeriod)
	}

	for {

		if running, err := checkRunning(ctx, cli, containerID); err != nil {
			cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{})
			ch <- serviceInstance{ContainerID: "", ServiceName: name, Error: err}

			return
		} else if running {
			break
		}

		if s.HealthCheck != nil {
			log.Debug("Sleeping for %v", s.HealthCheck.Interval)
			time.Sleep(s.HealthCheck.Interval)
		}
	}

	ch <- serviceInstance{ContainerID: containerID, ServiceName: name, Error: nil}
}

func checkRunning(ctx context.Context, cli *client.Client, containerID string) (bool, error) {
	stats, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, err
	}

	if stats.Config.Healthcheck != nil {
		if stats.State.Health.FailingStreak >= stats.Config.Healthcheck.Retries {
			return false, fmt.Errorf("container healthcheck failing: %v", stats.State.Health.Log)
		}

		log.Debugf("Container in status %s", stats.State.Health.Status)
		return stats.State.Health.Status == "healthy", nil
	}

	switch stats.State.Status {
	case "paused", "restarting", "removing", "exited", "dead":
		return false, fmt.Errorf("the container is into state: %s", stats.State.Status)
	default:
		return stats.State.Status == "running", nil
	}
}

// pull downloads the Docker image needed to run the Service from the registry.
// If there is any error in pulling the Docker image, it is returned.
func (s *Service) pull(ctx context.Context) error {
	cli := ctx.Value("client").(*client.Client)
	log.Debugf("pulling image %s", s.Image)
	reader, err := cli.ImagePull(ctx, s.Image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s when pulling service image: %w", s.Image, err)
	}
	defer reader.Close()

	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("failed to pull image %s when pulling service image: %w", s.Image, err)
	}
	log.Debugf("successfully pulled image %s", s.Image)
	return nil
}
