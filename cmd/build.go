/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/pako-23/gtdd/compose"
	"github.com/pako-23/gtdd/testsuite"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build [flags] [path to testsuite]",
	Short: "A brief description of your command",
	Args:  cobra.ExactArgs(1),
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: buildCommandExec,
}

func init() {
	rootCmd.AddCommand(buildCmd)
}

func buildCommandExec(cmd *cobra.Command, args []string) {
	path := args[0]

	app, err := compose.NewApp(&compose.AppConfig{
		Path:        path,
		ComposeFile: "docker-compose.yml",
	})
	if err != nil {
		log.Fatal(err)
	}

	suite, err := testsuite.TestSuiteFactory("java")
	if err != nil {
		log.Fatal(err)
	}

	var g errgroup.Group

	g.Go(app.Pull)
	g.Go(func() error { return suite.Build(path) })

	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}

	log.Info("Artifacts built was successful")
}
