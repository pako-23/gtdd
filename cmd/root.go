// Copyright 2023 The GTDD Authors. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

// Define all commands supported by GTDD.

package cmd

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	logLevel  string
	logFormat string
	logFile   string
)

func newRootCmd() *cobra.Command {
	rootCommand := &cobra.Command{
		Use:          "gtdd",
		Short:        "A tool to manage test suite dependency detection",
		Long:         `A tool to manage test suite dependency detection`,
		SilenceUsage: true,
	}

	rootCommand.PersistentFlags().StringVar(&logLevel, "log", "info", "Log level")
	rootCommand.PersistentFlags().StringVar(&logFormat, "format", "plain", "The log format")
	rootCommand.PersistentFlags().StringVar(&logFile, "log-file", "", "The log file")

	rootCommand.AddCommand(
		newBuildCmd(),
		newDepsCmd(),
		newGraphCmd(),
		newRunCmd(),
		newSchedulesCmd(),
	)

	return rootCommand

}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

// toLogLevel translates a string to the corresponding log level. If the
// provided string representing the log level is not supported, the program
// will exit with an error.
func toLogLevel(level string) log.Level {
	switch strings.ToLower(level) {
	case "info":
		return log.InfoLevel
	case "debug":
		return log.DebugLevel
	default:
		log.Fatalf("%s is not a supported logging level", level)
	}
	return log.InfoLevel
}

// configureLogging configures all the logging options.
func configureLogging(cmd *cobra.Command, args []string) {
	log.SetLevel(toLogLevel(logLevel))
	switch logFormat {
	case "json":
		log.SetFormatter(&log.JSONFormatter{})
	}

	if logFile == "" {
		return
	}

	file, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o666)
	if err == nil {
		log.SetOutput(file)
	}
}
