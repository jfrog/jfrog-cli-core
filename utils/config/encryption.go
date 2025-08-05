package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"os"
	"strconv"
	"syscall"

	ioutils "github.com/jfrog/gofrog/io"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

type SecurityConf struct {
	Version   string `yaml:"version,omitempty"`
	MasterKey string `yaml:"masterKey,omitempty"`
}

const masterKeyField = "masterKey"
const masterKeyLength = 32
const encryptErrorPrefix = "cannot encrypt config: "
const decryptErrorPrefix = "cannot decrypt config: "

type secretHandler func(string, string) (string, error)

// Encrypt config file if security configuration file exists and contains master key.
func (config *Config) encrypt() error {
	key, err := getEncryptionKey()
	if err != nil || key == "" {
		return err
	}
	// Mark config as encrypted.
	config.Enc = true
	return handleSecrets(config, encrypt, key)
}

// Decrypt config if encrypted and master key exists.
func (config *Config) decrypt() error {
	if !config.Enc {
		return updateEncryptionIfNeeded(config)
	}
	key, err := getEncryptionKey()
	if err != nil {
		return err
	}
	if key == "" {
		return errorutils.CheckErrorf(decryptErrorPrefix+"security configuration file was not found or the '%s' environment variable was not configured", coreutils.EncryptionKey)
	}
	config.Enc = false
	return handleSecrets(config, decrypt, key)
}

// Encrypt the config file if it is decrypted while security configuration file exists and contains a master key, or if the JFROG_CLI_ENCRYPTION_KEY environment variable exist.
func updateEncryptionIfNeeded(config *Config) error {
	masterKey, err := getEncryptionKey()
	if err != nil || masterKey == "" {
		return err
	}
	// The encryption key exists and will be loaded again in encrypt()
	return saveConfig(config)
}

// Encrypt/Decrypt all secrets in the provided config, with the provided master key.
func handleSecrets(config *Config, handler secretHandler, key string) error {
	var err error
	for _, serverDetails := range config.Servers {
		serverDetails.Password, err = handler(serverDetails.Password, key)
		if err != nil {
			return err
		}
		serverDetails.AccessToken, err = handler(serverDetails.AccessToken, key)
		if err != nil {
			return err
		}
		serverDetails.SshPassphrase, err = handler(serverDetails.SshPassphrase, key)
		if err != nil {
			return err
		}
		serverDetails.RefreshToken, err = handler(serverDetails.RefreshToken, key)
		if err != nil {
			return err
		}
		serverDetails.ArtifactoryRefreshToken, err = handler(serverDetails.ArtifactoryRefreshToken, key)
		if err != nil {
			return err
		}
	}
	return nil
}

func getEncryptionKey() (string, error) {
	if key, exist := os.LookupEnv(coreutils.EncryptionKey); exist {
		return key, nil
	}
	return getEncryptionKeyFromSecurityConfFile()
}

func getEncryptionKeyFromSecurityConfFile() (key string, err error) {
	secFile, err := coreutils.GetJfrogSecurityConfFilePath()
	if err != nil {
		return "", err
	}
	exists, err := fileutils.IsFileExists(secFile, false)
	if err != nil || !exists {
		return "", err
	}

	config := viper.New()
	config.SetConfigType("yaml")
	f, err := os.Open(secFile)
	defer ioutils.Close(f, &err)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	err = config.ReadConfig(f)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	key = config.GetString(masterKeyField)
	if key == "" {
		return "", errorutils.CheckErrorf(decryptErrorPrefix + "security configuration file does not contain an encryption master key")
	}
	return key, nil
}

func readMasterKeyFromConsole() (string, error) {
	log.Output("Please enter the master key: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin)) //nolint:unconvert
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	// New-line required after the input:
	log.Output()
	return string(bytePassword), nil
}

func encrypt(secret string, key string) (string, error) {
	if secret == "" {
		return "", nil
	}
	if len(key) != 32 {
		return "", errorutils.CheckErrorf("%s Wrong length for master key. Key should have a length of exactly: %s bytes", encryptErrorPrefix, strconv.Itoa(masterKeyLength))
	}
	c, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", errorutils.CheckError(err)
	}

	sealed := gcm.Seal(nonce, nonce, []byte(secret), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func decrypt(encryptedSecret string, key string) (string, error) {
	if encryptedSecret == "" {
		return "", nil
	}

	cipherText, err := base64.StdEncoding.DecodeString(encryptedSecret)
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	c, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	nonceSize := gcm.NonceSize()
	if len(cipherText) < nonceSize {
		return "", errorutils.CheckErrorf(decryptErrorPrefix + "unexpected cipher text size")
	}

	//#nosec G407
	nonce, cipherText := cipherText[:nonceSize], cipherText[nonceSize:]
	//#nosec G407
	plaintext, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return string(plaintext), nil
}
