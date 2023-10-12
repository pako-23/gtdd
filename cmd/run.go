package cmd

import (
	"fmt"

	"github.com/pako-23/gtdd/algorithms"
	"github.com/pako-23/gtdd/runners"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// inputFileName is the file.
var inputFileName string

type runResults struct {
	schedule []string
	results  []bool
}

// runCmd represents the run command.
var runCmd = &cobra.Command{
	Use:   "run [flags] [path to testsuite]",
	Short: "Run a test suite with parallelism based on with a given graph",
	Args:  cobra.ExactArgs(1),
	Long: `Runs a given test suite in parallel. The parallel schedules are
computed based on a given graph. If no graph is provided, it
will run the tests in the original order.`,
	PreRun: configureLogging,
	Run: func(cmd *cobra.Command, args []string) {
		if errors := execRunCmd(args[0]); len(errors) != 0 {
			for _, err := range errors {
				log.Fatal(err)
			}
		}
	},
}

// getSchedules
func getSchedules(tests []string) ([][]string, error) {
	if inputFileName == "" {
		return [][]string{tests}, nil
	}
	graph, err := algorithms.DependencyGraphFromJson(inputFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedules from graph: %w", err)
	}

	return graph.GetSchedules(tests), err
}

// runSchedule
func runSchedule(schedule []string, errCh chan error, resultsCh chan runResults, r *runners.RunnerSet) {
	runnerID, err := r.Reserve()
	if err != nil {
		if err != runners.ErrNoRunner {
			errCh <- err
		}

		return
	}
	defer r.Release(runnerID)

	out, err := r.Get(runnerID).Run(schedule)
	if err != nil {
		errCh <- err

		return
	}

	resultsCh <- runResults{schedule: schedule, results: out}
}

// execRunCmd
func execRunCmd(path string) []error {
	runners, tests := setupRunEnv(path)
	defer func() {
		if err := runners.Delete(); err != nil {
			log.Error(err)
		}
	}()

	schedules, err := getSchedules(tests)
	if err != nil {
		return []error{err}
	}

	errCh, resultsCh := make(chan error), make(chan runResults)

	for _, schedule := range schedules {
		go runSchedule(schedule, errCh, resultsCh, runners)
	}

	var errors []error = []error{}

	for i := 0; i < len(schedules); i++ {
		select {
		case err := <-errCh:
			close(errCh)
			close(resultsCh)
			return []error{err}
		case result := <-resultsCh:
			failed := algorithms.FindFailed(result.results)
			if failed != -1 {
				errors = append(errors, fmt.Errorf("test %v failed in schedule %v", result.schedule[failed], result.schedule))
			}
		}
	}

	return errors
}

func initRunCmd() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().UintVarP(&runnerCount, "runners", "r", runners.DefaultSetSize, "The number of concurrent runners")
	runCmd.Flags().StringVarP(&inputFileName, "input", "i", "", "")
	runCmd.Flags().StringArrayVarP(&testSuiteEnv, "var", "v", []string{}, "An environment variable to pass to the test suite container")
	runCmd.Flags().StringArrayVarP(&driver, "driver", "d", []string{}, "The driver image in the form <name>=<image>")
}
