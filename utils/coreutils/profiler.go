package coreutils

import (
	"errors"
	"fmt"
	"github.com/jfrog/gofrog/safeconvert"
	"os"
	"runtime/pprof"
	"time"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

const (
	// The default interval between 2 profiling actions
	defaultInterval = time.Second
	// The default number of profilings
	defaultRepetitions = 3
)

// This struct wraps the "pprof" profiler in Go.
// It is used for thread dumping.
type Profiler struct {
	interval    time.Duration
	repetitions uint
}

type ProfilerOption func(*Profiler)

func NewProfiler(opts ...ProfilerOption) *Profiler {
	profiler := &Profiler{
		interval:    defaultInterval,
		repetitions: defaultRepetitions,
	}
	for _, opt := range opts {
		opt(profiler)
	}
	return profiler
}

func WithInterval(interval time.Duration) ProfilerOption {
	return func(p *Profiler) {
		p.interval = interval
	}
}

func WithRepetitions(repetitions uint) ProfilerOption {
	return func(p *Profiler) {
		p.repetitions = repetitions
	}
}

func (p *Profiler) ThreadDump() (output string, err error) {
	var outputFilePath string
	if outputFilePath, err = p.threadDumpToFile(); err != nil {
		return
	}
	defer func() {
		err = errors.Join(err, errorutils.CheckError(os.Remove(outputFilePath)))
	}()
	return p.convertFileToString(outputFilePath)
}

func (p *Profiler) threadDumpToFile() (outputFilePath string, err error) {
	outputFile, err := fileutils.CreateTempFile()
	if err != nil {
		return
	}
	defer func() {
		err = errors.Join(err, errorutils.CheckError(outputFile.Close()))
	}()

	signedRepetitions, err := safeconvert.UintToInt(p.repetitions)
	if err != nil {
		return "", fmt.Errorf("failed to convert repetitions to int: %w", err)
	}
	for i := 0; i < signedRepetitions; i++ {
		fmt.Fprintf(outputFile, "========== Thread dump #%d ==========\n", i)
		prof := pprof.Lookup("goroutine")
		if err = errorutils.CheckError(prof.WriteTo(outputFile, 1)); err != nil {
			return
		}
		time.Sleep(p.interval)
	}
	return outputFile.Name(), nil
}

func (p *Profiler) convertFileToString(outputFilePath string) (string, error) {
	if outputBytes, err := os.ReadFile(outputFilePath); err != nil {
		return "", errorutils.CheckError(err)
	} else {
		return string(outputBytes), nil
	}
}
