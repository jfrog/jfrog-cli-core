package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

type memoryCredentialKeyring struct {
	mu                        sync.Mutex
	values                    map[string]string
	getErr, setErr, deleteErr error
	sets, deletes, failSetAt  int
	failAfterStore            bool
}

func (m *memoryCredentialKeyring) Get(service, user string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getErr != nil {
		return "", m.getErr
	}
	value, ok := m.values[service+user]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return value, nil
}
func (m *memoryCredentialKeyring) Set(service, user, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sets++
	if m.setErr != nil && (m.failSetAt == 0 || m.sets == m.failSetAt) {
		if m.failAfterStore {
			m.values[service+user] = value
		}
		return m.setErr
	}
	m.values[service+user] = value
	return nil
}
func (m *memoryCredentialKeyring) Delete(service, user string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletes++
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.values, service+user)
	return nil
}

func credentialTestEnv(t *testing.T) (*credentialStore, *memoryCredentialKeyring) {
	t.Helper()
	t.Setenv(coreutils.HomeDir, t.TempDir())
	t.Setenv(coreutils.EncryptionKey, "")
	m := &memoryCredentialKeyring{values: map[string]string{}}
	s := newCredentialStore()
	s.keyring = m
	return s, m
}

func testCredentialServer() *ServerDetails {
	return &ServerDetails{ServerId: "test", Url: "https://example.jfrog.io/", IsDefault: true, User: "user",
		Password: "test-password", AccessToken: "test-access", RefreshToken: "test-refresh",
		ArtifactoryRefreshToken: "test-art-refresh", SshPassphrase: "test-ssh"}
}

