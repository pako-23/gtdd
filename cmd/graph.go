package cmd

import (
	"os"

	"github.com/pako-23/gtdd/algorithms"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// graphCmd represents the graph command.
var graphCmd = &cobra.Command{
	Use:   "graph [flags] [path to graph file]",
	Short: "Generate a Graphviz graph of the dependencies between tests",
	Args:  cobra.ExactArgs(1),
	Long: `Produces a representation of the dependency graph between different
tests of a test suite.

The graph is presented in the DOT language. The typical program that can
read this format is GraphViz.`,
	PreRun: configureLogging,
	Run: func(cmd *cobra.Command, args []string) {
		g, err := algorithms.DependencyGraphFromJson(args[0])
		if err != nil {
			log.Fatal(err)
		}

		g.ToDOT(os.Stdout)
	},
}

func initGraphCmd() {
	rootCmd.AddCommand(graphCmd)
}
