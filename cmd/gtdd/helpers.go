package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pako-23/gtdd/internal/algorithms"
	"github.com/pako-23/gtdd/internal/runner"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type runResults struct {
	results  []bool
	schedule []string
	time     time.Duration
}

func runSchedules(schedules [][]string, runners *runner.RunnerSet) (time.Duration, error) {
	scheduleCh := make(chan []string, runners.Size())
	errCh, resultsCh := make(chan error), make(chan runResults, runners.Size())

	for i := 0; i < runners.Size(); i++ {
		go func() {
			for schedule := range scheduleCh {
				out, err := runners.RunSchedule(schedule)
				if err != nil {
					errCh <- err

					continue
				}

				resultsCh <- runResults{
					schedule: schedule,
					results:  out.Results,
					time:     out.RunningTime}

			}
		}()
	}

	go func() {
		for _, schedule := range schedules {
			scheduleCh <- schedule
		}
	}()

	var (
		errorMessages = []string{}
		duration      time.Duration
	)

	for i := 0; i < len(schedules); i++ {
		select {
		case err := <-errCh:
			return 0, err
		case result := <-resultsCh:
			failed := slices.Index(result.results, false)
			if failed != -1 {
				msg := fmt.Sprintf("test %v failed in schedule %v",
					result.schedule[failed], result.schedule)
				errorMessages = append(errorMessages, msg)
			}

			log.Infof("run schedule in %v", result.time)
			if duration < result.time {
				duration = result.time
			}
		}
	}
	close(scheduleCh)
	close(resultsCh)

	if len(errorMessages) > 0 {
		return 0, errors.New(strings.Join(errorMessages, "\n"))
	}

	return duration, nil
}

func getSchedules(tests []string, inputFileName string) ([][]string, error) {
	if inputFileName == "" {
		return [][]string{tests}, nil
	}
	graph, err := algorithms.DependencyGraphFromJson(inputFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedules from graph: %w", err)
	}

	return graph.GetSchedules(tests), err
}

func getDetector(strategy string) algorithms.DependencyDetector {
	switch strategy {
	case "pfast":
		return algorithms.PFAST
	case "pradet":
		return algorithms.PraDet
	case "mem-fast":
		return algorithms.MEMFAST
	default:
		return nil
	}
}
