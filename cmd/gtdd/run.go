package main

import (
	"fmt"
	runner "github.com/pako-23/gtdd/internal/runner/compose-runner"
	"github.com/pako-23/gtdd/internal/testsuite"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
	"path/filepath"
	"strings"
	"time"
)

type runResults struct {
	results  []bool
	schedule []string
	time     time.Duration
}

func newRunCmd() *cobra.Command {

	runCommand := &cobra.Command{
		Use:   "run [flags] [path to testsuite]",
		Short: "Run a test suite with parallelism based on with a given graph",
		Args:  cobra.ExactArgs(1),
		Long: `Runs a given test suite in parallel. The parallel schedules are
computed based on a given graph. If no graph is provided, it
will run the tests in the original order.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]

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

			schedules, err := getSchedules(tests, viper.GetString("schedules"))
			if err != nil {
				return err
			}

			errCh, resultsCh := make(chan error), make(chan runResults)

			for _, schedule := range schedules {
				go runSchedule(schedule, errCh, resultsCh, runners)

			}

			var (
				errorMessages    = []string{}
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

	runCommand.Flags().StringArrayP("env", "e", []string{}, "An environment variable to pass to the test suite container")
	runCommand.Flags().StringP("driver", "d", "", "The path to a Docker Compose file configuring the driver")
	runCommand.Flags().StringP("schedules", "s", "schedules.json", "A file containing the scedules to run")
	runCommand.Flags().StringP("type", "t", "", "The test suite type")
	runCommand.Flags().UintP("runners", "r", runner.DefaultSetSize, "The number of concurrent runners")
	viper.BindPFlags(runCommand.Flags())

	return runCommand
}

// runSchedule
func runSchedule(schedule []string, errCh chan error, resultsCh chan runResults, r *runner.RunnerSet) {
	runnerID, err := r.Reserve()
	if err != nil {
		if err != runner.ErrNoRunner {
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

	resultsCh <- runResults{schedule: schedule, results: out, time: time.Since(start)}
}
