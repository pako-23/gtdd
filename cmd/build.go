package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/pako-23/gtdd/compose"
	"github.com/pako-23/gtdd/testsuite"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func newBuildCmd() *cobra.Command {
	buildCommand := &cobra.Command{
		Use:    "build [flags] [path to testsuite]",
		Short:  "Builds the artifacts needed to run a test suite",
		Args:   cobra.ExactArgs(1),
		Long:   `Creates all the artifacts needed to run the test suite.`,
		PreRun: configureLogging,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]

			app, err := compose.NewApp(filepath.Join(path, "docker-compose.yml"))
			if err != nil {
				return err
			}

			suite, err := testsuite.FactoryTestSuite("java")
			if err != nil {
				return err
			}

			var waitgroup errgroup.Group

			waitgroup.Go(app.Pull)
			waitgroup.Go(func() error {
				if err := suite.Build(path); err != nil {
					return fmt.Errorf("test suite artifacts build failed: %w", err)
				}

				return nil
			})

			if err := waitgroup.Wait(); err != nil {
				return err
			}
			log.Info("artifacts build was successful")

			return nil
		},
	}

	return buildCommand
}
