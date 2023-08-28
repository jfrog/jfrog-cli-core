package commands

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/pipelines/services"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"strings"
	"time"
)

type WorkspaceSyncStatusCommand struct {
	serverDetails *config.ServerDetails
	project       string
}

func NewWorkspaceSyncStatusCommand() *WorkspaceSyncStatusCommand {
	return &WorkspaceSyncStatusCommand{}
}

func (wss *WorkspaceSyncStatusCommand) ServerDetails() (*config.ServerDetails, error) {
	return wss.serverDetails, nil
}

func (wss *WorkspaceSyncStatusCommand) SetServerDetails(serverDetails *config.ServerDetails) *WorkspaceSyncStatusCommand {
	wss.serverDetails = serverDetails
	return wss
}

func (wss *WorkspaceSyncStatusCommand) SetProject(p string) *WorkspaceSyncStatusCommand {
	wss.project = p
	return wss
}

func (wss *WorkspaceSyncStatusCommand) CommandName() string {
	return "pl_workspace_sync_status"
}

func (wss *WorkspaceSyncStatusCommand) Run() error {
	serviceManager, err := manager.CreateServiceManager(wss.serverDetails)
	if err != nil {
		return err
	}
	var response []services.WorkspacesResponse
	response, err = serviceManager.WorkspacePollSyncStatus()
	if err != nil {
		return err
	}
	log.Info("Workspace sync status : \n")
	for _, syncStatusResp := range response {
		if strings.EqualFold(syncStatusResp.Name, wss.project) {
			syncStatusOutput, err := json.MarshalIndent(syncStatusResp, "", "  ")
			if err != nil {
				return err
			}
			fmt.Print(string(syncStatusOutput))
		}
	}
	return nil
}

type SyncStatusInfo struct {
	ID                 int       `json:"id"`
	Name               string    `json:"name"`
	ProjectID          int       `json:"projectId"`
	IsSyncing          bool      `json:"isSyncing"`
	LastSyncStatusCode int       `json:"lastSyncStatusCode"`
	LastSyncStartedAt  time.Time `json:"lastSyncStartedAt"`
	LastSyncEndedAt    time.Time `json:"lastSyncEndedAt"`
	LastSyncLogs       string    `json:"lastSyncLogs"`
	CreatedBy          int       `json:"createdBy"`
}
