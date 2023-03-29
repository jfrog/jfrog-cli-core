package config

import (
	"encoding/base64"
	"encoding/json"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

const tokenVersion = 2

type configToken struct {
	Version              int    `json:"version,omitempty"`
	Url                  string `json:"url,omitempty"`
	ArtifactoryUrl       string `json:"artifactoryUrl,omitempty"`
	DistributionUrl      string `json:"distributionUrl,omitempty"`
	XrayUrl              string `json:"xrayUrl,omitempty"`
	MissionControlUrl    string `json:"missionControlUrl,omitempty"`
	PipelinesUrl         string `json:"pipelinesUrl,omitempty"`
	User                 string `json:"user,omitempty"`
	Password             string `json:"password,omitempty"`
	SshKeyPath           string `json:"sshKeyPath,omitempty"`
	SshPassphrase        string `json:"sshPassphrase,omitempty"`
	AccessToken          string `json:"accessToken,omitempty"`
	RefreshToken         string `json:"refreshToken,omitempty"`
	TokenRefreshInterval int    `json:"tokenRefreshInterval,omitempty"`
	ClientCertPath       string `json:"clientCertPath,omitempty"`
	ClientCertKeyPath    string `json:"clientCertKeyPath,omitempty"`
	ServerId             string `json:"serverId,omitempty"`
}

func (token *configToken) convertToV2() {
	if token.ArtifactoryUrl == "" {
		token.ArtifactoryUrl = token.Url
		token.Url = ""
	}
}

func fromServerDetails(details *ServerDetails) *configToken {
	return &configToken{
		Version:              tokenVersion,
		Url:                  details.Url,
		ArtifactoryUrl:       details.ArtifactoryUrl,
		DistributionUrl:      details.DistributionUrl,
		XrayUrl:              details.XrayUrl,
		MissionControlUrl:    details.MissionControlUrl,
		PipelinesUrl:         details.PipelinesUrl,
		User:                 details.User,
		Password:             details.Password,
		SshKeyPath:           details.SshKeyPath,
		SshPassphrase:        details.SshPassphrase,
		AccessToken:          details.AccessToken,
		RefreshToken:         details.ArtifactoryRefreshToken,
		TokenRefreshInterval: details.ArtifactoryTokenRefreshInterval,
		ClientCertPath:       details.ClientCertPath,
		ClientCertKeyPath:    details.ClientCertKeyPath,
		ServerId:             details.ServerId,
	}
}

func toServerDetails(detailsSerialization *configToken) *ServerDetails {
	return &ServerDetails{
		Url:                             detailsSerialization.Url,
		ArtifactoryUrl:                  detailsSerialization.ArtifactoryUrl,
		DistributionUrl:                 detailsSerialization.DistributionUrl,
		MissionControlUrl:               detailsSerialization.MissionControlUrl,
		PipelinesUrl:                    detailsSerialization.PipelinesUrl,
		XrayUrl:                         detailsSerialization.XrayUrl,
		User:                            detailsSerialization.User,
		Password:                        detailsSerialization.Password,
		SshKeyPath:                      detailsSerialization.SshKeyPath,
		SshPassphrase:                   detailsSerialization.SshPassphrase,
		AccessToken:                     detailsSerialization.AccessToken,
		ArtifactoryRefreshToken:         detailsSerialization.RefreshToken,
		ArtifactoryTokenRefreshInterval: detailsSerialization.TokenRefreshInterval,
		ClientCertPath:                  detailsSerialization.ClientCertPath,
		ClientCertKeyPath:               detailsSerialization.ClientCertKeyPath,
		ServerId:                        detailsSerialization.ServerId,
	}
}

func Export(details *ServerDetails) (string, error) {
	conf, err := readConf()
	if err != nil {
		return "", err
	}
	// If config is encrypted, ask for master key.
	if conf.Enc {
		masterKeyFromFile, err := getEncryptionKey()
		if err != nil {
			return "", err
		}
		masterKeyFromConsole, err := readMasterKeyFromConsole()
		if err != nil {
			return "", err
		}
		if masterKeyFromConsole != masterKeyFromFile {
			return "", errorutils.CheckErrorf("could not generate config token: config is encrypted, and wrong master key was provided")
		}
	}
	buffer, err := json.Marshal(fromServerDetails(details))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buffer), nil
}

func Import(configTokenString string) (*ServerDetails, error) {
	decoded, err := base64.StdEncoding.DecodeString(configTokenString)
	if err != nil {
		return nil, err
	}
	token := &configToken{}
	if err = json.Unmarshal(decoded, token); err != nil {
		return nil, err
	}
	if token.Version < tokenVersion {
		token.convertToV2()
	}
	return toServerDetails(token), nil
}
