package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
)

type containerMock struct {
	name       string
	config     *container.Config
	hostConfig *container.HostConfig
	state      *types.ContainerState
	networks   []string
}

var (
	createdContainers         = map[string]*containerMock{}
	lastContainerId      uint = 0
	createdContainersMu       = sync.Mutex{}
	errImageNotFound          = errors.New("container image not found")
	errContainerNotFound      = errors.New("container not found")
	existingImages            = map[string]*types.ContainerState{
		"running": {Status: "running"},
		"healthy": {
			Health: &types.Health{
				Status:        "healthy",
				FailingStreak: 0,
			},
		},
		"paused":   {Status: "paused"},
		"removing": {Status: "removing"},
		"exited":   {Status: "exited"},
		"dead":     {Status: "dead"},
		"not-passed-yet-health": {
			Health: &types.Health{
				FailingStreak: 1,
				Log: []*types.HealthcheckResult{
					{Output: "container error"},
				},
			},
		},
		"failing-health": {
			Health: &types.Health{
				FailingStreak: 6,
				Log: []*types.HealthcheckResult{
					{Output: "container error"},
				},
			},
		},
		"other-failing-health": {
			Health: &types.Health{
				FailingStreak: 5,
				Log: []*types.HealthcheckResult{
					{Output: "container error"},
				},
			},
		},
	}
)

func (m *mockClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	if _, ok := m.failures["ContainerCreate"]; ok {
		return container.CreateResponse{}, errInjectedFailure
	}

	if _, ok := existingImages[config.Image]; !ok {
		return container.CreateResponse{}, errImageNotFound
	}

	createdContainersMu.Lock()
	defer createdContainersMu.Unlock()

	id := fmt.Sprintf("container-%d", lastContainerId)
	lastContainerId += 1

	data, _ := json.Marshal(existingImages[config.Image])
	var state types.ContainerState

	_ = json.Unmarshal(data, &state)

	createdContainers[id] = &containerMock{
		name:       containerName,
		config:     config,
		hostConfig: hostConfig,
		state:      &state,
	}

	return container.CreateResponse{ID: id}, nil
}

func (m *mockClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	if _, ok := m.failures["ContainerInspect"]; ok {
		return types.ContainerJSON{}, errInjectedFailure
	}

	createdContainersMu.Lock()
	defer createdContainersMu.Unlock()

	existingContainer, ok := createdContainers[containerID]
	if !ok {
		return types.ContainerJSON{}, errContainerNotFound
	}
	var state types.ContainerState
	var config container.Config

	data, _ := json.Marshal(existingContainer.state)
	json.Unmarshal(data, &state)
	data, _ = json.Marshal(existingContainer.config)
	json.Unmarshal(data, &config)

	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{State: &state},
		Config:            &config,
	}, nil
}

func (m *mockClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	if _, ok := m.failures["ContainerStart"]; ok {
		return errInjectedFailure
	}

	return nil
}

func (m *mockClient) NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error {
	if _, ok := m.failures["NetworkConnect"]; ok {
		return errInjectedFailure
	}

	createdContainersMu.Lock()
	defer createdContainersMu.Unlock()
	container, ok := createdContainers[containerID]
	if !ok {
		return errContainerNotFound
	}

	networksMu.Lock()
	defer networksMu.Unlock()

	if _, ok = networks[networkID]; !ok {
		return errNetworkNotFound
	}

	container.networks = append(container.networks, networkID)

	return nil
}

