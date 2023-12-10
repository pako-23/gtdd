package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/pako-23/gtdd/runners"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

type runResults struct {
	results  []bool
	schedule []string
	time     time.Duration
}

func newRunCmd() *cobra.Command {
	var (
		driverConfig  string
		inputFileName string
		runnerCount   uint
		testSuiteEnv  []string
		testSuiteType string
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

			runners, tests, err := setupRunEnv(args[0], driverConfig, testSuiteType, testSuiteEnv, runnerCount)
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

			var (
				errorMessages    []string = []string{}
				expectedDuration time.Duration
			)

			for i := 0; i < len(schedules); i++ {
				select {
				case err := <-errCh:
					return err
				case result := <-resultsCh:
					failed := slices.Index(result.results, false)
					if failed != -1 {
						errorMessages = append(errorMessages, fmt.Sprintf("test %v failed in schedule %v", result.schedule[failed], result.schedule))
					}

					log.Infof("run schedule in %v", result.time)
					if expectedDuration < result.time {
						expectedDuration = result.time
					}
				}
			}

			if len(errorMessages) > 0 {
				return errors.New(strings.Join(errorMessages, "\n"))
			}

			log.Infof("expected running time %v", expectedDuration)
			return nil
		},
	}

	runCommand.Flags().StringArrayVarP(&testSuiteEnv, "var", "v", []string{}, "An environment variable to pass to the test suite container")
	runCommand.Flags().StringVarP(&driverConfig, "driver-config", "d", "", "The path to a Docker Compose file configuring the driver")
	runCommand.Flags().StringVarP(&inputFileName, "input", "i", "", "")
	runCommand.Flags().StringVarP(&testSuiteType, "suite-type", "t", "", "The test suite type")
	runCommand.Flags().UintVarP(&runnerCount, "runners", "r", runners.DefaultSetSize, "The number of concurrent runners")

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
	start := time.Now()

	out, err := r.Get(runnerID).Run(schedule)
	if err != nil {
		errCh <- err

		return
	}

	resultsCh <- runResults{schedule: schedule, results: out, time: time.Now().Sub(start)}
}
