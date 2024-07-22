package algorithms

import (
	"github.com/pako-23/gtdd/internal/runner"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type schedule []string

type result struct {
	schedule    schedule
	failedIndex int
	err         error
}

type state struct {
	table    [][]schedule
	revIndex map[string]int
	failed   map[string]struct{}
	graph    DependencyGraph
}

func newState(tests []string, jobCh chan<- schedule, resultCh <-chan result) (*state, error) {
	var (
		t = &state{
			failed:   map[string]struct{}{},
			revIndex: buildReverseIndex(tests),
			table:    make([][]schedule, len(tests)),
			graph:    NewDependencyGraph(tests),
		}
	)

	go func() {
		for _, test := range tests {
			jobCh <- schedule{test}
		}
	}()

	for i := 0; i < len(tests); i++ {
		res := <-resultCh
		if res.err != nil {
			return nil, res.err
		}

		if res.failedIndex == -1 {
			t.table[0] = append(t.table[0], res.schedule)
		} else {
			t.failed[res.schedule[0]] = struct{}{}
		}
	}

	return t, nil
}

func (t *state) uniqueInsert(s schedule) bool {
	if containSchedule(t.table[len(s)-1], s) {
		return false
	}

	t.table[len(s)-1] = append(t.table[len(s)-1], s)
	return true
}

func (s *schedule) Equal(other schedule) bool {
	if len(*s) != len(other) {
		return false
	}

	for i, test := range *s {
		if test != other[i] {
			return false
		}
	}

	return true
}

func containSchedule(schedules []schedule, s schedule) bool {
	for _, item := range schedules {
		if item.Equal(s) {
			return true
		}
	}

	return false
}

func buildReverseIndex(tests []string) map[string]int {
	revIndex := map[string]int{}
	for index, test := range tests {
		revIndex[test] = index
	}

	return revIndex
}

func mergeSchedules(s1 schedule, s2 schedule, revIndex map[string]int) schedule {
	result := make(schedule, 0, len(s1)+len(s2)+1)
	i, j := 0, 0

	for i < len(s1) && j < len(s2) {
		if revIndex[s1[i]] < revIndex[s2[j]] {
			result = append(result, s1[i])
			i++
		} else if revIndex[s1[i]] > revIndex[s2[j]] {
			result = append(result, s2[j])
			j++
		} else {
			result = append(result, s1[i])
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

func workerMEMFAST(r *runner.RunnerSet, jobCh <-chan schedule, resultCh chan<- result) {
	for job := range jobCh {
		tries := 0
		for {
			out, err := r.RunSchedule(job)
			if err != nil {
				resultCh <- result{nil, 0, err}
				return
			}
			log.Debugf("run tests %v -> %v", job, out.Results)
			tries++
			firstFailed := slices.Index(out.Results, false)
			if firstFailed == -1 || firstFailed == len(out.Results)-1 || tries >= 3 {
				resultCh <- result{schedule: job, failedIndex: firstFailed, err: nil}
				break
			}

		}
	}
}

func appendMEMFAST(s *state, schedules []schedule, jobCh chan<- schedule, resultCh <-chan result) error {
	go func() {
		for _, schedule := range schedules {
			jobCh <- schedule
		}
	}()

	for i := 0; i < len(schedules); i++ {
		res := <-resultCh
		if res.err != nil {
			return res.err
		}

		if res.failedIndex == -1 {
			rank := len(res.schedule) - 1
			s.table[rank] = append(s.table[rank], res.schedule)
			passedTest := res.schedule[rank]
			if _, ok := s.failed[passedTest]; ok {
				delete(s.failed, passedTest)
				s.graph.addDependency(passedTest, res.schedule[len(res.schedule)-2])
				log.Infof("done with test: %s, schedule: %v", res.schedule[len(res.schedule)-1], res.schedule)
			}
		}
	}

	return nil
}

func extensiveSearchMEMFAST(s *state, prefixLen int, jobCh chan<- schedule, resultCh <-chan result) error {
	schedules := make([]schedule, 0, prefixLen*prefixLen)

	for base := 1; base <= prefixLen/2; base++ {
		for test := range s.failed {
			for _, s1 := range s.table[base-1] {
				if s.revIndex[s1[len(s1)-1]] > s.revIndex[test] {
					continue
				}

				for _, s2 := range s.table[prefixLen-base-1] {
					if s.revIndex[s2[len(s2)-1]] > s.revIndex[test] {
						continue
					}

					testSchedule := mergeSchedules(s1, s2, s.revIndex)

					if len(testSchedule) == 0 {
						continue
					}

					testSchedule = append(testSchedule, test)
					if !containSchedule(schedules, testSchedule) {
						schedules = append(schedules, testSchedule)
					}
				}
			}
		}
	}

	go func() {
		for _, schedule := range schedules {
			jobCh <- schedule
		}
	}()

	passing := make([]schedule, 0, prefixLen*prefixLen*len(s.failed))
	storeSchedules := func(prefix schedule) {
		for item := range s.failed {
			if item != prefix[len(prefix)-1] {
				s := make(schedule, len(prefix)+1)
				copy(s, prefix)
				s[len(s)-1] = item
				passing = append(passing, s)
			}
		}
	}

	for i := 0; i < len(schedules); i++ {
		res := <-resultCh
		if res.err != nil {
			return res.err
		}

		if res.failedIndex == -1 {
			last := len(res.schedule) - 1
			passedTest := res.schedule[last]

			if _, ok := s.failed[passedTest]; ok {
				s.table[last] = append(s.table[last], res.schedule)
				delete(s.failed, res.schedule[last])

				for i := 0; i < len(res.schedule)-1; i++ {
					s.graph.addDependency(passedTest, res.schedule[i])
				}

			}

			storeSchedules(res.schedule)
			prefix := res.schedule[:len(res.schedule)-1]
			s.uniqueInsert(prefix)
		} else if res.failedIndex == len(res.schedule)-1 {
			s.uniqueInsert(res.schedule[:len(res.schedule)-1])
		}
	}

	if err := appendMEMFAST(s, passing, jobCh, resultCh); err != nil {
		return err
	}

	return nil
}

func MEMFAST(tests []string, r *runner.RunnerSet) (DependencyGraph, error) {
	resultCh := make(chan result)
	jobCh := make(chan schedule, r.Size())

	for i := 0; i < r.Size(); i++ {
		go workerMEMFAST(r, jobCh, resultCh)
	}

	log.Info("starting dependency detection algorithm")
	s, err := newState(tests, jobCh, resultCh)
	if err != nil {
		close(jobCh)
		close(resultCh)
		return nil, err
	}

	for rank := 1; rank < len(tests); rank++ {
		schedules := make([]schedule, 0, len(s.failed)*len(s.table[rank-1]))
		for test := range s.failed {
			for _, seq := range s.table[rank-1] {
				if s.revIndex[seq[len(seq)-1]] > s.revIndex[test] {
					continue
				}

				newSchedule := make(schedule, len(seq)+1)
				copy(newSchedule, seq)
				newSchedule[len(newSchedule)-1] = test
				schedules = append(schedules, newSchedule)
			}
		}

		if err := appendMEMFAST(s, schedules, jobCh, resultCh); err != nil {
			close(jobCh)
			close(resultCh)
			return nil, err
		}

		if len(s.failed) == 0 {
			break
		} else if _, ok := s.failed[tests[rank]]; !ok {
			log.Infof("done with rank %d, tests not passed are: %d", rank, len(s.failed))
			continue
		}

		log.Debugf("bruteforce test: %s started", tests[rank])
		for prefixLen := 2; prefixLen <= rank; prefixLen++ {
			if err := extensiveSearchMEMFAST(s, prefixLen, jobCh, resultCh); err != nil {
				close(jobCh)
				close(resultCh)
				return nil, err
			}

			if _, ok := s.failed[tests[rank]]; !ok {
				break
			}
		}

		if _, ok := s.failed[tests[rank]]; ok {
			log.Warnf("issue with test: %s", tests[rank])
		}

	}
	log.Info("finished dependency detection algorithm")

	s.graph.transitiveReduction()

	return s.graph, nil
}
