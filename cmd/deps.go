package cmd

import (
	"os"
	"strings"

	"github.com/pako-23/gtdd/algorithms"
	"github.com/pako-23/gtdd/compose"
	"github.com/pako-23/gtdd/runners"
	"github.com/pako-23/gtdd/testsuite"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newDepsCmd() *cobra.Command {
	var (
		runnerCount    uint
		strategy       string
		outputFileName string
		testSuiteEnv   []string
		driver         []string
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

			runners, tests, err := setupRunEnv(path, driver, testSuiteEnv, runnerCount)
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

	depsCommand.Flags().StringVarP(&strategy, "strategy", "s", "pfast", "The strategy to detect dependencies between tests")
	depsCommand.Flags().UintVarP(&runnerCount, "runners", "r", runners.DefaultSetSize, "The number of concurrent runners")
	depsCommand.Flags().StringVarP(&outputFileName, "output", "o", "graph.json", "The file used to output the resulting dependency graph")
	depsCommand.Flags().StringArrayVarP(&testSuiteEnv, "var", "v", []string{}, "An environment variable to pass to the test suite container")
	depsCommand.Flags().StringArrayVarP(&driver, "driver", "d", []string{}, "The driver image in the form <name>=<image>")

	return depsCommand
}

// setupRunEnv is a utility function to setup the resources needed to run
// the test suite. It returns the runners to run the test suite and the list
// of tests into the test suite in their original order.
func setupRunEnv(path string, driver, testSuiteEnv []string, runnerCount uint) (*runners.RunnerSet, []string, error) {
	suite, err := testsuite.FactoryTestSuite("java")
	if err != nil {
		return nil, nil, err
	}

	app, err := compose.NewApp(&compose.AppConfig{
		Path:        path,
		ComposeFile: "docker-compose.yml",
	})
	if err != nil {
		return nil, nil, err
	}

	tests, err := suite.ListTests()
	if err != nil {
		return nil, nil, err
	}
	log.Debugf("test suite contains tests: %v", tests)

	// "selenium": {Image: "selenium/standalone-chrome:115.0"},

	services := map[string]*compose.Service{}

	for _, service := range driver {
		name, image, _ := strings.Cut(service, "=")
		services[name] = &compose.Service{Image: image}
	}

	runners, err := runners.NewRunnerSet(&runners.RunnerSetConfig{
		App:          &app,
		Driver:       &compose.App{Services: services},
		Runners:      runnerCount,
		TestSuite:    suite,
		TestSuiteEnv: testSuiteEnv,
	})
	if err != nil {
		return nil, nil, err
	}

	return runners, tests, nil
}
