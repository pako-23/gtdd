package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/pako-23/gtdd/internal/runner"
	compose_runner "github.com/pako-23/gtdd/internal/runner/compose-runner"
	"github.com/pako-23/gtdd/internal/testsuite"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newFlakyCmd() *cobra.Command {
	flakyCommand := &cobra.Command{
		Use:   "flaky [flags] [path to testsuite]",
		Short: "Run the testsuite to detect flakiness",
		Args:  cobra.ExactArgs(1),
		Long: `Runs multiple instances of a test suite in the original
order to detect flakiness.`,
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

			runners, err := runner.NewRunnerSet(viper.GetInt("max-runners"),
				compose_runner.ComposeRunnerBuilder, options...)
			if err != nil {
				return err
			}
			defer func() {
				if err := runners.Delete(); err != nil {
					log.Error(err)
				}
			}()

			schedules, err := getSchedules(tests, "")
			if err != nil {
				return err
			}

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

			errorMessages := []string{}
			for i := 1; i <= runners.Size(); i++ {
				for j := 0; j < i; j++ {
					scheduleCh <- schedules[0]
				}

				for j := 0; j < i; j++ {
					select {
					case err := <-errCh:
						errorMessages = append(errorMessages, err.Error())
					case result := <-resultsCh:
						failed := slices.Index(result.results, false)
						if failed != -1 {
							msg := fmt.Sprintf("test %v failed in schedule %v",
								result.schedule[failed], result.schedule)
							errorMessages = append(errorMessages, msg)
						}
					}
				}

				if len(errorMessages) > 0 {
					break
				}

				log.Infof("Testsuite is not flaky with parallelism %d", i)
			}
			close(scheduleCh)
			close(resultsCh)

			if len(errorMessages) > 0 {
				return errors.New(strings.Join(errorMessages, "\n"))
			}

			return nil
		},
	}

	flakyCommand.Flags().StringArrayP("env", "e", []string{}, "an environment variable to pass to the test suite container")
	flakyCommand.Flags().StringP("driver", "d", "", "the path to a Docker Compose file configuring the driver")
	flakyCommand.Flags().Uint("max-runners", uint(runtime.NumCPU()), "the maximum number of concurrent runners")

	return flakyCommand
}