func configBytes(t *testing.T) []byte {
	t.Helper()
	path, err := getConfFilePath()
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func TestCredentialStoreRoundTrip(t *testing.T) {
	s, m := credentialTestEnv(t)
	server := testCredentialServer()
	require.NoError(t, s.saveServersConf([]*ServerDetails{server}))
	require.Empty(t, m.values)
	require.NoError(t, s.setCredentialStore(CredentialStoreKeychain))
	for _, secret := range []string{server.Password, server.AccessToken, server.RefreshToken, server.ArtifactoryRefreshToken, server.SshPassphrase} {
		require.NotContains(t, string(configBytes(t)), secret)
	}
	conf, err := s.readConf()
	require.NoError(t, err)
	require.Equal(t, CredentialStoreKeychain, conf.CredentialStore)
	require.Equal(t, secretsOf(server), secretsOf(conf.Servers[0]))
	// Portable serialization excludes machine-local references.
	data, err := json.Marshal(fromServerDetails(conf.Servers[0]))
	require.NoError(t, err)
	require.NotContains(t, string(data), "credentialRef")
	imported, err := Import(base64.StdEncoding.EncodeToString(data))
	require.NoError(t, err)
	require.Equal(t, server.Password, imported.Password)
	require.Equal(t, server.AccessToken, imported.AccessToken)
	require.NoError(t, s.setCredentialStore(CredentialStoreFile))
	require.Empty(t, m.values)
	require.Contains(t, string(configBytes(t)), server.Password)
	require.NotContains(t, string(configBytes(t)), "credentialRef")
}

func TestCredentialStoreStickyUpdatesAndRemoval(t *testing.T) {
	s, m := credentialTestEnv(t)
	require.NoError(t, s.setCredentialStore(CredentialStoreKeychain))
	server := testCredentialServer()
	require.NoError(t, s.saveServersConf([]*ServerDetails{server}))
	require.Equal(t, "test-password", server.Password)
	require.Empty(t, server.CredentialRef)
	oldKeys := cloneCredentialValues(m.values)
	server.AccessToken, server.RefreshToken = "updated-access", "updated-refresh"
	require.NoError(t, s.saveServersConf([]*ServerDetails{server}))
	for key := range oldKeys {
		require.NotContains(t, m.values, key)
	}
	updated, err := s.readConf()
	require.NoError(t, err)
	require.Equal(t, "updated-access", updated.Servers[0].AccessToken)
	require.NotContains(t, string(configBytes(t)), "updated-access")
	require.NoError(t, s.saveServersConf(nil))
	require.Empty(t, m.values)
}

func TestCredentialStoreUnavailableNeverFallsBack(t *testing.T) {
	s, m := credentialTestEnv(t)
	require.NoError(t, s.saveServersConf([]*ServerDetails{testCredentialServer()}))
	before := configBytes(t)
	m.setErr = errors.New("locked")
	require.ErrorContains(t, s.setCredentialStore(CredentialStoreKeychain), "no file fallback")
	require.Equal(t, before, configBytes(t))
	m.setErr = nil
	require.NoError(t, s.setCredentialStore(CredentialStoreKeychain))
	before = configBytes(t)
	m.getErr = errors.New("locked")
	ids, err := GetAllServerIds()
	require.NoError(t, err)
	require.Equal(t, []string{"test"}, ids)
	exists, err := IsServerConfExists()
	require.NoError(t, err)
	require.True(t, exists)
	_, err = s.readConf()
	require.ErrorContains(t, err, "no file fallback")
	require.Error(t, s.saveServersConf([]*ServerDetails{testCredentialServer()}))
	require.Equal(t, before, configBytes(t))
}

func cloneCredentialValues(values map[string]string) map[string]string {
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func TestCredentialStoreConfigWriteFailurePreservesOldSecrets(t *testing.T) {
	s, m := credentialTestEnv(t)
	require.NoError(t, s.saveServersConf([]*ServerDetails{testCredentialServer()}))
	require.NoError(t, s.setCredentialStore(CredentialStoreKeychain))
	before := configBytes(t)
	oldValues := cloneCredentialValues(m.values)
	s.writeConfig = func(string, []byte) (bool, error) { return false, errors.New("disk full") }
	server := testCredentialServer()
	server.AccessToken = "replacement"
	require.ErrorContains(t, s.saveServersConf([]*ServerDetails{server}), "disk full")
	require.Equal(t, before, configBytes(t))
	require.Equal(t, oldValues, m.values)
	loaded, err := s.readConf()
	require.NoError(t, err)
	require.Equal(t, "test-access", loaded.Servers[0].AccessToken)
}

func TestCredentialStoreLargeCredentialsAndPartialWrite(t *testing.T) {
	s, m := credentialTestEnv(t)
	require.NoError(t, s.setCredentialStore(CredentialStoreKeychain))
	server := testCredentialServer()
	server.AccessToken = strings.Repeat("long-token-", 1000)
	require.NoError(t, s.saveServersConf([]*ServerDetails{server}))
	require.Greater(t, len(m.values), 1)
	for _, value := range m.values {
		require.LessOrEqual(t, len(value), credentialChunkSize)
	}
	loaded, err := s.readConf()
	require.NoError(t, err)
	require.Equal(t, server.AccessToken, loaded.Servers[0].AccessToken)
	before, oldValues := configBytes(t), cloneCredentialValues(m.values)
	server.AccessToken += "changed"
	m.setErr, m.failSetAt = errors.New("write failed"), m.sets+2
	require.Error(t, s.saveServersConf([]*ServerDetails{server}))
	require.Equal(t, before, configBytes(t))
	require.Equal(t, oldValues, m.values)
}

func TestCredentialStoreBackendErrorAfterStoringRollsBack(t *testing.T) {
	s, m := credentialTestEnv(t)
	require.NoError(t, s.setCredentialStore(CredentialStoreKeychain))
	server := testCredentialServer()
	require.NoError(t, s.saveServersConf([]*ServerDetails{server}))
	before, oldValues := configBytes(t), cloneCredentialValues(m.values)
	server.AccessToken = "changed"
	m.setErr, m.failAfterStore = errors.New("failure after storing"), true
	require.ErrorContains(t, s.saveServersConf([]*ServerDetails{server}), "failure after storing")
	require.Equal(t, before, configBytes(t))
	require.Equal(t, oldValues, m.values)
}

func TestCredentialStoreNullServerReturnsError(t *testing.T) {
	credentialTestEnv(t)
	path, err := getConfFilePath()
	require.NoError(t, err)
	data, err := json.Marshal(&Config{ConfigV6: ConfigV6{ConfigV5: ConfigV5{
		Version: strconv.Itoa(coreutils.GetCliConfigVersion()), Servers: []*ServerDetails{nil},
	}}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))
	_, err = GetAllServerIds()
	require.ErrorContains(t, err, "invalid null server configuration")
}

func TestCredentialStoreEncryptionCompatibility(t *testing.T) {
	s, m := credentialTestEnv(t)
	t.Setenv(coreutils.EncryptionKey, strings.Repeat("k", 32))
	server := testCredentialServer()
	require.NoError(t, s.saveServersConf([]*ServerDetails{server}))
	require.NotContains(t, string(configBytes(t)), server.Password)
	require.NoError(t, s.setCredentialStore(CredentialStoreKeychain))
	for i := 0; i < 2; i++ {
		loaded, err := s.readConf()
		require.NoError(t, err)
		require.Equal(t, secretsOf(server), secretsOf(loaded.Servers[0]))
	}
	require.NoError(t, s.setCredentialStore(CredentialStoreFile))
	require.Empty(t, m.values)
	require.NotContains(t, string(configBytes(t)), server.Password)
	loaded, err := s.readConf()
	require.NoError(t, err)
	require.Equal(t, secretsOf(server), secretsOf(loaded.Servers[0]))
}

func TestCredentialStoreIsolatedHomes(t *testing.T) {
	s, m := credentialTestEnv(t)
	require.NoError(t, s.saveServersConf([]*ServerDetails{testCredentialServer()}))
	require.NoError(t, s.setCredentialStore(CredentialStoreKeychain))
	firstService, err := credentialService()
	require.NoError(t, err)
	firstCount := len(m.values)
	t.Setenv(coreutils.HomeDir, t.TempDir())
	secondService, err := credentialService()
	require.NoError(t, err)
	require.NotEqual(t, firstService, secondService)
	require.NoError(t, s.setCredentialStore(CredentialStoreKeychain))
	require.NoError(t, s.saveServersConf([]*ServerDetails{testCredentialServer()}))
	require.Len(t, m.values, firstCount*2)
	require.NoError(t, s.saveServersConf(nil))
	require.Len(t, m.values, firstCount)
}

func TestCredentialStoreValidationAndCorruption(t *testing.T) {
	for _, test := range []struct{ name, chunk, message string }{
		{"encoding", "not valid base64", "invalid keychain credential encoding"},
		{"empty", "", "invalid keychain credential chunk size"},
		{"oversized", strings.Repeat("x", credentialChunkSize+1), "invalid keychain credential chunk size"},
		{"json", base64.StdEncoding.EncodeToString([]byte(`{"password":"secret-leak",`)), "invalid keychain credential data"},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, m := credentialTestEnv(t)
			require.Error(t, s.setCredentialStore(""))
			require.Error(t, s.setCredentialStore("unknown"))
			require.NoError(t, s.setCredentialStore(CredentialStoreKeychain))
			require.NoError(t, s.saveServersConf([]*ServerDetails{testCredentialServer()}))
			for key := range m.values {
				m.values[key] = test.chunk
			}
			_, err := s.readConf()
			require.ErrorContains(t, err, test.message)
			require.NotContains(t, err.Error(), "secret-leak")
			var raw Config
			require.NoError(t, json.Unmarshal(configBytes(t), &raw))
			raw.Servers[0].CredentialChunks = maxCredentialChunks + 1
			require.ErrorContains(t, s.loadCredentials(&raw), "invalid keychain credential reference")
			raw.CredentialStore = CredentialStoreFile
			require.ErrorContains(t, s.loadCredentials(&raw), "references found")
		})
	}
}

