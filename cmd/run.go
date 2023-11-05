package cmd

import (
	"fmt"
	"strings"

	"github.com/pako-23/gtdd/algorithms"
	"github.com/pako-23/gtdd/runners"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type runResults struct {
	schedule []string
	results  []bool
}

func newRunCmd() *cobra.Command {
	var (
		runnerCount   uint
		testSuiteEnv  []string
		driverConfig  string
		inputFileName string
	)

	runCommand := &cobra.Command{
		Use:   "run [flags] [path to testsuite]",
		Short: "Run a test suite with parallelism based on with a given graph",
		Args:  cobra.ExactArgs(1),
		Long: `Runs a given test suite in parallel. The parallel schedules are
computed based on a given graph. If no graph is provided, it
will run the tests in the original order.`,
		PreRun: configureLogging,
		RunE: func(cmd *cobra.Command, args []string) error {

			runners, tests, err := setupRunEnv(args[0], driverConfig, testSuiteEnv, runnerCount)
			if err != nil {
				return err
			}
			defer func() {
				if err := runners.Delete(); err != nil {
					log.Error(err)
				}
			}()

			schedules, err := getSchedules(tests, inputFileName)
			if err != nil {
				return err
			}

			errCh, resultsCh := make(chan error), make(chan runResults)

			for _, schedule := range schedules {
				go runSchedule(schedule, errCh, resultsCh, runners)
			}

			var errorMessages []string = []string{}

			for i := 0; i < len(schedules); i++ {
				select {
				case err := <-errCh:
					close(errCh)
					close(resultsCh)
					return err
				case result := <-resultsCh:
					failed := algorithms.FindFailed(result.results)
					if failed != -1 {
						errorMessages = append(errorMessages, fmt.Sprintf("test %v failed in schedule %v", result.schedule[failed], result.schedule))
					}
				}
			}

			if len(errorMessages) > 0 {
				return errors.New(strings.Join(errorMessages, "\n"))
			}
			return nil
		},
	}

	runCommand.Flags().UintVarP(&runnerCount, "runners", "r", runners.DefaultSetSize, "The number of concurrent runners")
	runCommand.Flags().StringVarP(&inputFileName, "input", "i", "", "")
	runCommand.Flags().StringArrayVarP(&testSuiteEnv, "var", "v", []string{}, "An environment variable to pass to the test suite container")
	runCommand.Flags().StringVarP(&driverConfig, "driver-config", "d", "", "The path to a Docker Compose file configuring the driver")

	return runCommand
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
	defer func() { go r.Release(runnerID) }()

	out, err := r.Get(runnerID).Run(schedule)
	if err != nil {
		errCh <- err

		return
	}

	resultsCh <- runResults{schedule: schedule, results: out}
}