func TestSuccessfulContainerCreate(t *testing.T) {
	t.Parallel()

	client := newMockClient()
	defer client.Close()

	var tests = map[string]*service{
		"test1": {
			Command:     []string{"-c"},
			Entrypoint:  []string{"sh"},
			Environment: []string{"VAR1=1", "VAR2=2"},
			Image:       "running",
			ShmSize:     1024,
		},
		"test2": {
			Image:       "healthy",
			Healthcheck: &container.HealthConfig{Test: []string{"NONE"}},
		},
		"test3": {
			Image:       "healthy",
			Healthcheck: &container.HealthConfig{Test: []string{"true"}},
		},
		"test4": {
			Image: "healthy",
			Healthcheck: &container.HealthConfig{
				Test:        []string{"true"},
				StartPeriod: time.Duration(10),
			},
		},
		"test5": {
			Image: "healthy",
			Healthcheck: &container.HealthConfig{
				Test:     []string{"true"},
				Interval: time.Duration(10),
			},
		},
		"test6": {
			Image: "healthy",
			Healthcheck: &container.HealthConfig{
				Test:    []string{"true"},
				Timeout: time.Duration(10),
			},
		},
		"test7": {
			Image: "healthy",
			Healthcheck: &container.HealthConfig{
				Test:        []string{"true"},
				Interval:    time.Duration(5),
				Timeout:     time.Duration(10),
				StartPeriod: time.Duration(3),
				Retries:     2,
			},
		},
	}

	for name := range tests {
		func(name string) {
			id, err := client.create(context.TODO(), name, tests[name])
			assert.NilError(t, err)

			createdContainersMu.Lock()
			defer createdContainersMu.Unlock()

			assert.DeepEqual(t, createdContainers[id].name, name)
			assert.DeepEqual(t, createdContainers[id].hostConfig.ShmSize, tests[name].ShmSize)
			assert.DeepEqual(t, createdContainers[id].config.Image, tests[name].Image)
			assert.DeepEqual(t, createdContainers[id].config.Cmd, tests[name].Command)
			assert.DeepEqual(t, createdContainers[id].config.Entrypoint, tests[name].Entrypoint)
			assert.DeepEqual(t, createdContainers[id].config.Env, tests[name].Environment)
			assert.DeepEqual(t, createdContainers[id].config.Healthcheck, tests[name].Healthcheck)
			delete(createdContainers, id)

		}(name)
	}
}

func TestFailedContainerCreate(t *testing.T) {
	t.Parallel()
	client := newMockClient()
	defer client.Close()
	_, err := client.create(context.TODO(), "not-existing", &service{})
	assert.ErrorContains(t, err, errImageNotFound.Error())
}

func TestContainerIsRunning(t *testing.T) {
	t.Parallel()

	var tests = []*container.Config{

		{Image: "running"},
		{
			Image:       "healthy",
			Healthcheck: &container.HealthConfig{Retries: 10},
		},
	}

	client := newMockClient()
	defer client.Close()

	for _, test := range tests {
		res, _ := client.client.ContainerCreate(
			context.TODO(),
			test,
			&container.HostConfig{},
			nil,
			nil,
			"test")

		running, err := client.isRunning(context.TODO(), res.ID)
		assert.NilError(t, err)
		assert.Check(t, running)

		createdContainersMu.Lock()
		delete(createdContainers, res.ID)
		createdContainersMu.Unlock()
	}
}

func TestContainerIsRunningErrors(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		config *container.Config
		error  string
	}{

		{
			config: &container.Config{Image: "paused"},
			error:  "container is into state",
		},
		{
			config: &container.Config{Image: "removing"},
			error:  "container is into state",
		},
		{
			config: &container.Config{Image: "exited"},
			error:  "container is into state",
		},
		{
			config: &container.Config{Image: "dead"},
			error:  "container is into state",
		},
		{
			config: &container.Config{
				Image:       "failing-health",
				Healthcheck: &container.HealthConfig{Retries: 5},
			},
			error: "container error",
		},
		{
			config: &container.Config{
				Image:       "other-failing-health",
				Healthcheck: &container.HealthConfig{Retries: 5},
			},
			error: "container error",
		},
	}

	client := newMockClient()
	defer client.Close()

	for _, test := range tests {
		res, _ := client.client.ContainerCreate(
			context.TODO(),
			test.config,
			&container.HostConfig{},
			nil,
			nil,
			"test")

		running, err := client.isRunning(context.TODO(), res.ID)
		assert.ErrorContains(t, err, test.error)
		assert.Check(t, !running)

		createdContainersMu.Lock()
		delete(createdContainers, res.ID)
		createdContainersMu.Unlock()
	}

}

func TestContainerNotExistingIsRunningErrors(t *testing.T) {
	t.Parallel()

	client := newMockClient()
	defer client.Close()
	_, err := client.isRunning(context.TODO(), "not-existing")
	assert.ErrorContains(t, err, "failed to inspect Docker container")
}

func TestNotPassedYetHealthcheck(t *testing.T) {
	t.Parallel()

	client := newMockClient()
	defer client.Close()

	res, _ := client.client.ContainerCreate(
		context.TODO(),
		&container.Config{
			Image:       "not-passed-yet-health",
			Healthcheck: &container.HealthConfig{Retries: 5},
		},
		&container.HostConfig{},
		nil,
		nil,
		"test")

	running, err := client.isRunning(context.TODO(), res.ID)
	assert.NilError(t, err)
	assert.Check(t, !running)

	createdContainersMu.Lock()
	delete(createdContainers, res.ID)
	createdContainersMu.Unlock()
}

