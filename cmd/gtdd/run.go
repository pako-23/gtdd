package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pako-23/gtdd/internal/runner"
	compose_runner "github.com/pako-23/gtdd/internal/runner/compose-runner"
	"github.com/pako-23/gtdd/internal/testsuite"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
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
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
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

			options := []runner.RunnerOption[*compose_runner.ComposeRunner]{
				compose_runner.WithAppDefinition(filepath.Join(path, "docker-compose.yml")),
				compose_runner.WithEnv(viper.GetStringSlice("env")),
				compose_runner.WithTestsuite(suite),
			}

			if viper.GetString("driver") != "" {
				options = append(options,
					compose_runner.WithDriverDefinition(viper.GetString("driver")))
			}

			runners, err := runner.NewRunnerSet(viper.GetInt("runners"),
				compose_runner.ComposeRunnerBuilder, options...)
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

	return runCommand
}

// runSchedule
func runSchedule(schedule []string, errCh chan error, resultsCh chan runResults, r *runner.RunnerSet) {
	out, err := r.RunSchedule(schedule)
	if err != nil {
		errCh <- err

		return
	}

	resultsCh <- runResults{schedule: schedule, results: out.Results, time: out.RunningTime}
}
