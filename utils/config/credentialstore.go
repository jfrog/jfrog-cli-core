package config

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/lock"
	"github.com/zalando/go-keyring"
)

const (
	// CredentialStoreFile stores secrets in the configuration, using existing encryption settings.
	CredentialStoreFile = "file"
	// CredentialStoreKeychain stores secrets in the current user's OS credential store.
	CredentialStoreKeychain = "keychain"
	// Base64 chunks fit Windows' 2560-byte blobs and macOS security's command limit.
	credentialChunkSize = 2000
	maxCredentialChunks = 1024
)

type credentialKeyring interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

type systemKeyring struct{}

func (systemKeyring) Get(service, user string) (string, error) { return keyring.Get(service, user) }
func (systemKeyring) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}
func (systemKeyring) Delete(service, user string) error { return keyring.Delete(service, user) }

// Dependencies belong to an instance, so tests do not replace mutable globals.
// The writer reports whether replacement occurred even if durability confirmation fails.
type credentialStore struct {
	keyring     credentialKeyring
	writeConfig func(string, []byte) (replaced bool, err error)
}

func newCredentialStore() *credentialStore {
	return &credentialStore{keyring: systemKeyring{}, writeConfig: atomicWriteConfig}
}

func withConfigLock(action func() error) (err error) {
	dir, err := coreutils.GetJfrogConfigLockDir()
	if err != nil {
		return err
	}
	unlock, err := lock.CreateLock(dir)
	// CreateLock can create a file before failing to acquire the lock.
	defer func() { err = errors.Join(err, unlock()) }()
	if err != nil {
		return err
	}
	return action()
}

// SetCredentialStore migrates all credentials and persists the storage choice.
// Callers must not already hold the configuration lock.
func SetCredentialStore(store string) error {
	return newCredentialStore().setCredentialStore(store)
}

func (s *credentialStore) setCredentialStore(store string) error {
	if err := validateCredentialStore(store); err != nil {
		return err
	}
	if store == "" {
		return errors.New("credential store must be file or keychain")
	}
	return withConfigLock(func() error {
		conf, err := s.readConfLocked()
		if err != nil {
			return err
		}
		if store == CredentialStoreKeychain {
			if err = s.probe(); err != nil {
				return err
			}
		}
		conf.CredentialStore = store
		conf.Version = strconv.Itoa(coreutils.GetCliConfigVersion())
		return s.saveConfig(conf)
	})
}

func (s *credentialStore) probe() error {
	service, err := credentialService()
	if err != nil {
		return err
	}
	ref := "probe-" + uuid.NewString()
	if err = s.keyring.Set(service, ref, ref); err != nil {
		return keychainError("access", err)
	}
	value, readErr := s.keyring.Get(service, ref)
	deleteErr := s.keyring.Delete(service, ref)
	if readErr != nil || deleteErr != nil {
		return keychainError("verify", errors.Join(readErr, deleteErr))
	}
	if value != ref {
		return errors.New("keychain verification failed")
	}
	return nil
}

func (s *credentialStore) readConf() (conf *Config, err error) {
	err = withConfigLock(func() error {
		conf, err = s.readConfLocked()
		return err
	})
	return conf, err
}

// The lock spans metadata reads, secret reads, automatic encryption and writes.
// A reader can never observe references while another process deletes them.
func (s *credentialStore) readConfLocked() (*Config, error) {
	conf, err := readConfMetadataLocked()
	if err != nil {
		return nil, err
	}
	if err = s.loadCredentials(conf); err != nil {
		return nil, err
	}
	if conf.needsEncryption && conf.CredentialStore != CredentialStoreKeychain {
		key, keyErr := getEncryptionKey()
		if keyErr != nil {
			return nil, keyErr
		}
		if key != "" {
			if err = s.saveConfig(conf); err != nil {
				return nil, err
			}
		}
	}
	return conf, nil
}

func (s *credentialStore) saveServersConf(details []*ServerDetails) error {
	return withConfigLock(func() error { return s.saveServersConfLocked(details) })
}

func (s *credentialStore) saveServersConfLocked(details []*ServerDetails) error {
	conf, err := s.readConfLocked()
	if err != nil {
		return err
	}
	conf.Servers = details
	conf.Version = strconv.Itoa(coreutils.GetCliConfigVersion())
	return s.saveConfig(conf)
}

// saveConfig requires the configuration lock. Always clone before stripping secrets.
func (s *credentialStore) saveConfig(conf *Config) error {
	clone, err := conf.Clone()
	if err != nil {
		return err
	}
	return s.persistCredentials(clone)
}

