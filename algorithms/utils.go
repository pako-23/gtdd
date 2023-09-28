package algorithms

import "time"

// StartUpTime represents the time to wait before running a test suite for the
// app to be up and running.
const StartUpTime = time.Second * 30

// remove removes the element at a given index from a list of strings and
// resulting list.
func remove(s []string, index int) []string {
	ret := make([]string, 0)
	ret = append(ret, s[:index]...)

	return append(ret, s[index+1:]...)
}

// FindFailed returns the index of the first failed test into a list
// representing the results of a test suite run.
func FindFailed(results []bool) int {
	for i, value := range results {
		if !value {
			return i
		}
	}

	return -1
}
