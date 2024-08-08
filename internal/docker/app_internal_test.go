package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	cgotypes "github.com/compose-spec/compose-go/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/strslice"
	"gotest.tools/v3/assert"
)

var images = map[string]func() io.ReadCloser{
	"test-image": func() io.ReadCloser {
		return io.NopCloser(strings.NewReader(""))
	},
	"failing-read": func() io.ReadCloser {
		return io.NopCloser(&errorReader{})
	},
}

func (m *mockClient) ImagePull(
	ctx context.Context, refStr string, options image.PullOptions,
) (io.ReadCloser, error) {
	if _, ok := m.failures["ImagePull"]; ok {
		return nil, errInjectedFailure
	}
	reader, ok := images[refStr]
	if !ok {
		return nil, errImageNotFound
	}

	return reader(), nil
}

func TestNotExistingFile(t *testing.T) {
	t.Parallel()

	client := newMockClient()
	defer client.Close()

	app, err := client.NewApp("not-existing-file.yaml")
	assert.ErrorContains(t, err, "failed to load app definition file")
	assert.Check(t, app == nil)
}

func TestImagePullErrors(t *testing.T) {
	t.Parallel()
	client := newMockClient()
	defer client.Close()

	var tests = []struct {
		image string
		err   string
	}{
		{"failing-read", "failed to read Docker image pull logs"},
		{"not-existing", "failed to pull Docker image"},
	}

	for _, test := range tests {
		assert.ErrorContains(t, client.pull(context.TODO(), test.image), test.err)
	}
}

func TestNewService(t *testing.T) {
	t.Parallel()

	env := func(s string) *string {
		return &s
	}

	duration := func(d int64) *cgotypes.Duration {
		ret := cgotypes.Duration(d)
		return &ret
	}
	intPtr := func(i uint64) *uint64 { return &i }

	var tests = []*cgotypes.ServiceConfig{
		{
			Command:    []string{"-c"},
			Entrypoint: []string{"sh"},
			Environment: cgotypes.MappingWithEquals{
				"VAR1=1": nil,
				"VAR2":   env("2"),
			},
			Image:   "test",
			ShmSize: 1024,
		},
		{
			Command:    []string{"-c"},
			Entrypoint: []string{"sh"},
			Environment: cgotypes.MappingWithEquals{
				"VAR1=1": nil,
				"VAR2":   env("2"),
			},
			ShmSize: 1024,
			Build:   &cgotypes.BuildConfig{Context: "/tmp"},
		},
		{
			Command:    []string{"-c"},
			Entrypoint: []string{"sh"},
			Environment: cgotypes.MappingWithEquals{
				"VAR1=1": nil,
				"VAR2":   env("2"),
			},
			ShmSize: 1024,
			Build:   &cgotypes.BuildConfig{Context: "."},
		},
		{
			Image:       "test",
			HealthCheck: &cgotypes.HealthCheckConfig{Disable: true},
		},
		{
			Image:       "test",
			HealthCheck: &cgotypes.HealthCheckConfig{Test: []string{"true"}},
		},
		{
			Image: "test",
			HealthCheck: &cgotypes.HealthCheckConfig{
				Test:        []string{"true"},
				StartPeriod: duration(10),
			},
		},
		{
			Image: "test",
			HealthCheck: &cgotypes.HealthCheckConfig{
				Test:     []string{"true"},
				Interval: duration(10),
			},
		},
		{
			Image: "test",
			HealthCheck: &cgotypes.HealthCheckConfig{
				Test:    []string{"true"},
				Timeout: duration(10),
			},
		},
		{
			Image: "test",
			HealthCheck: &cgotypes.HealthCheckConfig{
				Test:        []string{"true"},
				Interval:    duration(5),
				Timeout:     duration(10),
				StartPeriod: duration(3),
				Retries:     intPtr(2),
			},
		},
	}

	for _, test := range tests {
		srv := newService("test", test)
		assert.Check(t, srv != nil)
		assert.Check(t, srv.Image != "")

		if test.Image == "" {
			assert.Check(t, strings.Contains(srv.Image, filepath.Base(test.Build.Context)))
			assert.Check(t, strings.Contains(srv.Image, "test"))
		} else {
			assert.Check(t, test.Image == srv.Image)
		}

		assert.DeepEqual(t, srv.Command, strslice.StrSlice(test.Command))
		assert.DeepEqual(t, srv.Entrypoint, strslice.StrSlice(test.Entrypoint))
		assert.Check(t, srv.ShmSize == int64(test.ShmSize))

		for key, value := range test.Environment {
			if value == nil {
				assert.Check(t, slices.Contains(srv.Environment, key))
			} else {
				envVar := strings.Join([]string{key, *value}, "=")
				assert.Check(t, slices.Contains(srv.Environment, envVar))
			}
		}

		if test.HealthCheck == nil {
			assert.Check(t, srv.Healthcheck == nil)
			continue
		}

		if test.HealthCheck.Disable {
			assert.DeepEqual(t, srv.Healthcheck.Test, []string{"NONE"})
		} else {
			assert.DeepEqual(t, []string(test.HealthCheck.Test), srv.Healthcheck.Test)
		}

		if test.HealthCheck.Interval != nil {
			assert.Check(t, time.Duration(*test.HealthCheck.Interval) == srv.Healthcheck.Interval)
		}

		if test.HealthCheck.Timeout != nil {
			assert.Check(t, time.Duration(*test.HealthCheck.Timeout) == srv.Healthcheck.Timeout)
		}

		if test.HealthCheck.StartPeriod != nil {
			assert.Check(t, time.Duration(*test.HealthCheck.StartPeriod) == srv.Healthcheck.StartPeriod)
		}

		if test.HealthCheck.Retries != nil {
			assert.Check(t, int(*test.HealthCheck.Retries) == srv.Healthcheck.Retries)
		}
	}
}

