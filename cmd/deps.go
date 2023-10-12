package cmd

import (
	"errors"
	"os"
	"strings"

	"github.com/pako-23/gtdd/algorithms"
	"github.com/pako-23/gtdd/compose"
	"github.com/pako-23/gtdd/runners"
	"github.com/pako-23/gtdd/testsuite"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	// runnerCount represents the number of runners to run test suites to
	// be allocated.
	runnerCount uint
	// strategy represents the strategy used to find dependencies between tests
	// into the test suite.
	strategy string
	// outputFileName represents the name of the file where the artifacts
	// of the program are outputted.
	outputFileName string
	// testSuiteEnv represents all the environment variables to be passed
	// to the test suite when running it.
	testSuiteEnv []string
	// driver represents the image
	driver []string
	// errStrategyNotExisting represents the error returned a user provided
	// strategy does not exist.
	errStrategyNotExisting = errors.New("strategy does not exist")
)

// depsCmd represents the deps command.
var depsCmd = &cobra.Command{
	Use:   "deps [flags] [path to testsuite]",
	Short: "Finds all the dependencies between tests into a test suite",
	Args:  cobra.ExactArgs(1),
	Long: `Finds all the dependencies between tests into a provided test
suite. The artifacts to run the test suite should already be
built.`,
	PreRun: configureLogging,
	Run: func(cmd *cobra.Command, args []string) {
		if err := execDepsCmd(args[0]); err != nil {
			log.Fatal(err)
		}
	},
}

func execDepsCmd(path string) error {
	var (
		g   algorithms.DependencyGraph
		err error
	)

	runners, tests := setupRunEnv(path)
	defer func() {
		if err := runners.Delete(); err != nil {
			log.Error(err)
		}
	}()

	switch strategy {
	case "pfast":
		g, err = algorithms.PFAST(tests, runners)
	case "pradet":
		g, err = algorithms.PraDet(tests, runners)
	default:
		return errStrategyNotExisting
	}

	if err != nil {
		return err
	}

	g.TransitiveReduction()

	file, err := os.Create(outputFileName)
	if err != nil {
		log.Fatalf("failed to create json from data: %w", err)
	}
	defer file.Close()

	g.ToJSON(file)

	return nil
}

// setupRunEnv is a utility function to setup the resources needed to run
// the test suite. It returns the runners to run the test suite and the list
// of tests into the test suite in their original order.
func setupRunEnv(path string) (*runners.RunnerSet, []string) {
	suite, err := testsuite.FactoryTestSuite("java")
	if err != nil {
		log.Fatal(err)
	}

	app, err := compose.NewApp(&compose.AppConfig{
		Path:        path,
		ComposeFile: "docker-compose.yml",
	})
	if err != nil {
		log.Fatal(err)
	}

	tests, err := suite.ListTests()
	if err != nil {
		log.Fatal(err)
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
		log.Fatal(err)
	}

	return runners, tests
}

// initDepsCmd initializes the deps command flags.
func initDepsCmd() {
	rootCmd.AddCommand(depsCmd)

	depsCmd.Flags().StringVarP(&strategy, "strategy", "s", "pfast", "The strategy to detect dependencies between tests")
	depsCmd.Flags().UintVarP(&runnerCount, "runners", "r", runners.DefaultSetSize, "The number of concurrent runners")
	depsCmd.Flags().StringVarP(&outputFileName, "output", "o", "graph.json", "The file used to output the resulting dependency graph")
	depsCmd.Flags().StringArrayVarP(&testSuiteEnv, "var", "v", []string{}, "An environment variable to pass to the test suite container")
	depsCmd.Flags().StringArrayVarP(&driver, "driver", "d", []string{}, "The driver image in the form <name>=<image>")
}
