package usage

import (
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	xray "github.com/jfrog/jfrog-cli-core/v2/xray/commands/utils"
	"github.com/jfrog/jfrog-client-go/artifactory/usage"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	ecosysusage "github.com/jfrog/jfrog-client-go/utils/usage"
	xrayusage "github.com/jfrog/jfrog-client-go/xray/usage"
)

const (
	ReportUsagePrefix     = "Usage Report:"
	clientIdAttributeName = "clientId"
)

type UsageReporter struct {
	ProductId       string
	serverDetails   *config.ServerDetails
	reportWaitGroup *errgroup.Group

	sendToEcosystem   bool
	sendToXray        bool
	sendToArtifactory bool
}

type ReportFeature struct {
	FeatureId  string
	ClientId   string
	Attributes []ReportUsageAttribute
}

type ReportUsageAttribute struct {
	AttributeName  string
	AttributeValue string
}

func NewUsageReporter(productId string, serverDetails *config.ServerDetails) *UsageReporter {
	return &UsageReporter{
		ProductId:         productId,
		serverDetails:     serverDetails,
		reportWaitGroup:   new(errgroup.Group),
		sendToEcosystem:   true,
		sendToXray:        true,
		sendToArtifactory: true,
	}
}

func ShouldReportUsage() (reportUsage bool) {
	reportUsage, err := clientutils.GetBoolEnvValue(coreutils.ReportUsage, true)
	if err != nil {
		log.Debug(ReportUsagePrefix + err.Error())
		return false
	}
	return reportUsage
}

func (ur *UsageReporter) SetSendToEcosystem(send bool) *UsageReporter {
	ur.sendToEcosystem = send
	return ur
}

func (ur *UsageReporter) SetSendToXray(send bool) *UsageReporter {
	ur.sendToXray = send
	return ur
}

func (ur *UsageReporter) SetSendToArtifactory(send bool) *UsageReporter {
	ur.sendToArtifactory = send
	return ur
}

// Report usage to Artifactory, Xray and Ecosystem
func (ur *UsageReporter) Report(features ...ReportFeature) {
	if !ShouldReportUsage() {
		log.Debug("Usage info is disabled.")
		return
	}
	log.Debug(ReportUsagePrefix, "Sending info...")
	if ur.sendToEcosystem {
		ur.reportWaitGroup.Go(func() (err error) {
			if err = errors.Join(err, ur.reportToEcosystem(features...)); err != nil {
				err = fmt.Errorf("ecosystem, %s", err.Error())
			}
			return
		})
	}
	if ur.sendToXray {
		ur.reportWaitGroup.Go(func() (err error) {
			if err = errors.Join(err, ur.reportToXray(features...)); err != nil {
				err = fmt.Errorf("xray, %s", err.Error())
			}
			return
		})
	}
	if ur.sendToArtifactory {
		ur.reportWaitGroup.Go(func() (err error) {
			if err = errors.Join(err, ur.reportToArtifactory(features...)); err != nil {
				err = fmt.Errorf("artifactory, %s", err.Error())
			}
			return
		})
	}
}

func (ur *UsageReporter) WaitForResponses() (err error) {
	if err = ur.reportWaitGroup.Wait(); err != nil {
		err = fmt.Errorf("%s %s", ReportUsagePrefix, err.Error())
	}
	return
}

func (ur *UsageReporter) reportToEcosystem(features ...ReportFeature) (err error) {
	if ur.serverDetails.Url == "" {
		err = errorutils.CheckErrorf("platform Url is not set")
		return
	}
	reports, err := ur.convertAttributesToEcosystemReports(features...)
	if len(reports) == 0 || err != nil {
		err = errorutils.CheckErrorf("Nothing to send.")
		return
	}
	return ecosysusage.SendEcosystemUsageReports(reports...)
}

