package algorithms

import (
	"fmt"
	"sync"

	"github.com/pako-23/gtdd/internal/runner"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type schedule []string

type result struct {
	schedule schedule
	outcome  []bool
	err      error
}

type table struct {
	table  [][]schedule
	failed map[string]struct{}
	mu     sync.Mutex
}

func (r result) Passed() bool {
	for _, outcome := range r.outcome {
		if !outcome {
			return false
		}
	}
	return true
}

func newTable(tests []string, r *runner.RunnerSet[runner.Runner]) (*table, error) {
	var (
		t = &table{
			failed: map[string]struct{}{},
			table:  [][]schedule{{}},
			mu:     sync.Mutex{},
		}
		n  = sync.WaitGroup{}
		ch = make(chan result)
	)

	for _, test := range tests {
		n.Add(1)
		go func(s schedule) {
			defer n.Done()
			runSchedule(r, s, ch)
		}(schedule{test})
	}

	go func() {
		n.Wait()
		close(ch)
	}()

	for res := range ch {
		if res.err != nil {
			close(ch)
			return nil, res.err
		}

		if res.outcome[0] {
			t.table[0] = append(t.table[0], res.schedule)
		} else {
			t.failed[res.schedule[0]] = struct{}{}
		}
	}

	log.Infof("table: %v", t)
	log.Infof("not passed: %v", t.failed)

	return t, nil
}

func (t *table) uniqueInsert(s schedule) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, stored := range t.table[len(s)-1] {
		equal := true

		for i := range stored {
			if stored[i] != s[i] {
				equal = false
				break
			}
		}

		if equal {
			return false
		}
	}

	t.table[len(s)-1] = append(t.table[len(s)-1], s)

	return true
}

func buildReverseIndex(tests []string) map[string]int {
	revIndex := map[string]int{}
	for index, test := range tests {
		revIndex[test] = index
	}

	return revIndex
}

func runSchedule(r *runner.RunnerSet[runner.Runner], s schedule, ch chan<- result) {
	results, err := r.RunSchedule(s)
	if err != nil {
		ch <- result{nil, nil, err}
		return
	}
	log.Debugf("run tests %v -> %v", s, results.Results)

	ch <- result{schedule: s, outcome: results.Results, err: nil}
}

func mergeSchedules(s1 schedule, s2 schedule, revIndex map[string]int) schedule {
	result := schedule{}
	i, j := 0, 0

	for i < len(s1) && j < len(s2) {
		if revIndex[s1[i]] < revIndex[s2[j]] {
			result = append(result, s1[i])
			i++
		} else if revIndex[s1[i]] > revIndex[s2[j]] {
			result = append(result, s2[j])
			j++
		} else {
			i++
			j++
		}
	}

	if i == len(s1) {
		result = append(result, s2[j:]...)
	} else {
		result = append(result, s1[i:]...)
	}

	return result
}

func MEMFAST(tests []string, r *runner.RunnerSet[runner.Runner]) (DependencyGraph, error) {

	revIndex := buildReverseIndex(tests)

	g := NewDependencyGraph(tests)

	log.Info("starting dependency detection algorithm")
	t, err := newTable(tests, r)
	if err != nil {
		return nil, err
	}

	for rank := 1; rank < len(tests); rank++ {
		t.table = append(t.table, []schedule{})
		n := sync.WaitGroup{}
		ch := make(chan result)

		for test := range t.failed {
			for _, seq := range t.table[rank-1] {
				if revIndex[seq[len(seq)-1]] > revIndex[test] {
					continue
				}
				n.Add(1)
				go func(s schedule) {
					defer n.Done()
					runSchedule(r, s, ch)
				}(append(seq, test))
			}
		}

		go func() {
			n.Wait()
			close(ch)
		}()

		for res := range ch {
			if res.err != nil {
				close(ch)
				return nil, err
			}

			if firstFailed := slices.Index(res.outcome, false); firstFailed == -1 {
				passedTest := res.schedule[len(res.schedule)-1]
				last := len(res.schedule) - 1

				t.table[last] = append(t.table[last], res.schedule)

				if _, ok := t.failed[passedTest]; ok {
					delete(t.failed, passedTest)
					g.addDependency(passedTest, res.schedule[len(res.schedule)-2])
				}

				log.Infof("done with test: %s", res.schedule[len(res.schedule)-1])

			} else if firstFailed != len(res.outcome)-1 {
				n.Add(1)
				go func(s schedule) {
					defer n.Done()
					runSchedule(r, s, ch)
				}(res.schedule)
			}
		}

		if len(t.failed) == 0 {
			break
		} else if _, ok := t.failed[tests[rank]]; !ok {
			log.Infof("finished on rank %d, size of not passed: %d", rank, len(t.failed))
			continue
		}

		log.Info("starting brute force")
		test := tests[rank]

		for prefixLen := 2; prefixLen <= rank; prefixLen++ {
			ch = make(chan result)
			for base := 1; base <= prefixLen/2; base++ {
				for _, s1 := range t.table[base-1] {
					if revIndex[s1[len(s1)-1]] > revIndex[test] {
						continue
					}

					for _, s2 := range t.table[prefixLen-base-1] {
						if revIndex[s2[len(s2)-1]] > revIndex[test] {
							continue
						}

						n.Add(1)
						go func(s1, s2 schedule, test string) {
							defer n.Done()

							mergedSchedule := mergeSchedules(s1, s2, revIndex)
							if len(mergedSchedule) == 0 {
								return
							} else if !t.uniqueInsert(mergedSchedule) {
								return
							}

							s := append(mergedSchedule, test)

							fmt.Printf("trying schedule %v\n", s)

							runSchedule(r, s, ch)
						}(s1, s2, test)

					}
				}
			}

			go func() {
				n.Wait()
				close(ch)
			}()

			for res := range ch {
				if res.err != nil {
					close(ch)
					return nil, err
				}

				if res.Passed() {
					last := len(res.schedule) - 1
					log.Infof("done with test: %s, schedule %v",
						res.schedule[last], res.schedule)
					t.table[last] = append(t.table[last], res.schedule)
					delete(t.failed, res.schedule[last])

					for i := 0; i < len(res.schedule)-1; i++ {
						g.addDependency(res.schedule[len(res.schedule)-1],
							res.schedule[i])
					}
				}
			}

			if _, ok := t.failed[tests[rank]]; !ok {
				break
			}
		}

		if _, ok := t.failed[tests[rank]]; ok {
			log.Warnf("issue with test: %s", tests[rank])
		}

		log.Infof("finished on rank %d, size of not passed: %d, table: %v", rank, len(t.failed), t)
	}
	log.Info("finished dependency detection algorithm")

	g.transitiveReduction()

	return g, nil
}
