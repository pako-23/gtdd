/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"github.com/pako-23/gtdd/algorithms"
	"github.com/pako-23/gtdd/compose"
	"github.com/pako-23/gtdd/runners"
	"github.com/pako-23/gtdd/testsuite"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var runnerCount uint
var strategy string

// depsCmd represents the deps command
var depsCmd = &cobra.Command{
	Use:   "deps [flags] [path to testsuite]",
	Short: "A brief description of your command",
	Args:  cobra.ExactArgs(1),
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: depsCommandExec,
}

func init() {
	rootCmd.AddCommand(depsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// depsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// depsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	depsCmd.Flags().StringVarP(&strategy, "strategy", "s", "ex-linear", "Strategy")
	depsCmd.Flags().UintVarP(&runnerCount, "runners", "r", 4, "The number of concurrent runners")
}

func depsCommandExec(cmd *cobra.Command, args []string) {
	path := args[0]

	suite, err := testsuite.TestSuiteFactory("java")
	if err != nil {
		log.Fatal(err)
	}
	tests, err := suite.ListTests()
	if err != nil {
		log.Fatal(err)
	}
	log.Debugf("Test suite contains tests: %v", tests)

	app, err := compose.NewApp(&compose.AppConfig{
		Path:        path,
		ComposeFile: "docker-compose.yml",
	})
	if err != nil {
		log.Fatal(err)
	}

	runners, err := runners.NewRunnerSet(&runners.RunnerSetConfig{
		App: &app,
		Driver: &compose.App{Services: map[string]*compose.Service{
			"selenium": {Image: "selenium/standalone-chrome:115.0"},
		}},
		Runners:      runnerCount,
		TestSuite:    suite,
		TestSuiteEnv: []string{"app_url=http://app", "driver_url=http://selenium:4444"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := runners.Delete(); err != nil {
			log.Error(err)
		}
	}()

	switch strategy {
	case "ex-linear":
		algorithms.ExLinear(tests, runners)
	case "pradet":
		algorithms.Pradet(tests, runners)
	default:
		cmd.Help()
		os.Exit(1)
	}
}
