// Copyright 2023 The GTDD Authors. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

// Load the services defined into a Docker Compose definition file to run them
// as a set of Docker containers.

package compose

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/docker/docker/api/types"
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
	var g errgroup.Group

	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create client to pull app images: %w", err)
	}
	defer cli.Close()
	log.Debugf("starting pulling Docker images for app")

	ctx := context.WithValue(context.Background(), "client", cli)
	for _, s := range a.Services {
		func(s *Service) {
			g.Go(func() error {
				return s.pull(ctx)
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
		app = AppInstance{}
		n   = sync.WaitGroup{}
		ch  = make(chan serviceInstance)
	)

	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create client to start app: %w", err)
	}
	defer cli.Close()
	log.Debugf("starting containers for app")

	ctx := context.WithValue(
		context.WithValue(context.Background(), "client", cli),
		"start-config",
		config,
	)
	for name, service := range a.Services {
		n.Add(1)

		go service.run(context.WithValue(ctx, "service-name", name), ch, &n)
	}

	go func() {
		n.Wait()
		close(ch)
	}()

	for service := range ch {
		if service.Error != nil {
			if deleteErr := app.Delete(); deleteErr != nil {
				log.Error(deleteErr)
			}
			return nil, fmt.Errorf("failed to start container for service %s: %w", service.ServiceName, service.Error)
		} else {
			log.Debugf("successfully started service %s as container %s", service.ServiceName, service.ContainerID)
			app[service.ServiceName] = service.ContainerID
		}
	}
	log.Debugf("successfully started all containers for app")

	return app, nil
}

// apply adds the declared start configurations to a given
// container. If there is any error, it is returned.
func (s *StartConfig) start(ctx context.Context, cli *client.Client, containerID string) error {
	for _, net := range s.Networks {
		err := cli.NetworkConnect(ctx, net, containerID, &network.EndpointSettings{})
		if err != nil {
			return fmt.Errorf("failed to connect container %s to %s when applying startup config: %w", containerID, net, err)
		}
		log.Debugf("successfully connected container %s to %s", containerID, net)
	}
	log.Debugf("successfully applied all start up configurations on container %s", containerID)

	return cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
}

// name returns the name of the Docker container associated to a service based
// on the context passed into the StartConfig.
func (s *StartConfig) name(name string) string {
	if s.Context == "" {
		return ""
	}
	return fmt.Sprintf("%s-%s", name, s.Context)
}
