package transferfiles

import (
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"time"
)

const waitTimeBetweenPropertiesStatusSeconds = 5
const propertiesPhaseDisabled = true

type propertiesDiffPhase struct {
	repoKey                   string
	checkExistenceInFilestore bool
	startTime                 time.Time
	srcUpService              *srcUserPluginService
	srcRtDetails              *coreConfig.ServerDetails
	targetRtDetails           *coreConfig.ServerDetails
	progressBar               *progressbar.TransferProgressMng
}

func (p *propertiesDiffPhase) getSourceDetails() *coreConfig.ServerDetails {
	return p.srcRtDetails
}

func (p *propertiesDiffPhase) setProgressBar(progressbar *progressbar.TransferProgressMng) {
	p.progressBar = progressbar
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
	return setPropsDiffHandlingCompleted(p.repoKey)
}

func (p *propertiesDiffPhase) shouldCheckExistenceInFilestore(shouldCheck bool) {
	p.checkExistenceInFilestore = shouldCheck
}

func (p *propertiesDiffPhase) shouldSkipPhase() (bool, error) {
	return propertiesPhaseDisabled, nil
}

func (p *propertiesDiffPhase) setSrcUserPluginService(service *srcUserPluginService) {
	p.srcUpService = service
}

func (p *propertiesDiffPhase) setSourceDetails(details *coreConfig.ServerDetails) {
	p.srcRtDetails = details
}

func (p *propertiesDiffPhase) setTargetDetails(details *coreConfig.ServerDetails) {
	p.targetRtDetails = details
}

func (p *propertiesDiffPhase) run() error {
	diffStart, diffEnd, err := getDiffHandlingRange(p.repoKey)
	if err != nil {
		return err
	}

	requestBody := HandlePropertiesDiff{
		TargetAuth:        createTargetAuth(p.targetRtDetails),
		RepoKey:           p.repoKey,
		StartMilliseconds: convertTimeToEpochMilliseconds(diffStart),
		EndMilliseconds:   convertTimeToEpochMilliseconds(diffEnd),
	}

	generalStatus, err := makePropsHandlingStatus()
	if err != nil {
		return err
	}

	// Periodically send handling requests to the user plugin to handle properties diff in a time range.
	// Update progress with the status return from those requests.
	// Done handling when all nodes return done status.
propertiesHandling:
	for {
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

		notifyPropertiesProgressDone()
		return nil
	}
}

type propsHandlingStatus struct {
	nodesStatus         []nodeStatus
	totalPropsToDeliver int64
	totalPropsDelivered int64
}
type nodeStatus struct {
	nodeId              string
	propertiesDelivered int64
	propertiesTotal     int64
	isDone              bool
}

func makePropsHandlingStatus() (propsHandlingStatus, error) {
	newStatus := propsHandlingStatus{}
	nodes, err := getNodesList()
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
	log.Error("Unknown node id '" + nodeId + "' was returned. Skipping...")
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
		// TODO handle error.
		return err
	}
	// Total has changed, update it.
	if remoteTotal != localNodeStatus.propertiesTotal {
		phs.totalPropsToDeliver += remoteTotal - localNodeStatus.propertiesTotal
		localNodeStatus.propertiesTotal = remoteTotal
		updatePropertiesProgressTotal(phs.totalPropsToDeliver)
	}

	// Delivered has changed, update it.
	delivered, err := remoteNodeStatus.PropertiesDelivered.Int64()
	if err != nil {
		// TODO handle error.
		return err
	}
	newDeliveries := delivered - localNodeStatus.propertiesDelivered
	incrementPropertiesProgress(newDeliveries)
	phs.totalPropsDelivered += newDeliveries
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
	addErrorsToNonConsumableFile(remoteNodeStatus.Errors)
	return phs.updateTotalAndDelivered(localNodeStatus, remoteNodeStatus)
}

func updatePropertiesProgressTotal(newTotal int64) {
	// TODO implement
}

func incrementPropertiesProgress(incr int64) {
	// TODO implement
}

func notifyPropertiesProgressDone() {
	// TODO implement
}

func addErrorsToNonConsumableFile(errors []PropertiesHandlingError) {
	// TODO implement
}
