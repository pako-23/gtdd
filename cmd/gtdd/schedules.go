package main

import (
	"encoding/json"
	"fmt"
	"github.com/pako-23/gtdd/internal/algorithms"
	"github.com/pako-23/gtdd/internal/testsuite"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

func newSchedulesCmd() *cobra.Command {

	schedulesCommand := &cobra.Command{
		Use:   "schedules [flags] [path to testsuite]",
		Short: "Run a test suite with parallelism based on with a given graph",
		Args:  cobra.ExactArgs(1),
		Long:  `.`,
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

			schedules, err := getSchedules(tests, viper.GetString("input"))
			if err != nil {
				return err
			}

			data, err := json.MarshalIndent(schedules, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to create json from data: %w", err)
			}

			if viper.GetString("output") == "" {
				_, err = os.Stdout.Write(data)

				return err
			}

			file, err := os.Create(viper.GetString("output"))
			if err != nil {
				return fmt.Errorf("failed to create json from data: %w", err)
			}
			defer file.Close()

			_, err = file.Write(data)

			return err
		},
	}

	schedulesCommand.Flags().StringP("input", "i", "graph.json", "The path to the file containing the graph representing the dependencies between tests")
	schedulesCommand.Flags().StringP("output", "o", "schedules.json", "The path where to write the resulting schedules")
	schedulesCommand.Flags().StringP("type", "t", "", "The test suite type")

	viper.BindPFlags(schedulesCommand.Flags())

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
