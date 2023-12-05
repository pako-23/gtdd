package compose

import (
	"path/filepath"
	"strings"
	"sync"

	"context"
	"fmt"
	"io"
	"time"

	cgo "github.com/compose-spec/compose-go/types"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

// A Service represents a container as it is defined inside a Docker Compose
// definition file.
type Service cgo.ServiceConfig

type serviceInstance struct {
	ServiceName string
	ContainerID string
	Error       error
}

func toDockerEnv(environment cgo.MappingWithEquals) []string {
	var env []string
	for k, v := range environment {
		if v == nil {
			env = append(env, k)
		} else {
			env = append(env, fmt.Sprintf("%s=%s", k, *v))
		}
	}
	return env
}

func toDockerHealthCheck(check *cgo.HealthCheckConfig) *container.HealthConfig {
	if check == nil {
		return nil
	}
	var (
		interval time.Duration
		timeout  time.Duration
		period   time.Duration
		retries  int
	)
	if check.Interval != nil {
		interval = time.Duration(*check.Interval)
	}
	if check.Timeout != nil {
		timeout = time.Duration(*check.Timeout)
	}
	if check.StartPeriod != nil {
		period = time.Duration(*check.StartPeriod)
	}
	if check.Retries != nil {
		retries = int(*check.Retries)
	}
	test := check.Test
	if check.Disable {
		test = []string{"NONE"}
	}
	return &container.HealthConfig{
		Test:        test,
		Interval:    interval,
		Timeout:     timeout,
		StartPeriod: period,
		Retries:     retries,
	}
}

// create creates the Docker container needed to run the Service. The image
// should already be pulled on the host. If there is any error in creating the
// Docker container, it is returned.
func (s *Service) create(ctx context.Context, name string) (string, error) {
	imageName := s.Image
	if len(imageName) == 0 {
		s.Build.Context = strings.Join([]string{filepath.Base(s.Build.Context), name}, "-")
	}

	var (
		cli             *client.Client    = ctx.Value("client").(*client.Client)
		containerConfig *container.Config = &container.Config{
			Cmd:         strslice.StrSlice(s.Command),
			Entrypoint:  strslice.StrSlice(s.Entrypoint),
			Env:         toDockerEnv(s.Environment),
			Image:       imageName,
			Healthcheck: toDockerHealthCheck(s.HealthCheck),
		}
		hostConfig *container.HostConfig = &container.HostConfig{
			ShmSize: int64(s.ShmSize),
		}
	)

	res, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, name)
	if err != nil {
		return "", fmt.Errorf("container creation with config %v failed: %w", containerConfig, err)
	}
	log.Debugf("created container %s with config: %v", res.ID, containerConfig)

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
		cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})
		ch <- serviceInstance{ContainerID: "", ServiceName: name, Error: err}

		return
	}

	if s.HealthCheck != nil {
		log.Debugf("sleeping for %v", s.HealthCheck.StartPeriod)
		time.Sleep(time.Duration(*s.HealthCheck.StartPeriod))
	}

	for {

		if running, err := checkRunning(ctx, cli, containerID); err != nil {
			cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})
			ch <- serviceInstance{ContainerID: "", ServiceName: name, Error: err}

			return
		} else if running {
			break
		}

		if s.HealthCheck != nil {
			log.Debugf("sleeping for %v", s.HealthCheck.Interval)
			time.Sleep(time.Duration(*s.HealthCheck.Interval))
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
			logs := []string{}
			for _, log := range stats.State.Health.Log {
				logs = append(logs, log.Output)
			}

			return false, fmt.Errorf("container healthcheck failing: %s", strings.Join(logs, " "))
		}

		log.Debugf("container in status %s", stats.State.Health.Status)
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
