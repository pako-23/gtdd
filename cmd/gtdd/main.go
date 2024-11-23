// Copyright 2023 The GTDD Authors. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

// Find dependencies between tests of a test suite.

package main

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newRootCmd() *cobra.Command {
	var cfgFile string

	rootCommand := &cobra.Command{
		Use:          "gtdd",
		Short:        "A tool to manage test suite dependency detection",
		Long:         `A tool to manage test suite dependency detection`,
		SilenceUsage: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlag("log", cmd.Flags().Lookup("log"))
			viper.BindPFlag("log-format", cmd.Flags().Lookup("log-format"))
			viper.BindPFlag("log-file", cmd.Flags().Lookup("log-file"))

			parseConfiguration(cfgFile)

			log.SetLevel(toLogLevel(viper.GetString("log")))
			switch viper.GetString("log-format") {
			case "json":
				log.SetFormatter(&log.JSONFormatter{})
			}

			if viper.GetString("log-file") == "" {
				return
			}

			file, err := os.OpenFile(viper.GetString("log-file"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o666)
			if err == nil {
				log.SetOutput(file)
			}
		},
	}

	rootCommand.PersistentFlags().StringVar(&cfgFile, "config", ".gtdd.yaml", "Configuration file")
	rootCommand.PersistentFlags().String("log", "info", "Log level")
	rootCommand.PersistentFlags().String("log-format", "plain", "The log format")
	rootCommand.PersistentFlags().String("log-file", "", "The log file")

	rootCommand.AddCommand(
		newBuildCmd(),
		newDepsCmd(),
		newFlakyCmd(),
		newGraphCmd(),
		newRunCmd(),
		newSchedulesCmd(),
	)

	return rootCommand
}

func parseConfiguration(cfgFile string) {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		cwd, err := os.Getwd()
		cobra.CheckErr(err)

		viper.AddConfigPath(cwd)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".gtdd")
	}

	viper.AutomaticEnv()
	_ = viper.ReadInConfig()
}

// toLogLevel translates a string to the corresponding log level. If the
// provided string representing the log level is not supported, the program
// will exit with an error.
func toLogLevel(level string) log.Level {
	switch strings.ToLower(level) {
	case log.InfoLevel.String():
		return log.InfoLevel
	case log.DebugLevel.String():
		return log.DebugLevel
	case log.ErrorLevel.String():
		return log.ErrorLevel
	default:
		log.Fatalf("%s is not a supported logging level", level)
	}
	return log.InfoLevel
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
