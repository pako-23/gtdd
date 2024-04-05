package docker

import "context"

func (c *Client) NetworkRemove(networkID string) error {
	return c.client.NetworkRemove(context.Background(), networkID)
}
