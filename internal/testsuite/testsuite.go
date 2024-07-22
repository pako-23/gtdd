// Copyright 2023 The GTDD Authors. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

// Builds and manages the resources needed to run a test suite.

package testsuite

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/pako-23/gtdd/internal/docker"
)

// ErrNotSupportedTestSuiteType represents the error returned when trying to
// create a test suite which is not supported.
var ErrNotSupportedTestSuiteType = errors.New("not a supported test-suite type")

var ErrNotTestSuiteType = errors.New("no test suite type provided")

// RunConfig represents the configurations that can be passed to a test suite
// when it is run.
type RunConfig struct {
	Name string
	// The environment to pass to the container running the test suite.
	Env []string
	// The list of tests to run.
	Tests []string
	// The configuration to apply to the container running the tests.
	StartConfig *docker.RunOptions
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

// FactoryTestSuite creates the correct test suite from a given type.
// If the type is not recognized, an error is returned.
func FactoryTestSuite(path, testSuiteType string) (TestSuite, error) {
	if path == "" {
		return nil, ErrNotTestSuiteType
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	switch testSuiteType {
	case "java-selenium":
		return &JavaSeleniumTestSuite{Image: strings.ToLower(filepath.Base(absolutePath))}, nil
	case "junit":
		return &JunitTestSuite{Image: strings.ToLower(filepath.Base(absolutePath))}, nil
	default:
		return nil, ErrNotSupportedTestSuiteType
	}
}
