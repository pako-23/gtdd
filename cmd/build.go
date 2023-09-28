package cmd

import (
	"fmt"

	"github.com/pako-23/gtdd/compose"
	"github.com/pako-23/gtdd/testsuite"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// buildCmd represents the build command.
var buildCmd = &cobra.Command{
	Use:    "build [flags] [path to testsuite]",
	Short:  "Builds the artifacts needed to run a test suite",
	Args:   cobra.ExactArgs(1),
	Long:   `Creates all the artifacts needed to run the test suite.`,
	PreRun: configureLogging,
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]

		app, err := compose.NewApp(&compose.AppConfig{
			Path:        path,
			ComposeFile: "docker-compose.yml",
		})
		if err != nil {
			log.Fatal(err)
		}

		suite, err := testsuite.FactoryTestSuite("java")
		if err != nil {
			log.Fatal(err)
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
			log.Fatal(err)
		}

		log.Info("artifacts build was successful")
	},
}

// initBuildCmd initializes the build command flags.
func initBuildCmd() {
	rootCmd.AddCommand(buildCmd)
}
