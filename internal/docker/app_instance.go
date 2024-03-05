package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

// An AppInstance represents a collection of running containers with the
// names of the services of the App defining them.
type AppInstance map[string]string

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
