package transferfiles

import (
	"testing"
)

func TestHandleStopInitAndClose(t *testing.T) {
	transferFilesCommand := NewTransferFilesCommand(nil, nil)

	shouldStop := false
	var newPhase transferPhase
	finishStopping := transferFilesCommand.handleStop(&shouldStop, &newPhase, nil)
	finishStopping()
}