func TestCredentialStoreDeleteFailureReportsCommittedState(t *testing.T) {
	s, m := credentialTestEnv(t)
	require.NoError(t, s.setCredentialStore(CredentialStoreKeychain))
	require.NoError(t, s.saveServersConf([]*ServerDetails{testCredentialServer()}))
	m.deleteErr = errors.New("delete denied")
	require.ErrorContains(t, s.saveServersConf(nil), "configuration saved")
	loaded, err := s.readConf()
	require.NoError(t, err)
	require.Empty(t, loaded.Servers)
	require.NotEmpty(t, m.values)
}

func TestCredentialStoreFailedLockAcquisitionCleansUp(t *testing.T) {
	s, _ := credentialTestEnv(t)
	dir, err := coreutils.GetJfrogConfigLockDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0700))
	invalid := filepath.Join(dir, "invalid-lock")
	require.NoError(t, os.WriteFile(invalid, nil, 0600))
	require.Error(t, s.setCredentialStore(CredentialStoreKeychain))
	require.NoError(t, os.Remove(invalid))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries)
	// No stale lock obstructs the next operation in the same live process.
	require.NoError(t, s.setCredentialStore(CredentialStoreKeychain))
}

func TestCredentialStoreUnchangedSecretsRetainReferences(t *testing.T) {
	s, m := credentialTestEnv(t)
	require.NoError(t, s.setCredentialStore(CredentialStoreKeychain))
	first, second := testCredentialServer(), testCredentialServer()
	second.ServerId = "second"
	require.NoError(t, s.saveServersConf([]*ServerDetails{first, second}))
	before, err := s.readConf()
	require.NoError(t, err)
	sets, deletes := m.sets, m.deletes
	first.Url = "https://changed.example/"
	require.NoError(t, s.saveServersConf([]*ServerDetails{first, second}))
	require.Equal(t, sets, m.sets)
	require.Equal(t, deletes, m.deletes)
	first.AccessToken = "refreshed"
	require.NoError(t, s.saveServersConf([]*ServerDetails{first, second}))
	require.Equal(t, sets+1, m.sets)
	require.Equal(t, deletes+1, m.deletes)
	after, err := s.readConf()
	require.NoError(t, err)
	require.NotEqual(t, before.Servers[0].CredentialRef, after.Servers[0].CredentialRef)
	require.Equal(t, before.Servers[1].CredentialRef, after.Servers[1].CredentialRef)
}

