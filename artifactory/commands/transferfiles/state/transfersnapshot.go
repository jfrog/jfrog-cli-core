package state

import (
	"sync"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/reposnapshot"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

var saveRepoSnapshotMutex sync.Mutex

type SnapshotActionFunc func(rts *RepoTransferSnapshot) error

var snapshotSaveIntervalMin = snapshotSaveIntervalMinDefault

const snapshotSaveIntervalMinDefault = 10

// RepoTransferSnapshot handles saving and loading the repository's transfer snapshot.
// A repository transfer snapshot stores the progress of transferring a repository, as explained in RepoSnapshotManager.
// In case a transfer is interrupted, the transfer can continue from it's saved snapshot instead of starting from scratch.
type RepoTransferSnapshot struct {
	snapshotManager   reposnapshot.RepoSnapshotManager
	lastSaveTimestamp time.Time
	// This boolean marks that this snapshot continues a previous run. It allows skipping certain checks if it was not loaded, because some data is known to be new.
	loadedFromSnapshot bool
}

// Runs the provided action on the snapshot manager, and periodically saves the repo state and snapshot to the snapshot dir.
func (ts *TransferStateManager) snapshotAction(action SnapshotActionFunc) (err error) {
	if ts.repoTransferSnapshot == nil {
		return errorutils.CheckErrorf("invalid call to snapshot manager before it was initialized")
	}
	if err = action(ts.repoTransferSnapshot); err != nil {
		return err
	}

	now := time.Now()
	if now.Sub(ts.repoTransferSnapshot.lastSaveTimestamp).Minutes() < float64(snapshotSaveIntervalMin) {
		return nil
	}

	if !saveRepoSnapshotMutex.TryLock() {
		return nil
	}
	defer saveRepoSnapshotMutex.Unlock()

	ts.repoTransferSnapshot.lastSaveTimestamp = now
	if err = ts.repoTransferSnapshot.snapshotManager.PersistRepoSnapshot(); err != nil {
		return err
	}

	return ts.persistTransferState(true)
}

func (ts *TransferStateManager) LookUpNode(relativePath string) (requestedNode *reposnapshot.Node, err error) {
	err = ts.snapshotAction(func(rts *RepoTransferSnapshot) error {
		requestedNode, err = rts.snapshotManager.LookUpNode(relativePath)
		return err
	})
	return
}

func (ts *TransferStateManager) WasSnapshotLoaded() (wasLoaded bool, err error) {
	err = ts.snapshotAction(func(rts *RepoTransferSnapshot) error {
		wasLoaded = rts.loadedFromSnapshot
		return nil
	})
	return
}

func (ts *TransferStateManager) GetDirectorySnapshotNodeWithLru(relativePath string) (node *reposnapshot.Node, err error) {
	err = ts.snapshotAction(func(rts *RepoTransferSnapshot) error {
		node, err = rts.snapshotManager.GetDirectorySnapshotNodeWithLru(relativePath)
		return err
	})
	return
}

func (ts *TransferStateManager) DisableRepoTransferSnapshot() {
	ts.repoTransferSnapshot = nil
}

func (ts *TransferStateManager) IsRepoTransferSnapshotEnabled() bool {
	return ts.repoTransferSnapshot != nil
}

func loadRepoTransferSnapshot(repoKey, snapshotFilePath string) (*RepoTransferSnapshot, bool, error) {
	snapshotManager, exists, err := reposnapshot.LoadRepoSnapshotManager(repoKey, snapshotFilePath)
	if err != nil || !exists {
		return nil, exists, err
	}
	return &RepoTransferSnapshot{snapshotManager: snapshotManager, lastSaveTimestamp: time.Now(), loadedFromSnapshot: true}, true, nil
}

func createRepoTransferSnapshot(repoKey, snapshotFilePath string) *RepoTransferSnapshot {
	return &RepoTransferSnapshot{snapshotManager: reposnapshot.CreateRepoSnapshotManager(repoKey, snapshotFilePath), lastSaveTimestamp: time.Now()}
}
