package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	log "github.com/sirupsen/logrus"
)

// errContainerLogs represents the error returned when a container has
// errors in its logs.
var errContainerLogs = errors.New("errors in the logs of container")

// ErrContainerLogs wraps errors found into the logs of a container.
func ErrContainerLogs(msg string) error {
	return fmt.Errorf("%w: %s", errContainerLogs, msg)
}

// getContainerLogs returns the logs from a given container. If there is any
// error in retrieving the logs, it is returned.
func (c *Client) GetContainerLogs(containerID string) (string, error) {
	ctx := context.Background()
	statusCh, errCh := c.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return "", err
		}
	case <-statusCh:
	}

	out, err := c.client.ContainerLogs(ctx, containerID, container.LogsOptions{ShowStdout: true})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve container logs: %w", err)
	}
	defer out.Close()
	log.Debugf("successfully connected to container %s to retrieve its logs", containerID)

	var stdout, stderr bytes.Buffer

	if _, err := stdcopy.StdCopy(&stdout, &stderr, out); err != nil {
		return "", fmt.Errorf("failed to copy logs from container: %w", err)
	} else if stderr.Len() > 0 {
		return "", ErrContainerLogs(stderr.String())
	}
	log.Debugf("successfully read logs from container %s", containerID)

	return stdout.String(), nil
}
