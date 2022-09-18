package transferfiles

import (
	"time"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const waitTimeBetweenPropertiesStatusSeconds = 5

type propertiesDiffPhase struct {
	phaseBase
}

func (p *propertiesDiffPhase) initProgressBar() error {
	return nil
}

func (p *propertiesDiffPhase) getPhaseName() string {
	return "Properties Diff Handling Phase"
}

func (p *propertiesDiffPhase) phaseStarted() error {
	p.startTime = time.Now()
	return setPropsDiffHandlingStarted(p.repoKey, p.startTime)
}

func (p *propertiesDiffPhase) phaseDone() error {
	if p.ShouldStop() {
		return nil
	}
	return setPropsDiffHandlingCompleted(p.repoKey)
}

func (p *propertiesDiffPhase) shouldSkipPhase() (bool, error) {
	return isPropertiesPhaseDisabled(), nil
}

func (p *propertiesDiffPhase) run() error {
	diffStart, diffEnd, err := getDiffHandlingRange(p.repoKey)
	if err != nil {
		return err
	}

	requestBody := HandlePropertiesDiff{
		TargetAuth:        createTargetAuth(p.targetRtDetails, p.proxyKey),
		RepoKey:           p.repoKey,
		StartMilliseconds: convertTimeToEpochMilliseconds(diffStart),
		EndMilliseconds:   convertTimeToEpochMilliseconds(diffEnd),
	}

	generalStatus, err := p.makePropsHandlingStatus()
	if err != nil {
		return err
	}

	// Periodically send handling requests to the user plugin to handle properties diff in a time range.
	// Update progress with the status return from those requests.
	// Done handling when all nodes return done status.
propertiesHandling:
	for {
		if p.ShouldStop() {
			return errorutils.CheckError(&InterruptionErr{})
		}
		remoteNodeStatus, err := p.srcUpService.handlePropertiesDiff(requestBody)
		if err != nil {
			return err
		}

		switch remoteNodeStatus.Status {
		case InProgress:
			err = generalStatus.handleInProgressStatus(remoteNodeStatus)
			if err != nil {
				return err
			}
		case Done:
			err = generalStatus.handleDoneStatus(remoteNodeStatus)
			if err != nil {
				return err
			}
		}

		for _, node := range generalStatus.nodesStatus {
			if !node.isDone {
				time.Sleep(waitTimeBetweenPropertiesStatusSeconds * time.Second)
				continue propertiesHandling
			}
		}

		return nil
	}
}

type propsHandlingStatus struct {
	nodesStatus []nodeStatus
}

type nodeStatus struct {
	nodeId              string
	propertiesDelivered int64
	propertiesTotal     int64
	isDone              bool
}

func (p *propertiesDiffPhase) makePropsHandlingStatus() (propsHandlingStatus, error) {
	newStatus := propsHandlingStatus{}
	nodes, err := getRunningNodes(p.context, p.srcRtDetails)
	if err != nil {
		return newStatus, err
	}
	for i := range nodes {
		newStatus.nodesStatus = append(newStatus.nodesStatus, nodeStatus{nodeId: nodes[i]})
	}
	return newStatus, nil
}

// Looks for the nodeStatus the belongs to the nodeId. May return nil if an unknown node id is returned.
func (phs propsHandlingStatus) getNodeStatus(nodeId string) *nodeStatus {
	for i := range phs.nodesStatus {
		if nodeId == phs.nodesStatus[i].nodeId {
			return &phs.nodesStatus[i]
		}
	}
	// Unknown node id was returned.
	log.Error("unknown node id '" + nodeId + "' was returned. Skipping...")
	return nil
}

func (phs propsHandlingStatus) handleInProgressStatus(remoteNodeStatus *HandlePropertiesDiffResponse) error {
	localNodeStatus := phs.getNodeStatus(remoteNodeStatus.NodeId)
	if localNodeStatus == nil {
		return nil
	}
	return phs.updateTotalAndDelivered(localNodeStatus, remoteNodeStatus)
}

func (phs propsHandlingStatus) updateTotalAndDelivered(localNodeStatus *nodeStatus, remoteNodeStatus *HandlePropertiesDiffResponse) error {
	remoteTotal, err := remoteNodeStatus.PropertiesTotal.Int64()
	if err != nil {
		return err
	}
	// Total has changed, update it.
	if remoteTotal != localNodeStatus.propertiesTotal {
		localNodeStatus.propertiesTotal = remoteTotal
	}

	// Delivered has changed, update it.
	delivered, err := remoteNodeStatus.PropertiesDelivered.Int64()
	if err != nil {
		return err
	}
	localNodeStatus.propertiesDelivered = delivered
	return nil
}

func (phs propsHandlingStatus) handleDoneStatus(remoteNodeStatus *HandlePropertiesDiffResponse) error {
	localNodeStatus := phs.getNodeStatus(remoteNodeStatus.NodeId)
	if localNodeStatus == nil {
		return nil
	}

	// Already handled reaching done.
	if localNodeStatus.isDone {
		return nil
	}

	localNodeStatus.isDone = true
	return phs.updateTotalAndDelivered(localNodeStatus, remoteNodeStatus)
}

func isPropertiesPhaseDisabled() bool {
	return true
}
