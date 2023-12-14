package coreutils

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestThreadDump(t *testing.T) {
	// Create default profiler
	profiler := NewProfiler()

	// Start a thread that sleeps
	go func() {
		dummyZzzz()
	}()

	// Run thread dump
	output, err := profiler.ThreadDump()
	assert.NoError(t, err)

	// Check results
	assert.Contains(t, output, "Thread dump #0")
	assert.Contains(t, output, "Thread dump #1")
	assert.Contains(t, output, "Thread dump #2")
	assert.Contains(t, output, "dummyZzzz")
}

func TestThreadInterval(t *testing.T) {
	// Create profiler with 10 repetitions and 10ms intervals
	var expectedRepetitions uint = 10
	var expectedInterval = 10 * time.Millisecond
	profiler := NewProfiler(WithInterval(expectedInterval), WithRepetitions(expectedRepetitions))

	// Check that the required values are set
	assert.Equal(t, profiler.interval, expectedInterval)
	assert.Equal(t, profiler.repetitions, expectedRepetitions)

	// start measure the time
	start := time.Now()

	// Run thread dump
	output, err := profiler.ThreadDump()
	assert.NoError(t, err)

	// Ensure duration less than 1 second
	assert.WithinDuration(t, start, time.Now(), time.Second)

	// Ensure 10 repetitions
	assert.Contains(t, output, "Thread dump #"+strconv.FormatUint(uint64(expectedRepetitions)-1, 10))
}

func dummyZzzz() {
	time.Sleep(2 * time.Second)
}
