package docker

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"gotest.tools/v3/assert"
)

func newMockLogger(stdout, stderr []byte) (*bytes.Buffer, error) {
	buffer := new(bytes.Buffer)
	dstStdout := stdcopy.NewStdWriter(buffer, stdcopy.Stdout)
	if _, err := dstStdout.Write(stdout); err != nil {
		return nil, err
	}
	dstStderr := stdcopy.NewStdWriter(buffer, stdcopy.Stderr)
	if _, err := dstStderr.Write(stderr); err != nil {
		return nil, err
	}

	return buffer, nil
}

var logMap = map[string]func() (io.ReadCloser, error){
	"correct": func() (io.ReadCloser, error) {
		r, err := newMockLogger([]byte("no error"), nil)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(r), nil
	},
	"stderr": func() (io.ReadCloser, error) {
		r, err := newMockLogger(nil, []byte("something in stderr"))
		if err != nil {
			return nil, err
		}
		return io.NopCloser(r), nil
	},
	"both": func() (io.ReadCloser, error) {
		r, err := newMockLogger([]byte("something went wrong"), []byte("something in stderr"))
		if err != nil {
			return nil, err
		}
		return io.NopCloser(r), nil
	},
	"invalid": func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("invalid reader")), nil
	},
}

func (m *mockClient) ContainerLogs(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error) {
	if _, ok := m.failures["ContainerLogs"]; ok {
		return nil, errInjectedFailure
	}

	if _, ok := logMap[container]; !ok {
		return nil, errContainerNotFound
	}

	return logMap[container]()
}

func (m *mockClient) ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {

	statusCh, errCh := make(chan container.WaitResponse), make(chan error)

	if _, ok := m.failures["ContainerWait"]; ok {
		go func() {
			errCh <- errInjectedFailure
		}()

		return statusCh, errCh
	}

	go func() {
		statusCh <- container.WaitResponse{StatusCode: 0}
	}()

	return statusCh, errCh
}

func TestErrContainerLogsMsg(t *testing.T) {
	t.Parallel()

	msg := "log error"

	err := ErrContainerLogs(msg)
	assert.ErrorContains(t, err, msg)
	assert.ErrorContains(t, err, errContainerLogs.Error())
}

func TestCorrectLogs(t *testing.T) {
	t.Parallel()

	client := newMockClient()
	defer client.Close()

	logs, err := client.GetContainerLogs("correct")
	assert.NilError(t, err)
	assert.Check(t, logs == "no error")
}

func TestLogErr(t *testing.T) {
	t.Parallel()

	client := newMockClient()
	defer client.Close()

	var tests = []struct {
		container string
		err       string
	}{
		{"stderr", errContainerLogs.Error()},
		{"both", errContainerLogs.Error()},
		{"invalid", "failed to copy logs from container"},
	}

	for _, test := range tests {
		logs, err := client.GetContainerLogs(test.container)
		assert.Check(t, logs == "")
		assert.ErrorContains(t, err, test.err)
	}

}

func TestLogClientErr(t *testing.T) {
	t.Parallel()

	var tests = []string{
		"ContainerLogs",
		"ContainerWait",
	}

	for _, test := range tests {
		client := newMockClient(test)
		logs, err := client.GetContainerLogs("container")

		assert.Check(t, logs == "")
		assert.ErrorContains(t, err, errInjectedFailure.Error())
		client.Close()
	}
}
