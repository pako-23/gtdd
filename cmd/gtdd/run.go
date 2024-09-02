package main

import (
	"fmt"
	"os"
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

			suite, err := testsuite.NewTestSuite(path)
			if err != nil {
				return err
			}
			tests, err := suite.ListTests()
			if err != nil {
				return err
			}

			options := []runner.RunnerOption[*compose_runner.ComposeRunner]{
				compose_runner.WithEnv(viper.GetStringSlice("env")),
				compose_runner.WithTestSuite(suite),
			}

			appComposePath := filepath.Join(path, "docker-compose.yml")
			if _, err := os.Stat(appComposePath); err == nil {
				options = append(options,
					compose_runner.WithAppDefinition(appComposePath))
			} else if !os.IsNotExist(err) {
				return err
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

			schedules, err := getSchedules(tests, viper.GetString("graph"))
			if err != nil {
				return err
			}

			duration, err := runSchedules(schedules, runners)
			if err != nil {
				return err
			}

			log.Infof("expected running time %v", duration)
			return nil
		},
	}

	runCommand.Flags().StringArrayP("env", "e", []string{}, "an environment variable to pass to the test suite container")
	runCommand.Flags().StringP("driver", "d", "", "the path to a Docker Compose file configuring the driver")
	runCommand.Flags().StringP("graph", "g", "", "the file containing the graph of dependencies")
	runCommand.Flags().UintP("runners", "r", runner.DefaultSetSize, "the number of concurrent runners")

	return runCommand
}

func runSchedules(schedules [][]string, runners *runner.RunnerSet) (time.Duration, error) {
	scheduleCh := make(chan []string, runners.Size())
	errCh, resultsCh := make(chan error), make(chan runResults, runners.Size())

	for i := 0; i < runners.Size(); i++ {
		go func() {
			for schedule := range scheduleCh {
				out, err := runners.RunSchedule(schedule)
				if err != nil {
					errCh <- err

					continue
				}

				resultsCh <- runResults{
					schedule: schedule,
					results:  out.Results,
					time:     out.RunningTime}

			}
		}()
	}

	go func() {
		for _, schedule := range schedules {
			scheduleCh <- schedule
		}
	}()

	var (
		errorMessages = []string{}
		duration      time.Duration
	)

	for i := 0; i < len(schedules); i++ {
		select {
		case err := <-errCh:
			return 0, err
		case result := <-resultsCh:
			failed := slices.Index(result.results, false)
			if failed != -1 {
				msg := fmt.Sprintf("test %v failed in schedule %v",
					result.schedule[failed], result.schedule)
				errorMessages = append(errorMessages, msg)
			}

			log.Infof("run schedule in %v", result.time)
			if duration < result.time {
				duration = result.time
			}
		}
	}
	close(scheduleCh)
	close(resultsCh)

	if len(errorMessages) > 0 {
		return 0, errors.New(strings.Join(errorMessages, "\n"))
	}

	return duration, nil
}
