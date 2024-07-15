package runner

type Runner interface {
	ResetApplication() error
	Delete() error
	Run(tests []string) ([]bool, error)
}

type RunnerOption[T Runner] func(runner T) error
type RunnerBuilder[T Runner] func(name string, options ...RunnerOption[T]) (T, error)
