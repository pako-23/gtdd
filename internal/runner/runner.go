package runner

type Runner interface {
	ResetApplication() error
	Delete() error
	Run(tests []string) ([]bool, error)
	Id() string
}

type RunnerOption[T Runner] func(runner T) error
type RunnerBuilder[T Runner] func(id string, options ...RunnerOption[T]) (T, error)
