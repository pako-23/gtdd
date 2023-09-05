/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var logLevel string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gtdd",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {

	// log.SetLevel(toLogLevel(logLevel))

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func toLogLevel(level string) log.Level {

	switch strings.ToLower(level) {
	case "info":
		return log.InfoLevel
	case "debug":
		return log.DebugLevel
	default:
		log.Fatalf("%s is not a supported logging level")
	}
	return log.InfoLevel
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&logLevel, "log", "debug", "Log level")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
}
