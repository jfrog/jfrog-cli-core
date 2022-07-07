package transferfiles

import (
	"testing"
)

func TestHandleStopSignals(t *testing.T) {
	transferFilesCommand := NewTransferFilesCommand(nil, nil)

	shouldStop := false
	var newPhase transferPhase
	finishStopping := transferFilesCommand.handleStop(&shouldStop, &newPhase, nil)
	finishStopping()
}
