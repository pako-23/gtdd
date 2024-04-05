package docker

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"gotest.tools/v3/assert"
)

type errorReader struct{}

func (*errorReader) Read(_ []byte) (int, error) {
	return 0, io.ErrNoProgress
}

var dockerfiles = map[string]func() io.ReadCloser{
	"correct": func() io.ReadCloser {
		return io.NopCloser(strings.NewReader(`{"log": "all good"}
{"log": "all good"}
{"log": "all good"}
{"log": "all good"}
`))
	},
	"invalid-logs": func() io.ReadCloser {
		return io.NopCloser(strings.NewReader(`{"log": "all good"}
{"log": "all good"}
daksjflaflkajf
{"log": "all good"}
`))
	},
	"error-logs": func() io.ReadCloser {
		return io.NopCloser(strings.NewReader(`{"log": "all good"}
{"error": "error"}
`))
	},
	"read-error": func() io.ReadCloser { return io.NopCloser(&errorReader{}) },
}

func (m *mockClient) ImageBuild(
	ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions,
) (types.ImageBuildResponse, error) {

	if _, ok := m.failures["ImageBuild"]; ok {
		return types.ImageBuildResponse{}, errInjectedFailure
	}

	body, ok := dockerfiles[options.Dockerfile]

	if !ok {
		return types.ImageBuildResponse{}, fmt.Errorf("not a valid Dockerfile")
	}

	return types.ImageBuildResponse{
		Body: body(),
	}, nil
}

func TestSuccessfulBuild(t *testing.T) {
	t.Parallel()
	client := newMockClient()
	defer client.Close()
	assert.NilError(t, client.BuildImage("test", "/tmp", "correct"))
}

func TestBuildFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file string
		err  string
	}{
		{"", "failed to build Docker image"},
		{"invalid-logs", "failed to get Docker image build logs"},
		{"error-logs", "failed to build Docker image"},
		{"read-error", "failed to get Docker image build logs"},
	}

	client := newMockClient()
	defer client.Close()

	for _, test := range tests {
		assert.ErrorContains(t, client.BuildImage("test", "/tmp", test.file), test.err)
	}
}