func TestCredentialStorePostReplacementFailureRetainsBothGenerations(t *testing.T) {
	s, m := credentialTestEnv(t)
	require.NoError(t, s.setCredentialStore(CredentialStoreKeychain))
	server := testCredentialServer()
	require.NoError(t, s.saveServersConf([]*ServerDetails{server}))
	oldValues := cloneCredentialValues(m.values)
	s.writeConfig = func(path string, data []byte) (bool, error) {
		replaced, err := atomicWriteConfig(path, data)
		if err != nil {
			return replaced, err
		}
		return true, errors.New("directory sync failed")
	}
	server.AccessToken = "replacement"
	require.ErrorContains(t, s.saveServersConf([]*ServerDetails{server}), "both credential generations retained")
	for key, value := range oldValues {
		require.Equal(t, value, m.values[key])
	}
	require.Greater(t, len(m.values), len(oldValues))
	loaded, err := s.readConf()
	require.NoError(t, err)
	require.Equal(t, "replacement", loaded.Servers[0].AccessToken)
}

type blockingCredentialKeyring struct {
	credentialKeyring
	entered, release chan struct{}
	once             sync.Once
}

func (b *blockingCredentialKeyring) Get(service, user string) (string, error) {
	b.once.Do(func() { close(b.entered); <-b.release })
	return b.credentialKeyring.Get(service, user)
}

func TestCredentialStoreReaderSnapshotCannotBeInvalidated(t *testing.T) {
	s, m := credentialTestEnv(t)
	require.NoError(t, s.setCredentialStore(CredentialStoreKeychain))
	server := testCredentialServer()
	require.NoError(t, s.saveServersConf([]*ServerDetails{server}))
	before := configBytes(t)
	blocking := &blockingCredentialKeyring{credentialKeyring: m, entered: make(chan struct{}), release: make(chan struct{})}
	reader := *s
	reader.keyring = blocking
	type result struct {
		conf *Config
		err  error
	}
	readDone := make(chan result, 1)
	go func() { conf, err := reader.readConf(); readDone <- result{conf, err} }()
	<-blocking.entered
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(blocking.release) }) }
	defer release()
	server.AccessToken = "new-token"
	writeDone := make(chan error, 1)
	go func() { writeDone <- s.saveServersConf([]*ServerDetails{server}) }()
	dir, err := coreutils.GetJfrogConfigLockDir()
	require.NoError(t, err)
	require.Eventually(t, func() bool { entries, err := os.ReadDir(dir); return err == nil && len(entries) == 2 }, 3*time.Second, time.Millisecond)
	require.Equal(t, before, configBytes(t))
	release()
	read := <-readDone
	require.NoError(t, read.err)
	require.Equal(t, "test-access", read.conf.Servers[0].AccessToken)
	require.NoError(t, <-writeDone)
	updated, err := s.readConf()
	require.NoError(t, err)
	require.Equal(t, "new-token", updated.Servers[0].AccessToken)
}

