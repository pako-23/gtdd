package runner_test

import (
	"errors"
	"math/rand"
	"sync"
	"testing"

	"github.com/pako-23/gtdd/internal/runner"
	"gotest.tools/v3/assert"
)

var errInjectedFailure = errors.New("injected failure")

type mockRunner struct {
	name       string
	failDelete bool
	failReset  bool
}

func newMockRunnerBuilder(id string, options ...runner.RunnerOption[*mockRunner]) (*mockRunner, error) {
	r := &mockRunner{
		name:       id,
		failDelete: false,
		failReset:  false,
	}

	for _, option := range options {
		if err := option(r); err != nil {
			return nil, err
		}
	}

	return r, nil
}

func withFailDelete() func(*mockRunner) error {
	return func(r *mockRunner) error {
		r.failDelete = true
		return nil
	}
}

func withFailReset() func(*mockRunner) error {
	return func(r *mockRunner) error {
		r.failReset = true
		return nil
	}
}

func withFailOption() func(*mockRunner) error {
	return func(r *mockRunner) error {
		return errInjectedFailure
	}
}

func (m *mockRunner) ResetApplication() error {
	if m.failReset {
		return errInjectedFailure
	}

	return nil
}

func (m *mockRunner) Delete() error {
	if m.failDelete {
		return errInjectedFailure
	}

	return nil
}

func (m *mockRunner) Run(tests []string) ([]bool, error) {
	results := make([]bool, len(tests))

	for i := range tests {
		results[i] = tests[i] == "PASS"
	}

	return results, nil
}

func (m *mockRunner) Id() string {
	return m.name
}

func TestNewRunnerSet(t *testing.T) {
	t.Parallel()

	for i := 0; i < 100; i++ {
		size := rand.Intn(10) + 1
		set, err := runner.NewRunnerSet(size, newMockRunnerBuilder)

		assert.NilError(t, err)
		assert.Equal(t, set.Size(), size)
	}
}

func TestNewRunnerSetInvalidSize(t *testing.T) {
	t.Parallel()

	for i := 0; i < 100; i++ {
		_, err := runner.NewRunnerSet(-rand.Intn(10), newMockRunnerBuilder)
		assert.ErrorIs(t, err, runner.ErrWrongRunnerSetSize)
	}
}

func TestRunnerSetDelete(t *testing.T) {
	t.Parallel()

	for i := 0; i < 100; i++ {
		set, err := runner.NewRunnerSet(rand.Intn(10)+1, newMockRunnerBuilder)

		assert.NilError(t, err)
		assert.NilError(t, set.Delete())
	}
}

func TestRunSchedule(t *testing.T) {
	t.Parallel()

	for i := 0; i < 100; i++ {
		var n sync.WaitGroup

		set, err := runner.NewRunnerSet(rand.Intn(10)+1, newMockRunnerBuilder)
		assert.NilError(t, err)

		size := rand.Intn(15) + 1

		for j := 0; j < size; j++ {
			n.Add(1)
			go func() {
				defer n.Done()
				schedule := make([]string, rand.Intn(10)+1)

				for k := range schedule {
					if rand.Intn(2) == 1 {
						schedule[k] = "PASS"
					} else {
						schedule[k] = "FAIL"
					}
				}

				results, err := set.RunSchedule(schedule)

				assert.NilError(t, err)
				assert.Equal(t, len(schedule), len(results.Results))

				for k := range schedule {
					assert.Equal(t, schedule[k] == "PASS", results.Results[k])
				}

				assert.Check(t, results.RunningTime > 0)
			}()
		}

		n.Wait()
	}
}
