package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pako-23/gtdd/runners"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// schedulesCmd represents the schedules command.
var schedulesCmd = &cobra.Command{
	Use:    "schedules [flags] [path to testsuite]",
	Short:  "Run a test suite with parallelism based on with a given graph",
	Args:   cobra.ExactArgs(1),
	Long:   `.`,
	PreRun: configureLogging,
	Run: func(cmd *cobra.Command, args []string) {
		if err := execSchedulesCmd(args[0]); err != nil {
			log.Fatal(err)
		}
	},
}

func execSchedulesCmd(path string) error {
	runners, tests := setupRunEnv(path)
	defer func() {
		if err := runners.Delete(); err != nil {
			log.Error(err)
		}
	}()

	schedules, err := getSchedules(tests)
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
		log.Fatalf("failed to create json from data: %w", err)
	}
	defer file.Close()

	_, err = file.Write(data)

	return err
}

func initSchedulesCmd() {
	rootCmd.AddCommand(schedulesCmd)

	schedulesCmd.Flags().UintVarP(&runnerCount, "runners", "r", runners.DefaultSetSize, "The number of concurrent runners")
	schedulesCmd.Flags().StringVarP(&outputFileName, "output", "o", "", "The file used to output the schedules")
	schedulesCmd.Flags().StringVarP(&inputFileName, "input", "i", "", "")
}
