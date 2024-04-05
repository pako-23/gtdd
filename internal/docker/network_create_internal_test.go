package docker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/docker/docker/api/types"
	"gotest.tools/v3/assert"
)

var (
	networks                = map[string]struct{}{}
	networksMu              = sync.Mutex{}
	errNetworkAlreadyExists = errors.New("network already exists")
)

func (m *mockClient) NetworkCreate(ctx context.Context, name string, config types.NetworkCreate) (types.NetworkCreateResponse, error) {
	if _, ok := m.failures["NetworkCreate"]; ok {
		return types.NetworkCreateResponse{}, errInjectedFailure
	}

	networksMu.Lock()
	defer networksMu.Unlock()

	if _, ok := networks[name]; ok {
		return types.NetworkCreateResponse{}, errNetworkAlreadyExists
	}

	networks[name] = struct{}{}

	return types.NetworkCreateResponse{ID: name}, nil
}

func TestNetworkCreation(t *testing.T) {
	client := newMockClient()
	defer client.Close()
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("network-create-%d", i)
		networkID, err := client.NetworkCreate(name)

		assert.NilError(t, err)
		assert.DeepEqual(t, name, networkID)
	}

	networksMu.Lock()
	defer networksMu.Unlock()
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("network-create-%d", i)

		_, ok := networks[name]
		assert.Check(t, ok)
		delete(networks, name)
	}
}

func TestNetworkCreationErr(t *testing.T) {
	client := newMockClient()
	defer client.Close()
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("network-create-%d", i)
		networkID, err := client.NetworkCreate(name)

		assert.NilError(t, err)
		assert.DeepEqual(t, name, networkID)
	}

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("network-create-%d", i)
		networkID, err := client.NetworkCreate(name)

		assert.ErrorContains(t, err, errNetworkAlreadyExists.Error())
		assert.DeepEqual(t, "", networkID)
	}
}