func (ur *UsageReporter) reportToXray(features ...ReportFeature) (err error) {
	if ur.serverDetails.XrayUrl == "" {
		err = errorutils.CheckErrorf("Xray Url is not set.")
		return
	}
	serviceManager, err := xray.CreateXrayServiceManager(ur.serverDetails)
	if err != nil {
		return
	}
	events := ur.convertAttributesToXrayEvents(features...)
	if len(events) == 0 {
		err = errorutils.CheckErrorf("Nothing to send.")
		return
	}
	return xrayusage.SendXrayUsageEvents(*serviceManager, events...)
}

func (ur *UsageReporter) reportToArtifactory(features ...ReportFeature) (err error) {
	if ur.serverDetails.ArtifactoryUrl == "" {
		err = errorutils.CheckErrorf("Artifactory Url is not set..")
		return
	}
	serviceManager, err := utils.CreateServiceManager(ur.serverDetails, -1, 0, false)
	if err != nil {
		return
	}
	converted := ur.convertAttributesToArtifactoryFeatures(features...)
	if len(converted) == 0 {
		err = errorutils.CheckErrorf("Nothing to send.")
		return
	}
	return usage.ReportUsageToArtifactory(ur.ProductId, serviceManager, converted...)
}

func convertAttributesToMap(reportFeature ReportFeature) (converted map[string]string) {
	if len(reportFeature.Attributes) == 0 {
		return
	}
	converted = make(map[string]string, len(reportFeature.Attributes))
	for _, attribute := range reportFeature.Attributes {
		if attribute.AttributeName != "" {
			converted[attribute.AttributeName] = attribute.AttributeValue
		}
	}
	return
}

func (ur *UsageReporter) convertAttributesToArtifactoryFeatures(reportFeatures ...ReportFeature) (features []usage.Feature) {
	for _, feature := range reportFeatures {
		featureInfo := usage.Feature{
			FeatureId:  feature.FeatureId,
			ClientId:   feature.ClientId,
			Attributes: convertAttributesToMap(feature),
		}
		features = append(features, featureInfo)
	}
	return
}

func (ur *UsageReporter) convertAttributesToXrayEvents(reportFeatures ...ReportFeature) (events []xrayusage.ReportXrayEventData) {
	for _, feature := range reportFeatures {
		convertedAttributes := []xrayusage.ReportUsageAttribute{}
		for _, attribute := range feature.Attributes {
			convertedAttributes = append(convertedAttributes, xrayusage.ReportUsageAttribute{
				AttributeName:  attribute.AttributeName,
				AttributeValue: attribute.AttributeValue,
			})
		}
		if feature.ClientId != "" {
			// Add clientId as attribute
			convertedAttributes = append(convertedAttributes, xrayusage.ReportUsageAttribute{
				AttributeName:  clientIdAttributeName,
				AttributeValue: feature.ClientId,
			})
		}
		events = append(events, xrayusage.CreateUsageEvent(
			ur.ProductId, feature.FeatureId, convertedAttributes...,
		))
	}
	return
}

func (ur *UsageReporter) convertAttributesToEcosystemReports(reportFeatures ...ReportFeature) (reports []ecosysusage.ReportEcosystemUsageData, err error) {
	accountId := ur.serverDetails.Url
	clientToFeaturesMap := map[string][]string{}
	// Combine
	for _, feature := range reportFeatures {
		if feature.FeatureId == "" {
			continue
		}
		if features, contains := clientToFeaturesMap[feature.ClientId]; contains {
			clientToFeaturesMap[feature.ClientId] = append(features, feature.FeatureId)
		} else {
			clientToFeaturesMap[feature.ClientId] = []string{feature.FeatureId}
		}
	}
	// Create data
	for clientId, features := range clientToFeaturesMap {
		var report ecosysusage.ReportEcosystemUsageData
		if report, err = ecosysusage.CreateUsageData(ur.ProductId, accountId, clientId, features...); err != nil {
			return
		}
		reports = append(reports, report)
	}
	return
}
