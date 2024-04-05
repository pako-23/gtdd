package docker

import (
	"context"
	"fmt"

	"testing"

	"github.com/docker/docker/api/types/container"
	"gotest.tools/v3/assert"
)

func (m *mockClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	if _, ok := m.failures["ContainerRemove"]; ok {
		return errInjectedFailure
	}

	createdContainersMu.Lock()
	defer createdContainersMu.Unlock()
	if _, ok := createdContainers[containerID]; !ok {
		return fmt.Errorf("container not found")
	}
	delete(createdContainers, containerID)
	return nil
}

func TestValidDelete(t *testing.T) {
	t.Parallel()

	client := newMockClient()
	defer client.Close()

	instance := AppInstance{}

	for _, name := range []string{"app1", "app2", "app3"} {
		res, _ := client.client.ContainerCreate(
			context.TODO(),
			&container.Config{Image: "running"},
			nil,
			nil,
			nil,
			name)

		instance[name] = res.ID

	}

	err := client.Delete(instance)
	assert.NilError(t, err)
	assert.Check(t, len(instance) == 0)
}

func TestInvalidDelete(t *testing.T) {
	t.Parallel()

	client := newMockClient()
	defer client.Close()

	instance := AppInstance{}
	apps := []string{"app1", "app2", "app3"}

	for _, name := range apps {
		res, _ := client.client.ContainerCreate(
			context.TODO(),
			&container.Config{Image: "running"},
			nil,
			nil,
			nil,
			name)
		instance[name] = res.ID
	}

	instance["app4"] = "invaliddelete5"

	err := client.Delete(instance)
	assert.ErrorContains(t, err, "failed in deleting application instance")
	assert.Check(t, len(instance) != 0)

	createdContainersMu.Lock()
	defer createdContainersMu.Unlock()

	for _, name := range apps {
		delete(createdContainers, name)
	}
}
