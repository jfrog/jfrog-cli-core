package usage

import (
	"fmt"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	xrayutils "github.com/jfrog/jfrog-cli-core/v2/utils/xray"
	"github.com/jfrog/jfrog-client-go/artifactory/usage"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	xrayusage "github.com/jfrog/jfrog-client-go/xray/usage"

	"golang.org/x/sync/errgroup"
)

const (
	ArtifactoryCallHomePrefix = "Artifactory Call Home:"
	clientIdAttributeName     = "clientId"
)

type UsageReporter struct {
	ProductId       string
	serverDetails   *config.ServerDetails
	reportWaitGroup *errgroup.Group

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
		sendToXray:        true,
		sendToArtifactory: true,
	}
}

func ShouldReportUsage() (reportUsage bool) {
	reportUsage, err := clientutils.GetBoolEnvValue(coreutils.ReportUsage, true)
	if err != nil {
		log.Debug(ArtifactoryCallHomePrefix + err.Error())
		return false
	}
	return reportUsage
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
	if len(features) == 0 {
		log.Debug(ArtifactoryCallHomePrefix, "Nothing to send.")
		return
	}
	log.Debug(ArtifactoryCallHomePrefix, "Sending info...")
	if ur.sendToXray {
		ur.reportWaitGroup.Go(func() (err error) {
			if err = ur.reportToXray(features...); err != nil {
				err = fmt.Errorf("xray, %w", err)
			}
			return
		})
	}
	if ur.sendToArtifactory {
		ur.reportWaitGroup.Go(func() (err error) {
			if err = ur.reportToArtifactory(features...); err != nil {
				err = fmt.Errorf("artifactory, %w", err)
			}
			return
		})
	}
}

func (ur *UsageReporter) WaitForResponses() (err error) {
	if err = ur.reportWaitGroup.Wait(); err != nil {
		err = fmt.Errorf("%s %s", ArtifactoryCallHomePrefix, err.Error())
	}
	return
}

func (ur *UsageReporter) reportToXray(features ...ReportFeature) (err error) {
	events := ur.convertAttributesToXrayEvents(features...)
	if len(events) == 0 {
		err = errorutils.CheckErrorf("Nothing to send.")
		return
	}
	if ur.serverDetails.XrayUrl == "" {
		err = errorutils.CheckErrorf("Xray Url is not set.")
		return
	}
	serviceManager, err := xrayutils.CreateXrayServiceManager(ur.serverDetails)
	if err != nil {
		return
	}
	return xrayusage.SendXrayUsageEvents(*serviceManager, events...)
}

func (ur *UsageReporter) reportToArtifactory(features ...ReportFeature) (err error) {
	converted := ur.convertAttributesToArtifactoryFeatures(features...)
	if len(converted) == 0 {
		err = errorutils.CheckErrorf("nothing to send")
		return
	}
	if ur.serverDetails.ArtifactoryUrl == "" {
		err = errorutils.CheckErrorf("Artifactory URL is not set")
		return
	}
	serviceManager, err := utils.CreateServiceManager(ur.serverDetails, -1, 0, false)
	if err != nil {
		return
	}
	return usage.NewArtifactoryCallHome().SendToArtifactory(ur.ProductId, serviceManager, converted...)
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
