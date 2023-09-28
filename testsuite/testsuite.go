// Copyright 2023 The GTDD Authors. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

// Builds and manages the resources needed to run a test suite.

package testsuite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/pako-23/gtdd/compose"
	log "github.com/sirupsen/logrus"
)

// ErrNotSupportedTestSuiteType represents the error returned when trying to
// create a test suite which is not supported.
var ErrNotSupportedTestSuiteType = errors.New("not a supported test-suite type")

// errContainerLogs represents the error returned when a container has
// errors in its logs.
var errContainerLogs = errors.New("errors in the logs of container")

// RunConfig represents the configurations that can be passed to a test suite
// when it is run.
type RunConfig struct {
	// The environment to pass to the container running the test suite.
	Env []string
	// The list of tests to run.
	Tests []string
	// The configuration to apply to the container running the tests.
	StartConfig *compose.StartConfig
}

// TestSuite defines the operations that should be supported by a generic
// test suite.
type TestSuite interface {
	// Build creates the artifacts needed to run the test suite given the path
	// to the test suite. If there is any error, it is returned.
	Build(string) error
	// ListTests returns the list of all tests declared into a test suite in
	// the order in which they are run. If there is any error, it is returned.
	ListTests() ([]string, error)
	// Run invokes the test suite with a given configuration and returns its
	// results. The test results are represented as booleans. If the test
	// is passed, the value is true; otherwise it is false. If there is
	// any error, it is returned.
	Run(*RunConfig) ([]bool, error)
}

// ErrContainerLogs wraps errors found into the logs of a container.
func ErrContainerLogs(msg string) error {
	return fmt.Errorf("%w: %s", errContainerLogs, msg)
}

// buildDockerImage builds a Docker image given the context and the name given
// to the resulting image name. If there is any error in the build process,
// it is returned.
func buildDockerImage(dockerContext, imageName string) error {
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
		Dockerfile:     "Dockerfile",
		Tags:           []string{imageName},
		Remove:         true,
		SuppressOutput: true,
	})
	if err != nil {
		return fmt.Errorf("failed to build Docker image: %w", err)
	}
	defer res.Body.Close()

	if _, err := io.Copy(io.Discard, res.Body); err != nil {
		return fmt.Errorf("failed to build Docker image: %w", err)
	}

	log.Infof("successfully built Docker image %s", imageName)
	return nil
}

// getContainerLogs returns the logs from a given container. If there is any
// error in retrieving the logs, it is returned.
func getContainerLogs(ctx context.Context, cli *client.Client, containerID string) (string, error) {
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

// FactoryTestSuite creates the correct test suite from a given type.
// If the type is not recognized, an error is returned.
func FactoryTestSuite(testSuiteType string) (TestSuite, error) {
	switch testSuiteType {
	case "java":
		return &JavaTestSuite{Image: "testsuite"}, nil
	default:
		return nil, ErrNotSupportedTestSuiteType
	}
}
