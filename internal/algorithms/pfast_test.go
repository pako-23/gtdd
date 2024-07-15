package algorithms_test

import (
	"testing"

	"github.com/pako-23/gtdd/internal/algorithms"
)

func TestPFASTNoDependencies(t *testing.T) {
	testNoDependencies(t, algorithms.PFAST)
}

func TestPFASTExistingDependencies(t *testing.T) {
	testExistingDependencies(t, algorithms.PFAST)
}

func TestPFASTOrDependencies(t *testing.T) {
	testOrDependencies(t, algorithms.PFAST)
}

func TestPFASTOrDependenciesMultipleLen(t *testing.T) {
	testMinLenOrDependencies(t, algorithms.PFAST)
}
