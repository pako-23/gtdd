package docker

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	cgo "github.com/compose-spec/compose-go/cli"
	cgotypes "github.com/compose-spec/compose-go/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/strslice"
	log "github.com/sirupsen/logrus"
)

type service struct {
	Command     strslice.StrSlice
	Entrypoint  strslice.StrSlice
	Environment []string
	Image       string
	Healthcheck *container.HealthConfig
	ShmSize     int64
}

type App map[string]*service

func newService(name string, config *cgotypes.ServiceConfig) *service {
	result := service{
		Command:     strslice.StrSlice(config.Command),
		Entrypoint:  strslice.StrSlice(config.Entrypoint),
		Environment: make([]string, len(config.Environment)),
		Image:       config.Image,
		Healthcheck: nil,
		ShmSize:     int64(config.ShmSize),
	}

	if len(result.Image) == 0 {
		result.Image = strings.Join([]string{filepath.Base(config.Build.Context), name}, "-")
	}

	for k, v := range config.Environment {
		if v == nil {
			result.Environment = append(result.Environment, k)
		} else {
			result.Environment = append(result.Environment, strings.Join([]string{k, *v}, "="))
		}
	}

	if config.HealthCheck == nil {
		return &result
	}

	result.Healthcheck = &container.HealthConfig{
		Test: config.HealthCheck.Test,
	}

	if config.HealthCheck.Interval != nil {
		result.Healthcheck.Interval = time.Duration(*config.HealthCheck.Interval)
	}

	if config.HealthCheck.Timeout != nil {
		result.Healthcheck.Timeout = time.Duration(*config.HealthCheck.Timeout)
	}

	if config.HealthCheck.StartPeriod != nil {
		result.Healthcheck.StartPeriod = time.Duration(*config.HealthCheck.StartPeriod)
	}

	if config.HealthCheck.Retries != nil {
		result.Healthcheck.Retries = int(*config.HealthCheck.Retries)
	}

	if config.HealthCheck.Disable {
		result.Healthcheck.Test = []string{"NONE"}
	}

	return &result
}

func (c *Client) pull(ctx context.Context, imageName string) error {
	reader, err := c.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull Docker image: %w", err)
	}
	defer reader.Close()

	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("failed to read Docker image pull logs: %w", err)
	}
	return nil
}

func (c *Client) NewApp(definition string) (App, error) {
	project, err := cgo.ProjectFromOptions(&cgo.ProjectOptions{
		ConfigPaths: []string{definition},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load app definition file: %w", err)
	}

	resultsCh := make(chan instance)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := make(App, len(project.ServiceNames()))
	for _, name := range project.ServiceNames() {
		go func(name string) {
			config, _ := project.GetService(name)

			if len(config.Image) != 0 {
				resultsCh <- instance{service: name, err: c.pull(ctx, config.Image)}
				return
			}
			basename := filepath.Base(config.Build.Context)
			imageName := strings.Join([]string{basename, name}, "-")

			err := c.buildImage(
				ctx,
				imageName,
				config.Build.Context,
				config.Build.Dockerfile,
			)

			resultsCh <- instance{service: name, err: err}
		}(name)
	}

	for i := 0; i < len(project.ServiceNames()); i++ {
		result := <-resultsCh

		if err != nil {
			continue
		} else if result.err != nil {
			cancel()
			err = result.err
			continue
		}

		config, _ := project.GetService(result.service)
		app[result.service] = newService(result.service, &config)
	}

	if err != nil {
		return nil, err
	}

	log.Infof("successfully read Docker Compose file from %s", definition)

	return app, nil
}
