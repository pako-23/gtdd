package cmd

import (
	"os"
	"path/filepath"

	"github.com/pako-23/gtdd/algorithms"
	"github.com/pako-23/gtdd/compose"
	"github.com/pako-23/gtdd/runners"
	"github.com/pako-23/gtdd/testsuite"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newDepsCmd() *cobra.Command {
	var (
		driverConfig   string
		outputFileName string
		runnerCount    uint
		strategy       string
		testSuiteEnv   []string
		testSuiteType  string
	)

	depsCommand := &cobra.Command{
		Use:   "deps [flags] [path to testsuite]",
		Short: "Finds all the dependencies between tests into a test suite",
		Args:  cobra.ExactArgs(1),
		Long: `Finds all the dependencies between tests into a provided test
suite. The artifacts to run the test suite should already be
built.`,
		PreRun: configureLogging,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]

			detector, err := algorithms.NewDependencyDetector(strategy)
			if err != nil {
				return err
			}

			runners, tests, err := setupRunEnv(path, driverConfig, testSuiteType, testSuiteEnv, runnerCount)
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

			g.TransitiveReduction()

			file, err := os.Create(outputFileName)
			if err != nil {
				log.Fatalf("failed to create output file %s: %w", outputFileName, err)
			}
			defer file.Close()

			g.ToJSON(file)

			return nil
		},
	}

	depsCommand.Flags().StringArrayVarP(&testSuiteEnv, "var", "v", []string{}, "An environment variable to pass to the test suite container")
	depsCommand.Flags().StringVarP(&driverConfig, "driver-config", "d", "", "The path to a Docker Compose file configuring the driver")
	depsCommand.Flags().StringVarP(&outputFileName, "output", "o", "graph.json", "The file used to output the resulting dependency graph")
	depsCommand.Flags().StringVarP(&strategy, "strategy", "s", "pfast", "The strategy to detect dependencies between tests")
	depsCommand.Flags().StringVarP(&testSuiteType, "suite-type", "t", "", "The test suite type")
	depsCommand.Flags().UintVarP(&runnerCount, "runners", "r", runners.DefaultSetSize, "The number of concurrent runners")

	return depsCommand
}

// setupRunEnv is a utility function to setup the resources needed to run
// the test suite. It returns the runners to run the test suite and the list
// of tests into the test suite in their original order.
func setupRunEnv(path, driverConfig, suiteType string, testSuiteEnv []string, runnerCount uint) (*runners.RunnerSet, []string, error) {
	var driver compose.App

	suite, err := testsuite.FactoryTestSuite(path, suiteType)
	if err != nil {
		return nil, nil, err
	}

	app, err := compose.NewApp(filepath.Join(path, "docker-compose.yml"))
	if err != nil {
		return nil, nil, err
	}

	tests, err := suite.ListTests()
	if err != nil {
		return nil, nil, err
	}
	log.Debugf("test suite contains tests: %v", tests)

	if driverConfig != "" {
		driver, err = compose.NewApp(driverConfig)
		if err != nil {
			return nil, nil, err
		}
	}

	runners, err := runners.NewRunnerSet(&runners.RunnerSetConfig{
		App:          &app,
		Driver:       &driver,
		Runners:      runnerCount,
		TestSuite:    suite,
		TestSuiteEnv: testSuiteEnv,
	})
	if err != nil {
		return nil, nil, err
	}

	return runners, tests, nil
}
