package main

import (
	"os"

	"github.com/pako-23/gtdd/internal/algorithms"
	"github.com/spf13/cobra"
)

func newGraphCmd() *cobra.Command {
	graphCommand := &cobra.Command{
		Use:   "graph [flags] [path to graph file]",
		Short: "Generate a Graphviz graph of the dependencies between tests",
		Args:  cobra.ExactArgs(1),
		Long: `Produces a representation of the dependency graph between different
tests of a test suite.

The graph is presented in the DOT language. The typical program that can
read this format is GraphViz.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := algorithms.DependencyGraphFromJson(args[0])
			if err != nil {
				return err
			}

			g.ToDOT(os.Stdout)

			return nil
		},
	}

	return graphCommand
}
