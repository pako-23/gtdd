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

// A single service declared into a Docker Compose definition file.
type Service struct {
	// The Docker command to run.
	Cmd []string `yaml:"command"`
	// The list of services on which the service depends on.
	DependsOn []string `yaml:"depends_on"`
	// The entrypoint to start the service container.
	Entrypoint []string `yaml:"entrypoint"`
	// The environment to pass into the service container.
	Env []string `yaml:"env"`
	// The Docker image to run the service container
	Image string `yaml:"image"`
}

// The configuration to create an App.
type AppConfig struct {
	// The path to where the Docker Compose definition file is located.
	Path string
	// The filename of the Docker Compose definition file.
	ComposeFile string
}

// The definition of all the services needed to run an App as
// they are declared inside a Docker Compose definition file.
type App struct {
	// The services defined in the definition file.
	Services map[string]*Service `yaml:"services"`
}

// The configurations to apply to a container before starting it
type StartConfig struct {
	// The context to use when naming containers. If empty, names are assigned
	// randomly to containers.
	Context string
	// The additional networks to which the container should be attached.
	// If empty, the container is not attached to any additional network.
	Networks []string
}

// An instance of a running application. It contains a mapping from service
// name to Docker container IDs.
type AppInstance map[string]string

// pull downloads the Docker image for the service. If there is any error
// in pulling the Docker image, the error is returned.
func (s *Service) pull(ctx context.Context, cli *client.Client) error {
	reader, err := cli.ImagePull(ctx, s.Image, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()
	// TODO: Check that pull was successful
	io.Copy(io.Discard, reader)
	return nil
}

// create creates the Docker container for a service. The image should already
// be pulled. If there is any error in creating the Docker container, it is
// returned.
func (s *Service) create(ctx context.Context, cli *client.Client, name string) (string, error) {
	config := container.Config{
		Cmd:        s.Cmd,
		Entrypoint: s.Entrypoint,
		Env:        s.Env,
		Image:      s.Image,
	}
	res, err := cli.ContainerCreate(ctx, &config, nil, nil, nil, name)
	if err != nil {
		return "", err
	}
	log.Debugf("Created container %s with config: %v", res.ID, config)

	return res.ID, nil
}

// NewApp creates an App from a Docker Compose services definition file.
// If there is any error in reading the definition file an error is returned.
func NewApp(config *AppConfig) (App, error) {
	definitionFile := filepath.Join(config.Path, config.ComposeFile)
	data, err := os.ReadFile(definitionFile)
	if err != nil {
		return App{}, fmt.Errorf("Failed to read compose definition file: %v", err)
	}

	var app App

	if err := yaml.Unmarshal(data, &app); err != nil {
		return App{}, fmt.Errorf("Failed to load compose definition file: %v", err)
	}
	log.Infof("Successfully read Docker Compose definition file from %s", definitionFile)

	return app, nil
}

// Pull downloads all the Docker images needed to run some App.
// If there is any error in downloading the images, it is returned.
func (a *App) Pull() error {
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()
	ctx := context.Background()
	var g errgroup.Group

	log.Debugf("Starting pulling Docker images for app")
	for _, s := range a.Services {
		g.Go(func(s *Service) func() error {
			return func() error {
				log.Debugf("Pulling image %s", s.Image)
				if err := s.pull(ctx, cli); err != nil {
					return err
				}
				log.Debugf("Successfully pulled image %s", s.Image)
				return nil
			}
		}(s))
	}

	if err := g.Wait(); err != nil {
		return err
	}
	log.Infof("Successfully pulled Docker images for app")

	return nil
}

// Start creates, configures and starts all the services needed to run some App
// as Docker containers. It returns the instance of a running application. If
// there is any error, it is returned.
func (a *App) Start(config *StartConfig) (AppInstance, error) {
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer cli.Close()
	ctx := context.Background()
	app := AppInstance{}

	log.Debugf("Starting containers for app")
	for name, service := range a.Services {
		containerId, err := service.create(ctx, cli, config.name(name))
		if err != nil {
			return nil, err
		}

		if err := config.apply(ctx, cli, containerId); err != nil {
			if deleteErr := app.Delete(); deleteErr != nil {
				log.Error(deleteErr)
			}
			return nil, err
		}

		err = cli.ContainerStart(ctx, containerId, types.ContainerStartOptions{})
		if err != nil {
			if deleteErr := app.Delete(); deleteErr != nil {
				log.Error(deleteErr)
			}
			return nil, err
		}
		app[name] = containerId
		log.Debugf("Successfully started service %s as container %s", name, containerId)
	}
	log.Debugf("Successfully started all containers for app")

	return app, nil
}

// apply adds the configurations to a container. If there is any error, it is
// returned.
func (s *StartConfig) apply(ctx context.Context, cli *client.Client, containerId string) error {
	for _, net := range s.Networks {
		err := cli.NetworkConnect(ctx, net, containerId, &network.EndpointSettings{})
		if err != nil {
			return err
		}
	}
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

// Delete will release all the resources needed to run the app. It will delete
// all the Docker containers needed to run the app. If there is any error, it
// is returned.
func (a *AppInstance) Delete() error {
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()
	ctx := context.Background()

	for name, containerId := range *a {
		err := cli.ContainerRemove(ctx, containerId, types.ContainerRemoveOptions{
			Force: true,
		})
		if err != nil {
			return err
		}
		delete(*a, name)
		log.Debugf("Successfully deleted container %s for service %s", containerId, name)
	}
	return nil
}
