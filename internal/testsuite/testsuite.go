// Copyright 2023 The GTDD Authors. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

// Builds and manages the resources needed to run a test suite.

package testsuite

import (
	log "github.com/sirupsen/logrus"

	"fmt"
	"path/filepath"
	"strings"

	"github.com/pako-23/gtdd/internal/docker"
)

type RunConfig struct {
	Name        string
	Env         []string
	Tests       []string
	StartConfig *docker.RunOptions
}

type TestSuite struct {
	image string
	path  string
}

func NewTestSuite(path string) (*TestSuite, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	return &TestSuite{
		image: strings.ToLower(filepath.Base(absPath)),
		path:  path,
	}, nil
}

func (t *TestSuite) Build() error {
	client, err := docker.NewClient()
	if err != err {
		return err
	}
	defer client.Close()

	return client.BuildImage(t.image, t.path, "Dockerfile")
}

func (t *TestSuite) ListTests() ([]string, error) {
	client, err := docker.NewClient()
	if err != err {
		return nil, err
	}
	defer client.Close()

	app := docker.App{
		"testsuite": {
			Command: []string{"--list-tests"},
			Image:   t.image,
		},
	}
	instance, err := client.Run(app, docker.RunOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to start Java test suite container: %w", err)
	}
	defer func() {
		deleteErr := client.Delete(instance)
		if err == nil {
			err = deleteErr

		}
	}()

	logs, err := client.GetContainerLogs(instance["testsuite"])
	if err != nil {
		return nil, err
	}

	return strings.Split(strings.Trim(logs, "\n"), "\n"), nil
}

func (t *TestSuite) Run(config *RunConfig) ([]bool, error) {
	client, err := docker.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client to run Java test suite: %w", err)
	}

	suite := docker.App{
		config.Name: {
			Command:     config.Tests,
			Image:       t.image,
			Environment: config.Env,
		},
	}

	instance, err := client.Run(suite, *config.StartConfig)
	if err != nil {
		return nil, fmt.Errorf("error in starting java test suite container: %w", err)
	}
	defer func() {
		deleteErr := client.Delete(instance)
		if err == nil {
			err = deleteErr

		}
	}()
	log.Debugf("successfully started testsuite container %s", instance[config.Name])

	logs, err := client.GetContainerLogs(instance[config.Name])
	if err != nil {
		return nil, err
	}
	log.Debugf("successfully obtained logs from java test suite container %s", instance["testsuite"])
	log.Debugf("container logs: %s", logs)

	result := make([]bool, len(config.Tests))
	lines := strings.Split(strings.Trim(logs, "\n"), "\n")
	for i, line := range lines[len(lines)-len(config.Tests):] {
		result[i] = line[len(line)-1] == '1'
	}

	return result, nil
}
