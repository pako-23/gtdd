package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
)

func (c *Client) Delete(instance AppInstance) error {
	options := container.RemoveOptions{Force: true}

	for name, containerID := range instance {
		if err := c.client.ContainerRemove(context.Background(), containerID, options); err != nil {
			return fmt.Errorf("failed in deleting application instance: %w", err)
		}
		delete(instance, name)
	}
	return nil
}