func TestRun(t *testing.T) {
	t.Parallel()

	client := newMockClient()
	defer client.Close()

	var (
		app1 = App{
			"test1": {
				Command:     []string{"-c"},
				Entrypoint:  []string{"sh"},
				Environment: []string{"VAR1=1", "VAR2=2"},
				Image:       "running",
				ShmSize:     1024,
			},
		}
		app2 = App{
			"test2": {
				Image:       "healthy",
				Healthcheck: &container.HealthConfig{Test: []string{"NONE"}},
			},
			"test3": {
				Image:       "healthy",
				Healthcheck: &container.HealthConfig{Test: []string{"true"}},
			},
			"test4": {
				Image: "healthy",
				Healthcheck: &container.HealthConfig{
					Test:        []string{"true"},
					StartPeriod: time.Duration(10),
				},
			},
			"test5": {
				Image: "healthy",
				Healthcheck: &container.HealthConfig{
					Test:     []string{"true"},
					Interval: time.Duration(10),
				},
			},
			"test6": {
				Image: "healthy",
				Healthcheck: &container.HealthConfig{
					Test:    []string{"true"},
					Timeout: time.Duration(10),
				},
			},
			"test7": {
				Image: "healthy",
				Healthcheck: &container.HealthConfig{
					Test:        []string{"true"},
					Interval:    time.Duration(5),
					Timeout:     time.Duration(10),
					StartPeriod: time.Duration(3),
					Retries:     2,
				},
			},
		}
	)

	testNetworks := make([]string, 4)
	for i := 0; i < 4; i++ {
		name := fmt.Sprintf("run-method-test-%d", i)
		_, _ = client.client.NetworkCreate(context.TODO(), name, types.NetworkCreate{})
		testNetworks[i] = name
	}

	var tests = []struct {
		app     App
		options RunOptions
	}{
		{app: app1},
		{app: app2},
		{app: app1, options: RunOptions{Prefix: "prefix"}},
		{app: app2, options: RunOptions{Prefix: "prefix"}},
		{app: app1, options: RunOptions{Networks: []string{testNetworks[0]}}},
		{app: app2, options: RunOptions{Networks: []string{testNetworks[0]}}},
		{app: app1, options: RunOptions{Prefix: "prefix", Networks: []string{testNetworks[0]}}},
		{app: app2, options: RunOptions{Prefix: "prefix", Networks: []string{testNetworks[0]}}},
		{app: app1, options: RunOptions{Networks: testNetworks}},
		{app: app2, options: RunOptions{Networks: testNetworks}},
		{app: app1, options: RunOptions{Prefix: "prefix", Networks: testNetworks}},
		{app: app2, options: RunOptions{Prefix: "prefix", Networks: testNetworks}},
	}

	for i := range tests {

		func(test App, options RunOptions) {
			instance, err := client.Run(test, options)

			assert.NilError(t, err)
			assert.Check(t, len(instance) == len(test))

			networks := make(map[string]struct{}, len(options.Networks))
			for _, network := range options.Networks {
				networks[network] = struct{}{}
			}

			createdContainersMu.Lock()
			defer createdContainersMu.Unlock()

			for name := range test {
				id, ok := instance[name]
				assert.Check(t, ok)

				runningContainer, ok := createdContainers[id]
				assert.Check(t, ok)

				assert.Check(t, strings.HasPrefix(runningContainer.name, options.Prefix))
				assert.Check(t, strings.HasSuffix(runningContainer.name, name))
				assert.DeepEqual(t, runningContainer.hostConfig.ShmSize, test[name].ShmSize)
				assert.DeepEqual(t, runningContainer.config.Image, test[name].Image)
				assert.DeepEqual(t, runningContainer.config.Cmd, test[name].Command)
				assert.DeepEqual(t, runningContainer.config.Entrypoint, test[name].Entrypoint)
				assert.DeepEqual(t, runningContainer.config.Env, test[name].Environment)
				assert.DeepEqual(t, runningContainer.config.Healthcheck, test[name].Healthcheck)

				if test[name].Healthcheck != nil {
					assert.Check(t, runningContainer.state.Health.Status == "healthy")
				} else {
					assert.Check(t, runningContainer.state.Status == "running")
				}

				assert.Check(t, len(runningContainer.networks) == len(options.Networks))
				for _, network := range runningContainer.networks {
					_, ok = networks[network]
					assert.Check(t, ok)
				}

				delete(createdContainers, id)
			}
		}(tests[i].app, tests[i].options)
	}

	for i := 0; i < 4; i++ {
		_ = client.client.NetworkRemove(context.TODO(), fmt.Sprintf("run-method-test-%d", i))
	}
}

