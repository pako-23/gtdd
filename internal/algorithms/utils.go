package algorithms

// remove removes the element at a given index from a list of strings and
// resulting list.
func remove(s []string, index int) []string {
	ret := make([]string, 0)
	ret = append(ret, s[:index]...)

	return append(ret, s[index+1:]...)
}
