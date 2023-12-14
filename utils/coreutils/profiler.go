package coreutils

import (
	"errors"
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

const (
	defaultInterval    = time.Second
	defaultRepetitions = 3
)

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
		err = errors.Join(err, os.Remove(outputFilePath))
	}()
	return p.convertFileToString(outputFilePath)
}

func (p *Profiler) threadDumpToFile() (outputFilePath string, err error) {
	outputFile, err := fileutils.CreateTempFile()
	if err != nil {
		return
	}
	defer func() {
		err = errors.Join(err, outputFile.Close())
	}()

	for i := 0; i < int(p.repetitions); i++ {
		fmt.Fprintf(outputFile, "========== Thread dump #%d ==========\n", i)
		prof := pprof.Lookup("goroutine")
		if err = prof.WriteTo(outputFile, 1); err != nil {
			return
		}
		time.Sleep(p.interval)
	}
	return outputFile.Name(), nil
}

func (p *Profiler) convertFileToString(outputFilePath string) (output string, err error) {
	var outputBytes []byte
	if outputBytes, err = os.ReadFile(outputFilePath); err != nil {
		return
	}
	output = string(outputBytes)
	return
}
