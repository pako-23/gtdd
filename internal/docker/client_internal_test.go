package docker

import (
	"errors"
	"testing"

	client "github.com/docker/docker/client"
	"gotest.tools/v3/assert"
)

var errInjectedFailure = errors.New("test injected failure")

type mockClient struct {
	closed   bool
	failures map[string]struct{}
}

func newMockClient(failures ...string) *Client {
	client := &mockClient{
		closed:   false,
		failures: make(map[string]struct{}, len(failures)),
	}

	for _, failure := range failures {
		client.failures[failure] = struct{}{}
	}

	return &Client{client: client}
}

func (m *mockClient) Close() error {
	if _, ok := m.failures["Close"]; ok {
		return errInjectedFailure
	}

	m.closed = true
	return nil
}

func TestNewDefaultClient(t *testing.T) {
	t.Parallel()
	cli, err := NewDefaultClient()
	assert.NilError(t, err)
	assert.Check(t, cli != nil)
	_ = cli.Close()
}

func TestNewClientErr(t *testing.T) {
	t.Parallel()
	cli, err := NewClient(client.WithHost("not a valid url"))
	assert.ErrorContains(t, err, "failed to create Docker client")
	assert.Check(t, cli == nil)
}

func TestClientClose(t *testing.T) {
	t.Parallel()
	mockCli := &mockClient{closed: false}
	cli := &Client{client: mockCli}
	err := cli.Close()
	assert.NilError(t, err)
	assert.Check(t, mockCli.closed)
}

func TestClientCloseErr(t *testing.T) {
	t.Parallel()
	cli := newMockClient("Close")

	err := cli.Close()
	assert.ErrorContains(t, err, err.Error())
}
