package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/archive"
)

func (c *Client) buildImage(ctx context.Context, imageName, srcPath, dockerfile string) error {
	tar, _ := archive.TarWithOptions(srcPath, &archive.TarOptions{
		Compression: archive.Gzip,
	})

	res, err := c.client.ImageBuild(ctx, tar, types.ImageBuildOptions{
		Dockerfile:     dockerfile,
		ForceRemove:    true,
		NoCache:        false,
		Remove:         true,
		SuppressOutput: true,
		Tags:           []string{imageName},
	})
	if err != nil {
		return fmt.Errorf("failed to build Docker image: %w", err)
	}
	defer res.Body.Close()

	scanner := bufio.NewScanner(res.Body)
	var logLine struct {
		Error string `json:"error"`
	}

	for scanner.Scan() {
		if err := json.Unmarshal([]byte(scanner.Text()), &logLine); err != nil {
			return fmt.Errorf("failed to get Docker image build logs: %w", err)
		}

		if logLine.Error != "" {
			return fmt.Errorf("failed to build Docker image: %s", logLine.Error)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to get Docker image build logs: %w", err)
	}

	return nil
}

func (c *Client) BuildImage(imageName, srcPath, dockerfile string) error {
	return c.buildImage(context.Background(), imageName, srcPath, dockerfile)
}
