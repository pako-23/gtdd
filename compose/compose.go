// Copyright 2023 The GTDD Authors. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

// Load the services defined into a Docker Compose definition file to run them
// as a set of Docker containers.

package compose

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

// An App represents a collection of services with their names as they
// are defined inside a Docker Compose definition file.
type App struct {
	// The service with their names as they are defined inside the Docker
	// Compose definition file.
	Services map[string]*Service `yaml:"services"`
}

// An AppConfig represents the configuration needed to load an App from a
// Docker Compose definition file.
type AppConfig struct {
	// The path to where the Docker Compose definition file is located.
	Path string
	// The name of the file containing of the Docker Compose definition file.
	ComposeFile string
}

// An AppInstance represents a collection of running containers with the
// names of the services of the App defining them.
type AppInstance map[string]string

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
	Image string `yaml:"image"`
}

// The StartConfig represents the configurations to apply to an App when
// creating an instance of it.
type StartConfig struct {
	// A context to apply when assigning names to containers.
	// If empty, names are assigned randomly to containers.
	Context string
	// The additional networks to which the containers of an App should be
	// attached to. If empty, the container is not attached to any additional
	// network.
	Networks []string
}

// NewApp load an App from a Docker Compose definition file.
// If there is any error in reading or parsing the definition file,
// it is returned.
func NewApp(config *AppConfig) (App, error) {
	var (
		app            App
		definitionFile = filepath.Join(config.Path, config.ComposeFile)
	)

	data, err := os.ReadFile(definitionFile)
	if err != nil {
		return App{}, fmt.Errorf("failed to read app definition file: %w", err)
	}

	if err := yaml.Unmarshal(data, &app); err != nil {
		return App{}, fmt.Errorf("failed to load app definition file: %w", err)
	}

	log.Infof("successfully read Docker Compose file from %s", definitionFile)

	return app, nil
}

// Pull downloads all the Docker images needed to run some App.
// If there is any error in downloading the images, it is returned.
func (a *App) Pull() error {
	var (
		g   errgroup.Group
		ctx = context.Background()
	)

	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create client to pull app images: %w", err)
	}
	defer cli.Close()

	log.Debugf("starting pulling Docker images for app")

	for _, s := range a.Services {
		func(s *Service) {
			g.Go(func() error {
				return s.pull(ctx, cli)
			})
		}(s)
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("failed to pull images for app: %w", err)
	}

	log.Infof("successfully pulled Docker images for app")

	return nil
}

// Start creates, configures and starts all the services needed to run some App
// as Docker containers. It returns the instance of a running application. If
// there is any error, it is returned.
func (a *App) Start(config *StartConfig) (AppInstance, error) {
	var (
		ctx = context.Background()
		app = AppInstance{}
	)

	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create client to start app: %w", err)
	}
	defer cli.Close()

	log.Debugf("starting containers for app")

	for name, service := range a.Services {
		containerID, err := service.create(ctx, cli, config.name(name))
		if err != nil {
			return nil, err
		}

		if err := config.apply(ctx, cli, containerID); err != nil {
			if deleteErr := app.Delete(); deleteErr != nil {
				log.Error(deleteErr)
			}

			return nil, err
		}

		err = cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
		if err != nil {
			if deleteErr := app.Delete(); deleteErr != nil {
				log.Error(deleteErr)
			}

			return nil, err
		}

		app[name] = containerID
		log.Debugf("successfully started service %s as container %s", name, containerID)
	}

	log.Debugf("successfully started all containers for app")

	return app, nil
}

// Delete will release all the resources needed to run an App. It will delete
// all the Docker containers it started. If there is any error, it is returned.
func (a *AppInstance) Delete() error {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create client to delete app resources: %w", err)
	}
	defer cli.Close()

	for name, containerID := range *a {
		err := cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
			Force: true,
		})
		if err != nil {
			return fmt.Errorf("container for service %s deletion failed: %w", name, err)
		}
		delete(*a, name)
		log.Debugf("successfully deleted container %s for service %s", containerID, name)
	}
	return nil
}

// create creates the Docker container needed to run the Service. The image
// should already be pulled on the host. If there is any error in creating the
// Docker container, it is returned.
func (s *Service) create(ctx context.Context, cli *client.Client, name string) (string, error) {
	config := container.Config{
		Cmd:        s.Cmd,
		Entrypoint: s.Entrypoint,
		Env:        s.Env,
		Image:      s.Image,
	}
	res, err := cli.ContainerCreate(ctx, &config, nil, nil, nil, name)
	if err != nil {
		return "", fmt.Errorf("container creation with config %v failed: %w", config, err)
	}
	log.Debugf("created container %s with config: %v", res.ID, config)

	return res.ID, nil
}

// pull downloads the Docker image needed to run the Service from the registry.
// If there is any error in pulling the Docker image, it is returned.
func (s *Service) pull(ctx context.Context, cli *client.Client) error {
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

// apply adds the declared start configurations to a given
// container. If there is any error, it is returned.
func (s *StartConfig) apply(ctx context.Context, cli *client.Client, containerID string) error {
	for _, net := range s.Networks {
		err := cli.NetworkConnect(ctx, net, containerID, &network.EndpointSettings{})
		if err != nil {
			return fmt.Errorf("failed to connect container %s to %s when applying startup config: %w", containerID, net, err)
		}
		log.Debugf("successfully connected container %s to %s", containerID, net)
	}
	log.Debugf("successfully applied all start up configurations on container %s", containerID)

	return nil
}

// name returns the name of the Docker container associated to a service based
// on the context passed into the StartConfig.
func (s *StartConfig) name(name string) string {
	if s.Context == "" {
		return ""
	}
	return fmt.Sprintf("%s-%s", name, s.Context)
}
