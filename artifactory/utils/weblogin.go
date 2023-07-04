package utils

import (
	"github.com/google/uuid"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/auth"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/pkg/browser"
	"time"
)

func DoWebLogin(serverDetails *config.ServerDetails) (token auth.CommonTokenParams, err error) {
	if err = sendUnauthenticatedPing(serverDetails); err != nil {
		return
	}

	uuidToken, err := uuid.NewRandom()
	if errorutils.CheckError(err) != nil {
		return
	}
	uuidStr := uuidToken.String()
	accessManager, err := CreateAccessServiceManager(serverDetails, false)
	if err != nil {
		return
	}
	if err = accessManager.SendLoginAuthenticationRequest(uuidStr); err != nil {
		log.Info("web login is only supported for Artifactory version 7.63.1 and above. " +
			"Make sure the details you entered are correct and that Artifactory stands in the version requirement.")
		return
	}
	log.Info("Please log in to the JFrog platform using the opened browser.")
	err = browser.OpenURL(clientUtils.AddTrailingSlashIfNeeded(serverDetails.Url) + "ui/login?jfClientSession=" + uuidStr + "&jfClientName=JFrogCLI")
	if err != nil {
		return
	}
	time.Sleep(1 * time.Second)
	log.Info("Attempting to get the authentication token...")
	token, err = accessManager.GetLoginAuthenticationToken(uuidStr)
	if err != nil {
		return
	}
	if token.AccessToken == "" {
		return token, errorutils.CheckErrorf("failed getting authentication token after web log")
	}
	log.Info("Received token from platform!")
	return
}

func sendUnauthenticatedPing(serverDetails *config.ServerDetails) error {
	artifactoryManager, err := CreateServiceManager(serverDetails, 3, 0, false)
	if err != nil {
		return err
	}
	_, err = artifactoryManager.Ping()
	return err
}
