package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
)

func (c *Client) NetworkCreate(name string) (string, error) {
	res, err := c.client.NetworkCreate(context.Background(), name, types.NetworkCreate{})
	if err != nil {
		return "", fmt.Errorf("failed to create network: %w", err)
	}
	return res.ID, nil
}
