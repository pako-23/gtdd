package docker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	log "github.com/sirupsen/logrus"
)

type RunOptions struct {
	Prefix   string
	Networks []string
}

type instance struct {
	service     string
	containerID string
	err         error
}

type AppInstance map[string]string

func (c *Client) create(ctx context.Context, name string, config *service) (string, error) {
	var (
		containerConfig = &container.Config{
			Cmd:         config.Command,
			Entrypoint:  config.Entrypoint,
			Env:         config.Environment,
			Image:       config.Image,
			Healthcheck: config.Healthcheck,
		}
		hostConfig = &container.HostConfig{
			ShmSize: config.ShmSize,
		}
	)

	res, err := c.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, name)
	if err != nil {
		return "", fmt.Errorf("failed to create Docker container: %w", err)
	}
	log.Debugf("started container with config: %v", containerConfig)

	return res.ID, nil
}

func (c *Client) isRunning(ctx context.Context, containerID string) (bool, error) {
	stats, err := c.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, fmt.Errorf("failed to inspect Docker container: %w", err)
	}

	if stats.Config.Healthcheck == nil {
		switch stats.State.Status {
		case "exited":
			if stats.State.ExitCode == 0 {
				return true, nil
			}

			return false, fmt.Errorf("the container exited with status code %d", stats.State.ExitCode)
		case "paused", "restarting", "removing", "dead":
			return false, fmt.Errorf("the container is into state: %s", stats.State.Status)
		default:
			return stats.State.Status == "running", nil
		}

	}

	if stats.State.Health.Status == "healthy" {
		return true, nil
	}

	if stats.State.Health.FailingStreak >= stats.Config.Healthcheck.Retries {
		logs := make([]string, len(stats.State.Health.Log))

		for i, log := range stats.State.Health.Log {
			logs[i] = log.Output
		}

		return false, fmt.Errorf("container healthcheck failing: %s", strings.Join(logs, " "))
	}

	return false, nil
}

func (c *Client) Run(app App, config RunOptions) (AppInstance, error) {
	result := make(AppInstance, len(app))
	ch := make(chan instance)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cleanup := func(containerID string) {
		err := c.client.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})
		if err != nil {
			log.Errorf("failed to delete failed Docker container: %v", err)
		}
	}

	for name, srv := range app {
		go func(name string, srv *service) {
			var containerName string

			if config.Prefix == "" {
				containerName = name
			} else {
				containerName = strings.Join([]string{config.Prefix, name}, "-")
			}

			containerID, err := c.create(ctx, containerName, srv)
			if err != nil {
				ch <- instance{err: err}

				return
			}

			for _, net := range config.Networks {
				if err := c.client.NetworkConnect(ctx, net, containerID, &network.EndpointSettings{}); err != nil {
					cleanup(containerID)
					ch <- instance{err: err}

					return
				}

			}

			if err := c.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
				cleanup(containerID)
				ch <- instance{err: err}

				return
			}

			if srv.Healthcheck != nil {
				time.Sleep(time.Duration(srv.Healthcheck.StartPeriod))
			}

			for {
				if running, err := c.isRunning(ctx, containerID); err != nil {
					cleanup(containerID)
					ch <- instance{err: err}

					return
				} else if running {
					break
				}

				if srv.Healthcheck != nil {
					time.Sleep(time.Duration(srv.Healthcheck.Interval))
				}
			}

			ch <- instance{err: nil, service: name, containerID: containerID}
		}(name, srv)
	}

	var runErr error = nil

	for i := 0; i < len(app); i++ {
		stat := <-ch
		if runErr == nil && stat.err != nil {
			cancel()
			runErr = stat.err
			continue
		}
		result[stat.service] = stat.containerID
	}

	if runErr != nil {
		c.Delete(result)

		return nil, runErr
	}

	return result, nil
}