func TestCredentialStoreAutomaticEncryptionCannotUndoMigration(t *testing.T) {
	s, _ := credentialTestEnv(t)
	require.NoError(t, s.saveServersConf([]*ServerDetails{testCredentialServer()}))
	// A prior metadata snapshot is never passed to an encrypt-on-read writer.
	stale, err := readConfMetadata()
	require.NoError(t, err)
	require.Empty(t, stale.CredentialStore)
	require.NoError(t, s.setCredentialStore(CredentialStoreKeychain))
	before := configBytes(t)
	t.Setenv(coreutils.EncryptionKey, strings.Repeat("k", 32))
	loaded, err := s.readConf()
	require.NoError(t, err)
	require.Equal(t, CredentialStoreKeychain, loaded.CredentialStore)
	require.Equal(t, before, configBytes(t))
	// decrypt itself is now pure: even an old snapshot cannot trigger a write.
	require.NoError(t, stale.decrypt())
	require.Equal(t, before, configBytes(t))
}

func TestCredentialStoreEncryptionAndMigrationSerialize(t *testing.T) {
	s, _ := credentialTestEnv(t)
	require.NoError(t, s.saveServersConf([]*ServerDetails{testCredentialServer()}))
	t.Setenv(coreutils.EncryptionKey, strings.Repeat("k", 32))
	reader := *s
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	reader.writeConfig = func(path string, data []byte) (bool, error) {
		close(entered)
		<-release
		return atomicWriteConfig(path, data)
	}
	readDone := make(chan error, 1)
	go func() { _, err := reader.readConf(); readDone <- err }()
	<-entered
	migrationDone := make(chan error, 1)
	go func() { migrationDone <- s.setCredentialStore(CredentialStoreKeychain) }()
	dir, err := coreutils.GetJfrogConfigLockDir()
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		entries, err := os.ReadDir(dir)
		return err == nil && len(entries) == 2
	}, 3*time.Second, time.Millisecond)
	unblock()
	require.NoError(t, <-readDone)
	require.NoError(t, <-migrationDone)
	conf, err := s.readConf()
	require.NoError(t, err)
	require.Equal(t, CredentialStoreKeychain, conf.CredentialStore)
	require.Equal(t, "test-password", conf.Servers[0].Password)
	require.NotContains(t, string(configBytes(t)), "test-password")
}

func TestCredentialStoreRejectsTruncatedIntermediateChunk(t *testing.T) {
	s, m := credentialTestEnv(t)
	require.NoError(t, s.setCredentialStore(CredentialStoreKeychain))
	server := testCredentialServer()
	server.AccessToken = strings.Repeat("large", 1000)
	require.NoError(t, s.saveServersConf([]*ServerDetails{server}))
	for key := range m.values {
		if strings.HasSuffix(key, "/0") {
			m.values[key] = "AAAA"
		}
	}
	_, err := s.readConf()
	require.ErrorContains(t, err, "invalid keychain credential chunk size")
}

func TestAtomicCredentialConfigWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	replaced, err := atomicWriteConfig(path, []byte("first"))
	require.NoError(t, err)
	require.True(t, replaced)
	replaced, err = writeConfigWithReplace(path, []byte("second"), func(source, destination string) (bool, error) {
		data, readErr := os.ReadFile(source)
		require.NoError(t, readErr)
		require.Equal(t, "second", string(data))
		info, statErr := os.Stat(source)
		require.NoError(t, statErr)
		if os.PathSeparator != '\\' {
			require.Equal(t, os.FileMode(0600), info.Mode().Perm())
		}
		return false, errors.New("rename failed")
	})
	require.ErrorContains(t, err, "rename failed")
	require.False(t, replaced)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "first", string(data))
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	require.Len(t, entries, 1)
}
