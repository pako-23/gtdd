package docker

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"gotest.tools/v3/assert"
)

var errNetworkNotFound = errors.New("network not found")

func (m *mockClient) NetworkRemove(ctx context.Context, networkID string) error {
	if _, ok := m.failures["NetworkRemove"]; ok {
		return errInjectedFailure
	}

	networksMu.Lock()
	defer networksMu.Unlock()

	if _, ok := networks[networkID]; !ok {
		return errNetworkNotFound
	}

	delete(networks, networkID)
	return nil
}

func TestNetworkRemove(t *testing.T) {
	client := newMockClient()
	defer client.Close()

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("network-remove-%d", i)
		_, err := client.NetworkCreate(name)

		assert.NilError(t, err)
	}

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("network-remove-%d", i)
		err := client.NetworkRemove(name)
		assert.NilError(t, err)
	}

	networksMu.Lock()
	defer networksMu.Unlock()
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("network-remove-%d", i)

		_, ok := networks[name]
		assert.Check(t, !ok)
	}
}

func TestNetworkRemoveErr(t *testing.T) {
	client := newMockClient()
	defer client.Close()

	_, err := client.NetworkCreate("network-remove-err")
	assert.NilError(t, err)
	err = client.NetworkRemove("network-remove-not-existing")
	assert.ErrorContains(t, err, errNetworkNotFound.Error())

	networksMu.Lock()
	defer networksMu.Unlock()
	_, ok := networks["network-remove-err"]
	assert.Check(t, ok)
	delete(networks, "network-remove-err")
}
