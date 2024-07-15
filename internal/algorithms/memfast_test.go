package algorithms_test

import (
	"testing"

	"github.com/pako-23/gtdd/internal/algorithms"
)

func TestMEMFASTNoDependencies(t *testing.T) {
	testNoDependencies(t, algorithms.MEMFAST)
}

func TestMEMFASTExistingDependencies(t *testing.T) {
	testExistingDependencies(t, algorithms.MEMFAST)
}

func TestMEMFASTOrDependencies(t *testing.T) {
	testOrDependencies(t, algorithms.MEMFAST)
}

func TestMEMFASTOrDependenciesMultipleLen(t *testing.T) {
	testMinLenOrDependencies(t, algorithms.MEMFAST)
}