func validateCredentialStore(store string) error {
	switch store {
	case "", CredentialStoreFile, CredentialStoreKeychain:
		return nil
	default:
		return fmt.Errorf("unsupported credential store %q (expected file or keychain)", store)
	}
}

func credentialService() (string, error) {
	path, err := getConfFilePath()
	if err != nil {
		return "", err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("jfrog-cli:%x", sha256.Sum256([]byte(path))), nil
}

func keychainError(operation string, err error) error {
	return fmt.Errorf("cannot %s OS keychain credentials (ensure the credential store is available and unlocked; no file fallback): %w", operation, err)
}

type serverSecrets struct {
	Password                string `json:"password,omitempty"`
	AccessToken             string `json:"accessToken,omitempty"`
	RefreshToken            string `json:"refreshToken,omitempty"`
	ArtifactoryRefreshToken string `json:"artifactoryRefreshToken,omitempty"`
	SshPassphrase           string `json:"sshPassphrase,omitempty"`
}

func secretsOf(s *ServerDetails) serverSecrets {
	return serverSecrets{
		Password:                s.Password,
		AccessToken:             s.AccessToken,
		RefreshToken:            s.RefreshToken,
		ArtifactoryRefreshToken: s.ArtifactoryRefreshToken,
		SshPassphrase:           s.SshPassphrase,
	}
}

func (secrets serverSecrets) apply(s *ServerDetails) {
	s.Password = secrets.Password
	s.AccessToken = secrets.AccessToken
	s.RefreshToken = secrets.RefreshToken
	s.ArtifactoryRefreshToken = secrets.ArtifactoryRefreshToken
	s.SshPassphrase = secrets.SshPassphrase
}

func (s *credentialStore) loadCredentials(conf *Config) error {
	if err := validateCredentialStore(conf.CredentialStore); err != nil {
		return err
	}
	service, err := credentialService()
	if err != nil {
		return err
	}
	for _, server := range conf.Servers {
		if server == nil {
			return errors.New("invalid null server configuration")
		}
		if conf.CredentialStore != CredentialStoreKeychain {
			if server.CredentialRef != "" || server.CredentialChunks != 0 {
				return errors.New("keychain credential references found without keychain storage enabled")
			}
			continue
		}
		if server.CredentialRef == "" || server.CredentialChunks < 1 || server.CredentialChunks > maxCredentialChunks {
			return fmt.Errorf("invalid keychain credential reference for server %q", server.ServerId)
		}
		if secretsOf(server) != (serverSecrets{}) {
			return fmt.Errorf("unexpected file credentials in keychain configuration for server %q", server.ServerId)
		}
		var encoded strings.Builder
		encoded.Grow(server.CredentialChunks * credentialChunkSize)
		for i := 0; i < server.CredentialChunks; i++ {
			chunk, getErr := s.keyring.Get(service, chunkName(server.CredentialRef, i))
			if getErr != nil {
				return keychainError("read", getErr)
			}
			if len(chunk) == 0 || len(chunk) > credentialChunkSize || (i < server.CredentialChunks-1 && len(chunk) != credentialChunkSize) {
				return errors.New("invalid keychain credential chunk size")
			}
			encoded.WriteString(chunk)
		}
		data, err := base64.StdEncoding.DecodeString(encoded.String())
		if err != nil {
			return errors.New("invalid keychain credential encoding")
		}
		var secrets serverSecrets
		if err = json.Unmarshal(data, &secrets); err != nil {
			// Decoder errors can quote secret material.
			return errors.New("invalid keychain credential data")
		}
		secrets.apply(server)
	}
	return nil
}

func chunkName(ref string, i int) string { return ref + "/" + strconv.Itoa(i) }

// persistCredentials requires the lock and uses immutable snapshots. Unchanged
// credentials retain their references; only superseded snapshots are deleted.
func (s *credentialStore) persistCredentials(conf *Config) (err error) {
	if err = validateCredentialStore(conf.CredentialStore); err != nil {
		return err
	}
	oldContent, err := getConfigFile()
	if err != nil {
		return err
	}
	var previous Config
	if len(oldContent) > 0 {
		if err = json.Unmarshal(oldContent, &previous); err != nil {
			return err
		}
	}
	service, err := credentialService()
	if err != nil {
		return err
	}
	byID := make(map[string]*ServerDetails)
	if previous.CredentialStore == CredentialStoreKeychain && conf.CredentialStore == CredentialStoreKeychain {
		if err = s.loadCredentials(&previous); err != nil {
			return err
		}
		for _, server := range previous.Servers {
			byID[server.ServerId] = server
		}
	}
	retained := make(map[string]bool)
	var created []*ServerDetails
	replaced := false
	defer func() {
		// A post-replacement sync failure must retain BOTH generations.
		if !replaced {
			err = errors.Join(err, s.deleteCredentialRefs(service, created))
		}
	}()
	if conf.CredentialStore == CredentialStoreKeychain {
		conf.Enc = false
		for _, server := range conf.Servers {
			if server == nil {
				return errors.New("invalid null server configuration")
			}
			if old := byID[server.ServerId]; old != nil && secretsOf(old) == secretsOf(server) {
				server.CredentialRef, server.CredentialChunks = old.CredentialRef, old.CredentialChunks
				retained[old.CredentialRef] = true
			} else {
				if err = s.storeCredentials(service, server, &created); err != nil {
					return err
				}
			}
			(serverSecrets{}).apply(server)
		}
	} else {
		for _, server := range conf.Servers {
			if server == nil {
				return errors.New("invalid null server configuration")
			}
			server.CredentialRef, server.CredentialChunks = "", 0
		}
		if err = conf.encrypt(); err != nil {
			return err
		}
	}
	content, err := conf.getContent()
	if err != nil {
		return err
	}
	path, err := getConfFilePath()
	if err != nil {
		return err
	}
	replaced, err = s.writeConfig(path, content)
	if err != nil {
		if replaced {
			return fmt.Errorf("configuration replaced but durability not confirmed; both credential generations retained: %w", err)
		}
		return err
	}
	if !replaced {
		return errors.New("configuration writer did not replace the file")
	}
	if previous.CredentialStore == CredentialStoreKeychain {
		var obsolete []*ServerDetails
		for _, server := range previous.Servers {
			if server != nil && !retained[server.CredentialRef] {
				obsolete = append(obsolete, server)
			}
		}
		if err = s.deleteCredentialRefs(service, obsolete); err != nil {
			return fmt.Errorf("configuration saved, but old keychain credentials could not be removed: %w", err)
		}
	}
	return nil
}

func (s *credentialStore) storeCredentials(service string, server *ServerDetails, created *[]*ServerDetails) error {
	data, err := json.Marshal(secretsOf(server))
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	chunks := (len(encoded) + credentialChunkSize - 1) / credentialChunkSize
	if chunks > maxCredentialChunks {
		return errors.New("credentials exceed keychain storage limit")
	}
	server.CredentialRef, server.CredentialChunks = uuid.NewString(), 0
	*created = append(*created, server)
	for i := 0; i < chunks; i++ {
		end := min((i+1)*credentialChunkSize, len(encoded))
		// Include the attempted chunk in rollback: a backend can report an
		// error after storing it. Deleting a missing chunk is harmless.
		server.CredentialChunks++
		if err = s.keyring.Set(service, chunkName(server.CredentialRef, i), encoded[i*credentialChunkSize:end]); err != nil {
			return keychainError("write", err)
		}
	}
	return nil
}

func (s *credentialStore) deleteCredentialRefs(service string, servers []*ServerDetails) error {
	var result error
	for _, server := range servers {
		if server == nil || server.CredentialRef == "" {
			continue
		}
		if server.CredentialChunks < 0 || server.CredentialChunks > maxCredentialChunks {
			result = errors.Join(result, errors.New("invalid keychain chunk count"))
			continue
		}
		for i := 0; i < server.CredentialChunks; i++ {
			err := s.keyring.Delete(service, chunkName(server.CredentialRef, i))
			if err != nil && !errors.Is(err, keyring.ErrNotFound) {
				result = errors.Join(result, keychainError("delete", err))
			}
		}
	}
	return result
}

func atomicWriteConfig(path string, content []byte) (bool, error) {
	return writeConfigWithReplace(path, content, replaceCredentialConfig)
}

func writeConfigWithReplace(path string, content []byte, replace func(string, string) (bool, error)) (bool, error) {
	file, err := os.CreateTemp(filepath.Dir(path), ".jfrog-config-*")
	if err != nil {
		return false, err
	}
	defer func() { _ = os.Remove(file.Name()) }()
	if _, err = file.Write(content); err != nil {
		return false, errors.Join(err, file.Close())
	}
	if err = file.Sync(); err != nil {
		return false, errors.Join(err, file.Close())
	}
	if err = file.Close(); err != nil {
		return false, err
	}
	return replace(file.Name(), path)
}
