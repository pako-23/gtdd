package main

import (
	"os"
	"path/filepath"

	"github.com/pako-23/gtdd/internal/algorithms"
	runner "github.com/pako-23/gtdd/internal/runner/compose-runner"
	"github.com/pako-23/gtdd/internal/testsuite"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newDepsCmd() *cobra.Command {

	depsCommand := &cobra.Command{
		Use:   "deps [flags] [path to testsuite]",
		Short: "Finds all the dependencies between tests into a test suite",
		Args:  cobra.ExactArgs(1),
		Long: `Finds all the dependencies between tests into a provided test
suite. The artifacts to run the test suite should already be
built.`,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]

			detector, err := algorithms.NewDependencyDetector(viper.GetString("strategy"))
			if err != nil {
				return err
			}

			suite, err := testsuite.FactoryTestSuite(path, viper.GetString("type"))
			if err != nil {
				return err
			}
			tests, err := suite.ListTests()
			if err != nil {
				return err
			}

			runners, err := runner.NewRunnerSet(&runner.RunnerSetConfig{
				App:          filepath.Join(path, "docker-compose.yml"),
				Driver:       viper.GetString("driver"),
				Runners:      viper.GetUint("runners"),
				TestSuite:    suite,
				TestSuiteEnv: viper.GetStringSlice("env"),
			})
			if err != nil {
				return err
			}

			defer func() {
				if err := runners.Delete(); err != nil {
					log.Error(err)
				}
			}()

			g, err := detector.FindDependencies(tests, runners)
			if err != nil {
				return err
			}

			file, err := os.Create(viper.GetString("output"))
			if err != nil {
				log.Fatalf("failed to create output file %s: %v", viper.GetString("output"), err)
			}
			defer file.Close()

			g.ToJSON(file)

			return nil
		},
	}

	depsCommand.Flags().StringArrayP("env", "e", []string{}, "An environment variable to pass to the test suite container")
	depsCommand.Flags().StringP("driver", "d", "", "The path to a Docker Compose file configuring the driver")
	depsCommand.Flags().StringP("output", "o", "graph.json", "The file used to output the resulting dependency graph")
	depsCommand.Flags().StringP("strategy", "s", "pfast", "The strategy to detect dependencies between tests")
	depsCommand.Flags().StringP("type", "t", "", "The test suite type")
	depsCommand.Flags().UintP("runners", "r", runner.DefaultSetSize, "The number of concurrent runners")

	return depsCommand
}
