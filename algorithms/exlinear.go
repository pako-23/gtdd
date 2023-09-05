package algorithms

import (
	"sync"
	"time"

	"github.com/pako-23/gtdd/runners"
)

func remove(s []string, index int) []string {
	ret := make([]string, 0)
	ret = append(ret, s[:index]...)

	return append(ret, s[index+1:]...)
}

func findFailed(results []bool) int {
	for i, value := range results {
		if !value {
			return i
		}
	}

	return -1
}

// TODO: add docs here (+ find name)
func ExLinear(tests []string, oracle *runners.RunnerSet) (DependencyGraph, error) {
	type data struct {
		edge
		err error
	}
	ch, n, g := make(chan data), sync.WaitGroup{}, NewDependencyGraph(tests)
	var iteration func([]string, int, int)

	iteration = func(tests_list []string, i, j int) {
		defer n.Done()
		schedule := remove(tests, j)
		runnerId, err := oracle.Reserve()
		if err != nil {
			ch <- data{err: err}
			return
		}
		defer oracle.Release(runnerId)
		time.Sleep(time.Second * 30)
		results, err := oracle.Get(runnerId).Run(schedule)
		if err != nil {
			ch <- data{err: err}
			return
		}

		firstFailed := findFailed(results)
		if firstFailed == -1 {
			ch <- data{}
		} else {
			ch <- data{edge: edge{from: schedule[firstFailed], to: tests[i]}}
			n.Add(1)
			go iteration(schedule, i, firstFailed)
		}
	}

	for i := 0; i < len(tests)-1; i++ {
		n.Add(1)
		go iteration(tests, i, i)
	}

	go func() {
		n.Wait()
		close(ch)
	}()

	for result := range ch {
		if result.err != nil {
			return nil, result.err
		} else if result.from != "" && result.to != "" {
			g.AddDependency(result.from, result.to)
		}
	}

	return g, nil
}
