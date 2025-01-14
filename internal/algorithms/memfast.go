package algorithms

import (
	"strings"

	"github.com/pako-23/gtdd/internal/runner"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type schedule []string

type scheduleSet map[string]schedule

type result struct {
	schedule    schedule
	failedIndex int
	err         error
}

type state struct {
	table    []scheduleSet
	revIndex map[string]int
	failed   map[string]struct{}
	graph    DependencyGraph
	max      int
	runned   map[string]struct{}
}

func (s scheduleSet) Insert(sched schedule) {
	s[strings.Join(sched, ",")] = sched
}

func newState(tests []string, jobCh chan<- schedule, resultCh <-chan result) (*state, error) {
	var (
		t = &state{
			failed:   map[string]struct{}{},
			revIndex: buildReverseIndex(tests),
			table:    make([]scheduleSet, len(tests)),
			graph:    NewDependencyGraph(tests),
			max:      0,
			runned:   map[string]struct{}{},
		}
	)

	for i := 0; i < len(t.table); i++ {
		t.table[i] = make(scheduleSet)
	}

	go func() {
		for _, test := range tests {
			jobCh <- schedule{test}
		}
	}()

	for range tests {
		res := <-resultCh
		if res.err != nil {
			return nil, res.err
		}

		t.Register(res.schedule)
		if res.failedIndex == -1 {
			t.table[0].Insert(res.schedule)
		} else {
			t.failed[res.schedule[0]] = struct{}{}
		}
	}

	return t, nil
}

func (s *state) Register(sched schedule) {
	s.runned[strings.Join(sched, ",")] = struct{}{}
}

func (s *state) Runned(sched schedule) bool {
	_, ok := s.runned[strings.Join(sched, ",")]
	return ok
}

func (s *state) TableInsert(sched schedule) {
	s.table[len(sched)-1].Insert(sched)
	if len(sched)-1 > s.max {
		s.max = len(sched) - 1
	}
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

	for range schedules {
		res := <-resultCh
		if res.err != nil {
			return res.err
		}

		if res.failedIndex != -1 {
			continue
		}
		s.TableInsert(res.schedule)
		passedTest := res.schedule[len(res.schedule)-1]
		if _, ok := s.failed[passedTest]; ok {
			delete(s.failed, passedTest)
			s.graph.AddDependency(passedTest, res.schedule[len(res.schedule)-2])
			log.Infof("done with test: %s, schedule: %v", passedTest, res.schedule)
		}
	}

	return nil
}

func extensiveSearchMEMFAST(s *state, prefixLen, rank int, jobCh chan<- schedule, resultCh <-chan result) error {
	schedules := make(scheduleSet, prefixLen*prefixLen)

	for base := 1; base < prefixLen; base++ {
		for _, s1 := range s.table[base-1] {
			for idx := prefixLen - base - 1; idx <= s.max; idx++ {
				for _, s2 := range s.table[idx] {
					for test := range s.failed {
						if s.revIndex[s1[len(s1)-1]] > s.revIndex[test] {
							continue
						}

						if s.revIndex[s2[len(s2)-1]] > s.revIndex[test] {
							continue
						}

						testSchedule := mergeSchedules(s1, s2, s.revIndex)
						if len(testSchedule) == 0 || len(testSchedule) != prefixLen {
							continue
						}
						testSchedule = append(testSchedule, test)
						if !s.Runned(testSchedule) {
							schedules.Insert(testSchedule)
							s.Register(testSchedule)
						}
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

	passing := make(scheduleSet, len(schedules))

	for range schedules {
		res := <-resultCh
		if res.err != nil {
			return res.err
		}

		if res.failedIndex == -1 {
			last := len(res.schedule) - 1
			passedTest := res.schedule[last]

			scheduleCopy := make(schedule, len(res.schedule)-1)
			copy(scheduleCopy, res.schedule[:len(res.schedule)-1])
			s.TableInsert(scheduleCopy)

			if _, ok := s.failed[passedTest]; ok {
				s.TableInsert(res.schedule)
				delete(s.failed, res.schedule[last])

				for i := 0; i < len(res.schedule)-1; i++ {
					s.graph.AddDependency(passedTest, res.schedule[i])
				}
			}

			if len(res.schedule) <= rank {
				passing.Insert(res.schedule)
			}

		} else if res.failedIndex == len(res.schedule)-1 {
			s.TableInsert(res.schedule[:len(res.schedule)-1])
		}
	}

	for len(passing) > 0 {
		schedules := make([]schedule, 0, len(passing)*len(s.failed))
		for _, prefix := range passing {
			for test := range s.failed {
				if s.revIndex[test] < s.revIndex[prefix[len(prefix)-1]] {
					continue
				}

				sched := make(schedule, len(prefix)+1)
				copy(sched, prefix)
				sched[len(sched)-1] = test
				schedules = append(schedules, sched)
				s.Register(sched)
			}
		}

		go func() {
			for _, schedule := range schedules {
				jobCh <- schedule
			}
		}()

		updatedPassing := make(scheduleSet, len(schedules))

		for range schedules {
			res := <-resultCh
			if res.err != nil {
				return res.err
			}

			if res.failedIndex != -1 {
				continue
			}

			last := len(res.schedule) - 1
			passedTest := res.schedule[last]

			s.TableInsert(res.schedule)

			delete(s.failed, res.schedule[last])

			s.graph.AddDependency(passedTest, res.schedule[len(res.schedule)-2])
			if len(res.schedule) <= rank {
				updatedPassing.Insert(res.schedule)
			}
		}

		passing = updatedPassing
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
				if !s.Runned(newSchedule) {
					schedules = append(schedules, newSchedule)
					s.Register(newSchedule)
				}
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
			if err := extensiveSearchMEMFAST(s, prefixLen, rank, jobCh, resultCh); err != nil {
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

	s.graph.TransitiveReduction()

	return s.graph, nil
}
