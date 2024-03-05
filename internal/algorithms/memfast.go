package algorithms

import (
	"sync"

	runner "github.com/pako-23/gtdd/internal/runner/compose-runner"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type MEMFAST struct{}

type schedule []string

type result struct {
	schedule schedule
	outcome  []bool
	err      error
}

type table [][]schedule

func (r result) Passed() bool {
	for _, outcome := range r.outcome {
		if !outcome {
			return false
		}
	}
	return true
}

func (t table) getParallelSchedules() ParallelSchedules {
	initialScan := []schedule{}
	result := ParallelSchedules{}
	covered := map[string]struct{}{}

	for _, schedules := range t {
		for _, s := range schedules {
			if _, ok := covered[s[len(s)-1]]; !ok {
				initialScan = append(initialScan, s)
				covered[s[len(s)-1]] = struct{}{}
			}
		}
	}

	covered = map[string]struct{}{}
	for i := len(initialScan) - 1; i >= 0; i++ {
		toAdd := false
		for _, test := range initialScan[i] {
			if _, ok := covered[test]; !ok {
				toAdd = true
				covered[test] = struct{}{}
			}
		}
		if toAdd {
			result = append(result, initialScan[i])
		}
	}

	return result
}

func buildReverseIndex(tests []string) map[string]int {
	revIndex := map[string]int{}
	for index, test := range tests {
		revIndex[test] = index
	}

	return revIndex
}

func runSchedule(r *runner.RunnerSet, s schedule, ch chan<- result) {
	runnerId, err := r.Reserve()
	if err != nil {
		ch <- result{nil, nil, err}
		return
	}
	defer func() { go r.Release(runnerId) }()
	results, err := r.Get(runnerId).Run(s)
	if err != nil {
		ch <- result{nil, nil, err}
		return
	}
	log.Debugf("run tests %v -> %v", s, results)

	ch <- result{schedule: s, outcome: results, err: nil}
}

func initTable(tests []string, r *runner.RunnerSet) (map[string]struct{}, table, error) {

	var (
		notPassed = map[string]struct{}{}
		t         = table{[]schedule{}}
		n         = sync.WaitGroup{}
		ch        = make(chan result)
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
			return nil, nil, res.err
		}

		if res.outcome[0] {
			t[0] = append(t[0], res.schedule)
		} else {
			notPassed[res.schedule[0]] = struct{}{}
		}
	}

	log.Infof("table: %v", t)
	log.Infof("not passed: %v", notPassed)

	return notPassed, t, nil
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

func (*MEMFAST) FindDependencies(tests []string, r *runner.RunnerSet) (DetectorArtifact, error) {

	revIndex := buildReverseIndex(tests)

	log.Info("starting dependency detection algorithm")
	log.Infof("detection on test suite: %v", tests)
	notPassed, t, err := initTable(tests, r)
	if err != nil {
		return nil, err
	}

	for rank := 1; rank < len(tests); rank++ {
		t = append(t, []schedule{})
		n := sync.WaitGroup{}
		ch := make(chan result)

		for test := range notPassed {
			for _, seq := range t[rank-1] {
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

		someTestPassed := false
		for res := range ch {
			if res.err != nil {
				close(ch)
				return nil, err
			}

			if firstFailed := slices.Index(res.outcome, false); firstFailed == -1 {
				t[len(res.schedule)-1] = append(t[len(res.schedule)-1], res.schedule)
				delete(notPassed, res.schedule[len(res.schedule)-1])
				someTestPassed = true
				log.Infof("done with test: %s", res.schedule[len(res.schedule)-1])
			} else if firstFailed != len(res.outcome)-1 {
				n.Add(1)
				go func(s schedule) {
					defer n.Done()
					runSchedule(r, s, ch)
				}(res.schedule)
			}
		}

		if len(notPassed) == 0 {
			break
		} else if _, ok := notPassed[tests[rank]]; !ok && someTestPassed {
			log.Infof("finished on rank %d, size of not passed: %d", rank, len(notPassed))
			continue
		}

		log.Info("starting brute force")

		for maxLen := 2; maxLen <= rank; maxLen++ {
			ch = make(chan result)
			for test := range notPassed {
				for base := 1; base <= maxLen/2; base++ {
					for _, s1 := range t[base-1] {
						if revIndex[s1[len(s1)-1]] > revIndex[test] {
							continue
						}

						for _, s2 := range t[maxLen-base-1] {
							if revIndex[s2[len(s2)-1]] > revIndex[test] {
								continue
							}

							n.Add(1)
							go func(s1, s2 schedule, test string) {
								defer n.Done()
								s := append(mergeSchedules(s1, s2, revIndex), test)
								runSchedule(r, s, ch)
							}(s1, s2, test)

						}
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
					log.Infof("done with test: %s, schedule %v", res.schedule[len(res.schedule)-1], res.schedule)
					t[len(res.schedule)-1] = append(t[len(res.schedule)-1], res.schedule)
					delete(notPassed, res.schedule[len(res.schedule)-1])
				}
			}

			if _, ok := notPassed[tests[rank]]; !ok {
				break
			}
		}

		if _, ok := notPassed[tests[rank]]; ok {
			log.Warnf("issue with test: %s", tests[rank])
		}

		log.Infof("finished on rank %d, size of not passed: %d, table: %v", rank, len(notPassed), t)
	}
	log.Info("finished dependency detection algorithm")

	return t.getParallelSchedules(), nil
}
