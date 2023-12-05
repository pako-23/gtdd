package compose

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/stdcopy"
	log "github.com/sirupsen/logrus"
)

// errContainerLogs represents the error returned when a container has
// errors in its logs.
var errContainerLogs = errors.New("errors in the logs of container")

// BuildDockerImage builds a Docker image given the context and the name given
// to the resulting image name. If there is any error in the build process,
// it is returned.
func BuildDockerImage(imageName, dockerContext, dockerfile string) error {
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create client to build Docker image %s: %w", imageName, err)
	}
	defer cli.Close()
	ctx := context.Background()

	log.Debugf("starting build for Docker image %s", imageName)
	tar, err := archive.TarWithOptions(dockerContext, &archive.TarOptions{})
	if err != nil {
		return fmt.Errorf("failed to create tar archive to build Docker image: %w", err)
	}
	log.Debugf("successfully created tar archive to build Docker image: %s", imageName)

	res, err := cli.ImageBuild(ctx, tar, types.ImageBuildOptions{
		Dockerfile:     dockerfile,
		ForceRemove:    true,
		NoCache:        false,
		Remove:         true,
		SuppressOutput: true,
		Tags:           []string{imageName},
	})
	if err != nil {
		return fmt.Errorf("failed to build Docker image: %w", err)
	}
	defer res.Body.Close()

	if err := getBuildErrors(res.Body); err != nil {
		return err
	}

	log.Infof("successfully built docker image %s", imageName)
	return nil
}

func getBuildErrors(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	var logLine struct {
		Error string `json:"error"`
	}

	for scanner.Scan() {
		if err := json.Unmarshal([]byte(scanner.Text()), &logLine); err != nil {
			return fmt.Errorf("failed to read docker logs: %w", err)
		}

		if logLine.Error != "" {
			return fmt.Errorf("docker image build error: %s", logLine.Error)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read docker build logs: %w", err)
	}

	return nil
}

// ErrContainerLogs wraps errors found into the logs of a container.
func ErrContainerLogs(msg string) error {
	return fmt.Errorf("%w: %s", errContainerLogs, msg)
}

// getContainerLogs returns the logs from a given container. If there is any
// error in retrieving the logs, it is returned.
func GetContainerLogs(ctx context.Context, cli *client.Client, containerID string) (string, error) {
	statusCh, errCh := cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return "", err
		}
	case <-statusCh:
	}
	log.Debugf("container successfully finished")

	out, err := cli.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{ShowStdout: true})
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