func TestRunErrors(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		app     App
		options RunOptions
		err     string
	}{
		{app: App{"run-error-fail-create-1": {Image: "not-existing"}}, err: errImageNotFound.Error()},
		{app: App{"run-error-fail-create-2": {Image: "not-existing"}}, options: RunOptions{Prefix: "prefix"}, err: errImageNotFound.Error()},

		{
			app: App{
				"run-error-fail-create-3": {Image: "running"},
				"run-error-fail-create-4": {Image: "not-existing"},
			},
			options: RunOptions{Prefix: "prefix"},
			err:     errImageNotFound.Error(),
		},
		{
			app: App{
				"run-errors-fail-create-5": {Image: "running"},
				"run-errors-fail-create-6": {Image: "not-existing"},
			},
			options: RunOptions{Prefix: "prefix"},
			err:     errImageNotFound.Error(),
		},
		{
			app:     App{"run-errors-7": {Image: "running"}},
			options: RunOptions{Networks: []string{"no-net"}},
			err:     errNetworkNotFound.Error(),
		},
		{
			app:     App{"run-errors-8": {Image: "running"}},
			options: RunOptions{Prefix: "prefix", Networks: []string{"no-net"}},
			err:     errNetworkNotFound.Error(),
		},
		{
			app: App{"run-error-fail-create-9": {Image: "paused"}},
			err: "the container is into state",
		},
		{
			app: App{
				"run-error-fail-create-10": {Image: "paused"},
				"run-error-fail-create-11": {Image: "running"},
			},
			err: "the container is into state",
		},
		{
			app: App{"run-error-fail-create-12": {Image: "exited"}},
			err: "the container is into state",
		},
	}

	client := newMockClient()
	defer client.Close()

	for _, test := range tests {
		func(app App, options RunOptions, errMsg string) {
			instance, err := client.Run(app, options)
			assert.Check(t, len(instance) == 0)
			assert.ErrorContains(t, err, errMsg)

			createdContainersMu.Lock()
			defer createdContainersMu.Unlock()

			for _, existingContainer := range createdContainers {
				name := existingContainer.name
				if options.Prefix != "" {
					name, _ = strings.CutPrefix(name, options.Prefix+"-")
				}
				_, ok := app[name]
				assert.Check(t, !ok)
			}

		}(test.app, test.options, test.err)
	}
}

func TestFailedStart(t *testing.T) {
	t.Parallel()

	app := App{"test1": {Image: "running"}}
	var tests = [][]string{
		{"ContainerStart"},
		{"ContainerStart", "ContainerRemove"},
	}

	for _, test := range tests {
		client := newMockClient(test...)
		instance, err := client.Run(app, RunOptions{})

		assert.Check(t, instance == nil)
		assert.ErrorContains(t, err, errInjectedFailure.Error())
		client.Close()
	}
}

func TestRunningAfterFailure(t *testing.T) {
	t.Parallel()

	name := "running-after-failure-test"
	app := App{
		name: {
			Image: "not-passed-yet-health",
			Healthcheck: &container.HealthConfig{
				Retries:  5,
				Interval: 100 * time.Millisecond,
			},
		},
	}

	client := newMockClient()
	defer client.Close()

	go func() {
		var runningContainer *containerMock = nil

		for runningContainer == nil {
			time.Sleep(50 * time.Millisecond)
			createdContainersMu.Lock()
			for _, mock := range createdContainers {
				if mock.name == name {
					runningContainer = mock
					break
				}
			}
			createdContainersMu.Unlock()
		}

		time.Sleep(500 * time.Millisecond)
		createdContainersMu.Lock()
		runningContainer.state.Health.Status = "healthy"
		createdContainersMu.Unlock()

	}()

	instance, err := client.Run(app, RunOptions{})

	assert.NilError(t, err)
	assert.Check(t, instance != nil)

	createdContainersMu.Lock()
	defer createdContainersMu.Unlock()

	id, ok := instance[name]
	assert.Check(t, ok)

	runningContainer, ok := createdContainers[id]
	assert.Check(t, ok)

	assert.Check(t, runningContainer.name == name)
	assert.DeepEqual(t, runningContainer.config.Image, app[name].Image)
	assert.DeepEqual(t, runningContainer.config.Healthcheck, app[name].Healthcheck)
	assert.Check(t, runningContainer.state.Health.Status == "healthy")
	delete(createdContainers, id)
}
