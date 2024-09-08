package precheckrunner

import (
	"context"
	"fmt"
	"github.com/jfrog/gofrog/safeconvert"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestChecks(t *testing.T) {
	// Empty runner
	runner := NewPreChecksRunner()
	runner.AddCheck(nil)
	assert.Len(t, runner.checks, 0)
	runner.AddCheck(NewCheck("check", nil))
	assert.Len(t, runner.checks, 1)
}

func TestRunChecks(t *testing.T) {
	// Init
	expectedErr := fmt.Errorf("CHECK_ERROR")
	nSuccess := 3
	nFail := 2
	runner := NewPreChecksRunner()
	successCheck := NewCheck("success", func(args RunArguments) (bool, error) {
		return true, nil
	})
	failCheck := NewCheck("fail", func(args RunArguments) (bool, error) {
		return false, nil
	})
	errCheck := NewCheck("error", func(args RunArguments) (bool, error) {
		return false, expectedErr
	})
	// Empty
	runAndAssert(t, 0, 0, nil, runner)
	// With checks
	for i := 0; i < nSuccess; i++ {
		runner.AddCheck(successCheck)
	}
	unsignedNumSuccess, err := safeconvert.IntToUint(nSuccess)
	assert.NoError(t, err)
	runAndAssert(t, unsignedNumSuccess, 0, nil, runner)
	// With failed checks
	for i := 0; i < nFail; i++ {
		runner.AddCheck(failCheck)
	}
	unsignedNumSuccess, err = safeconvert.IntToUint(nSuccess)
	assert.NoError(t, err)
	unsignedNumFail, err := safeconvert.IntToUint(nFail)
	assert.NoError(t, err)
	runAndAssert(t, unsignedNumSuccess, unsignedNumFail, nil, runner)
	// With check that has error
	runner.AddCheck(errCheck)
	runAndAssert(t, 0, 0, expectedErr, runner)
}

func runAndAssert(t *testing.T, expectedSuccess, expectedFail uint, shouldHaveErr error, runner *PreCheckRunner) {
	err := runner.Run(context.TODO(), nil)
	if shouldHaveErr != nil {
		assert.Errorf(t, err, shouldHaveErr.Error())
	} else {
		assert.NoError(t, err)
		assert.Equal(t, expectedSuccess, runner.status.successes)
		assert.Equal(t, expectedFail, runner.status.failures)
		signedExpectedSuccess, err := safeconvert.UintToInt(expectedSuccess)
		assert.NoError(t, err)
		signedExpectedFail, err := safeconvert.UintToInt(expectedFail)
		assert.NoError(t, err)
		assert.Len(t, runner.checks, signedExpectedSuccess+signedExpectedFail)
	}
}
