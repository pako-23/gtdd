package main

import (
	"fmt"
	"github.com/pako-23/gtdd/internal/docker"
	"github.com/pako-23/gtdd/internal/testsuite"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
	"path/filepath"
)

func newBuildCmd() *cobra.Command {

	buildCommand := &cobra.Command{
		Use:   "build [flags] [path to testsuite]",
		Short: "Builds the artifacts needed to run a test suite",
		Args:  cobra.ExactArgs(1),
		Long:  `Creates all the artifacts needed to run the test suite.`,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]

			var waitgroup errgroup.Group

			waitgroup.Go(func() error {
				client, err := docker.NewClient()
				if err != nil {
					return nil
				}
				defer client.Close()

				_, err = client.NewApp(filepath.Join(path, "docker-compose.yml"))
				if err != nil {
					return err
				}
				return nil

			})
			waitgroup.Go(func() error {
				suite, err := testsuite.FactoryTestSuite(path, viper.GetString("type"))
				if err != nil {
					return err
				}

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

	buildCommand.Flags().StringP("type", "t", "", "The test suite type")

	return buildCommand
}
