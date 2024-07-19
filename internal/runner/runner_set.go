package runner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// The default number of runners into a set of runners.
const DefaultSetSize = 1

var (
	ErrNoRunner           = errors.New("no runner to reserve")
	ErrWrongRunnerSetSize = errors.New("a runner set must have at least size 1")
)

type RunResults struct {
	Results     []bool
	RunningTime time.Duration
}

// RunnerSet represents a group of runners used to run a test suites.
type RunnerSet struct {
	runners chan Runner
	reset   chan Runner
	size    atomic.Int32
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewRunnerSet creates a new set of runner with the provided configuration.
// If there is an error in creating the set of runners, it is returned.
func NewRunnerSet[T Runner](size int, builder RunnerBuilder[T], options ...RunnerOption[T]) (*RunnerSet, error) {
	var n sync.WaitGroup

	if size < 1 {
		return nil, ErrWrongRunnerSetSize
	}

	ctx, cancel := context.WithCancel(context.Background())

	set := &RunnerSet{
		runners: make(chan Runner, size),
		reset:   make(chan Runner),
		size:    atomic.Int32{},
		ctx:     ctx,
		cancel:  cancel,
	}

	set.size.Store(int32(size))

	for i := 0; i < size; i++ {
		var runnerName = fmt.Sprintf("runner-%d", i)

		runner, err := builder(runnerName, options...)
		if err != nil {
			if deleteErr := set.Delete(); deleteErr != nil {
				log.Errorf("failed to delete runner %s: %v", runnerName, deleteErr)
			}

			set.cancel()
			return nil, err
		}

		n.Add(1)
		go func() {
			defer n.Done()
			set.release()
		}()

		set.reset <- runner
	}

	n.Wait()

	for i := 0; i < set.Size(); i++ {
		go func() {
			for {
				if !set.release() {
					return
				}
			}
		}()
	}

	log.Infof("successfully initialized %d runners", set.Size())

	return set, nil
}

func (r *RunnerSet) release() bool {
	select {
	case runner := <-r.reset:
		if err := runner.ResetApplication(); err != nil {
			log.Errorf("failed to reset application on runner %s: %v", runner.Id(), err)

			if err = runner.Delete(); err != nil {
				log.Errorf("failed to delete runner %s: %v", runner.Id(), err)
			}

			r.size.Add(-1)
			return false
		}
		r.runners <- runner
		return true
	case <-r.ctx.Done():
		return false
	}

}

func (r *RunnerSet) Size() int {
	return int(r.size.Load())
}

// Delete releases all the resources needed by the set of runners.
// If there is an error in the process, it is returned.
func (r *RunnerSet) Delete() error {
	var waitgroup errgroup.Group

	r.cancel()

	for i := 0; i < r.Size(); i++ {
		runner := <-r.runners

		waitgroup.Go(func(runner Runner) func() error {
			return func() error {
				return runner.Delete()
			}
		}(runner))
	}

	if err := waitgroup.Wait(); err != nil {
		return fmt.Errorf("failed to delete set of runners: %w", err)
	}

	return nil
}

func (r *RunnerSet) RunSchedule(schedule []string) (RunResults, error) {
	if r.Size() == 0 {
		return RunResults{}, ErrNoRunner
	}

	runner := <-r.runners
	start := time.Now()
	result, err := runner.Run(schedule)
	duration := time.Since(start)

	r.reset <- runner

	return RunResults{
		Results:     result,
		RunningTime: duration,
	}, err
}
