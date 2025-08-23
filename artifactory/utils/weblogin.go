package utils

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/auth"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/pkg/browser"
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
		err = errors.Join(err,
			errorutils.CheckErrorf("Oops! it looks like the web login option is unavailable right now. This could be because your JFrog Artifactory version is less than 7.64.0. \n"+
				"Don't worry! You can use the \"jf c add\" command to authenticate with the JFrog Platform using other methods"))
		return
	}
	loginUrl := clientUtils.AddTrailingSlashIfNeeded(serverDetails.Url) + "ui/login?jfClientSession=" + uuidStr + "&jfClientName=JFrog-CLI&jfClientCode=1"

	log.Info("Please open the following URL in your browser to authenticate:")
	log.Info(coreutils.PrintBoldTitle(loginUrl))
	log.Info("")
	log.Info("After logging in via your web browser, please enter the code if prompted: " + coreutils.PrintBoldTitle(uuidStr[len(uuidStr)-4:]))

	// Attempt to open in browser with improved error handling
	if err = openBrowserWithFallback(loginUrl); err != nil {
		log.Warn("Failed to automatically open the browser: " + err.Error())
		log.Info("")
		log.Info("Please manually copy and paste the URL above into your browser.")
		log.Info("If you're using WSL2, you can set the JFROG_CLI_BROWSER_COMMAND environment variable")
		log.Info("to specify a custom browser command, for example:")
		log.Info("  export JFROG_CLI_BROWSER_COMMAND=\"/mnt/c/Program\\ Files/Google/Chrome/Application/chrome.exe\"")
		log.Info("  or")
		log.Info("  export BROWSER=\"wslview\"  # if you have wslu installed")
		log.Info("")
		// Do not return, continue the flow
	} else {
		log.Debug("Browser opened successfully")
	}

	// Give a bit more time for browser to open and user to see the instructions
	time.Sleep(2 * time.Second)

	log.Info("Waiting for authentication to complete...")
	log.Info("Please complete the login process in your browser.")
	log.Debug("Attempting to get the authentication token...")

	// The GetLoginAuthenticationToken method handles its own polling and timeout logic
	token, err = accessManager.GetLoginAuthenticationToken(uuidStr)
	if err != nil {
		// Provide helpful error message for common timeout scenarios
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "context deadline exceeded") {
			log.Error("Authentication timed out. This may happen if:")
			log.Error("  1. The browser failed to open (check the URL above)")
			log.Error("  2. You didn't complete the login process in time")
			log.Error("  3. Network connectivity issues")
			log.Error("")
			log.Error("Please try again. If the issue persists in WSL2, consider setting:")
			log.Error("  export JFROG_CLI_BROWSER_COMMAND=\"/mnt/c/Program\\ Files/Google/Chrome/Application/chrome.exe\"")
		}
		return
	}
	if token.AccessToken == "" {
		return token, errorutils.CheckErrorf("failed getting authentication token after web login")
	}
	log.Info("You're now logged in!")
	return
}

// openBrowserWithFallback attempts to open a URL in a browser with improved error handling
// and support for custom browser commands via environment variables.
// This is particularly useful for WSL2 environments where standard browser opening may fail.
func openBrowserWithFallback(url string) error {
	// Check if user has specified a custom browser command
	if customBrowserCmd := os.Getenv(coreutils.BrowserCommand); customBrowserCmd != "" {
		log.Debug("Using custom browser command from environment variable:", customBrowserCmd)
		return runCustomBrowserCommand(customBrowserCmd, url)
	}

	// Check if BROWSER environment variable is set (common convention)
	if browserEnv := os.Getenv("BROWSER"); browserEnv != "" {
		log.Debug("Using browser from BROWSER environment variable:", browserEnv)
		if browserEnv == "none" || browserEnv == "" {
			// User explicitly disabled browser opening
			return errors.New("browser opening disabled via BROWSER environment variable")
		}
		return runCustomBrowserCommand(browserEnv, url)
	}

	// Fall back to pkg/browser default behavior
	return browser.OpenURL(url)
}

// runCustomBrowserCommand executes a custom browser command with the given URL
func runCustomBrowserCommand(browserCmd, url string) error {
	// Split the command to handle arguments
	parts := strings.Fields(browserCmd)
	if len(parts) == 0 {
		return errors.New("empty browser command")
	}

	cmd := exec.Command(parts[0], append(parts[1:], url)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func sendUnauthenticatedPing(serverDetails *config.ServerDetails) error {
	artifactoryManager, err := CreateServiceManager(serverDetails, 3, 0, false)
	if err != nil {
		return err
	}
	_, err = artifactoryManager.Ping()
	return err
}
