package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pako-23/gtdd/algorithms"
	"github.com/spf13/cobra"
)

func newSchedulesCmd() *cobra.Command {
	var (
		inputFileName  string
		outputFileName string
		testSuiteType  string
	)

	schedulesCommand := &cobra.Command{
		Use:    "schedules [flags] [path to testsuite]",
		Short:  "Run a test suite with parallelism based on with a given graph",
		Args:   cobra.ExactArgs(1),
		Long:   `.`,
		PreRun: configureLogging,
		RunE: func(cmd *cobra.Command, args []string) error {
			runners, tests, err := setupRunEnv(args[0], "", testSuiteType, []string{}, 1)
			if err != nil {
				return err
			}

			if err := runners.Delete(); err != nil {
				return err
			}

			schedules, err := getSchedules(tests, inputFileName)
			if err != nil {
				return err
			}

			data, err := json.MarshalIndent(schedules, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to create json from data: %w", err)
			}

			if outputFileName == "" {
				_, err = os.Stdout.Write(data)

				return err
			}

			file, err := os.Create(outputFileName)
			if err != nil {
				return fmt.Errorf("failed to create json from data: %w", err)
			}
			defer file.Close()

			_, err = file.Write(data)

			return err
		},
	}

	schedulesCommand.Flags().StringVarP(&inputFileName, "input", "i", "", "")
	schedulesCommand.Flags().StringVarP(&outputFileName, "output", "o", "", "The file used to output the schedules")
	schedulesCommand.Flags().StringVarP(&testSuiteType, "suite-type", "t", "", "The test suite type")

	return schedulesCommand
}

// getSchedules
func getSchedules(tests []string, inputFileName string) ([][]string, error) {
	if inputFileName == "" {
		return [][]string{tests}, nil
	}
	graph, err := algorithms.DependencyGraphFromJson(inputFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedules from graph: %w", err)
	}

	return graph.GetSchedules(tests), err
}