func TestValidAppDefinition(t *testing.T) {
	t.Parallel()

	type build struct {
		context    string
		dockerfile string
	}
	var defintions = []map[string]struct {
		image string
		build build
	}{
		{"app": {image: "test-image"}},
		{"app": {build: build{context: ".", dockerfile: "correct"}}},
		{
			"app":  {image: "test-image"},
			"app1": {build: build{context: ".", dockerfile: "correct"}},
		},
	}
	client := newMockClient()
	defer client.Close()

	for i := range defintions {
		func(i int) {

			file, err := os.CreateTemp("", "test-app-compose")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(file.Name())

			_, _ = file.WriteString("version: '3.9'\n\nservices:\n")

			for name, defintion := range defintions[i] {
				if defintions[i][name].image != "" {
					fmt.Fprintf(file, "  %s:\n    image: %s\n", name, defintion.image)
				} else {
					fmt.Fprintf(file, "  %s:\n    build:\n", name)
					fmt.Fprintf(file, "      context: %s\n", defintion.build.context)
					fmt.Fprintf(file, "      dockerfile: %s\n", defintion.build.dockerfile)
				}
			}

			file.Close()
			app, err := client.NewApp(file.Name())

			assert.NilError(t, err)
			assert.Check(t, app != nil)
			assert.Check(t, len(app) == len(defintions[i]))

			for name := range defintions[i] {
				_, ok := app[name]
				assert.Check(t, ok)
			}
		}(i)

	}
}

func TestInvalidAppDefinition(t *testing.T) {
	t.Parallel()

	type build struct {
		context    string
		dockerfile string
	}
	var defintions = []map[string]struct {
		image string
		build build
	}{
		{
			"app":  {image: "not-existing-image"},
			"app2": {build: build{context: ".", dockerfile: "correct"}},
			"app3": {build: build{context: ".", dockerfile: "correct"}},
			"app4": {build: build{context: ".", dockerfile: "correct"}},
			"app5": {build: build{context: ".", dockerfile: "correct"}},
		},
		{
			"app":  {image: "test-image"},
			"app2": {build: build{context: ".", dockerfile: "correct"}},
			"app3": {build: build{context: ".", dockerfile: "error-logs"}},
			"app4": {build: build{context: ".", dockerfile: "correct"}},
			"app5": {build: build{context: ".", dockerfile: "correct"}},
		},
		{
			"app":  {image: "test-image"},
			"app3": {build: build{context: ".", dockerfile: "error-logs"}},
		},
	}

	client := newMockClient()
	defer client.Close()

	for i := range defintions {
		func(i int) {

			file, err := os.CreateTemp("", "test-app-compose")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(file.Name())

			_, _ = file.WriteString("version: '3.9'\n\nservices:\n")

			for name, defintion := range defintions[i] {
				if defintion.image != "" {
					fmt.Fprintf(file, "  %s:\n    image: %s\n", name, defintion.image)
				} else {
					fmt.Fprintf(file, "  %s:\n    build:\n", name)
					fmt.Fprintf(file, "      context: %s\n", defintion.build.context)
					fmt.Fprintf(file, "      dockerfile: %s\n", defintion.build.dockerfile)
				}
			}

			file.Close()
			app, err := client.NewApp(file.Name())

			assert.Check(t, app == nil)
			assert.Check(t, err != nil)
		}(i)
	}
}
