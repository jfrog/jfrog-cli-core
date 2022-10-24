package transferinstall

import (
	"testing"
)

func TestInstallTransferPlugin(t *testing.T) {
	// TODO: this is template, change when done
	//testServer, serverDetails, serviceManager := commonTests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
	//w.WriteHeader(http.StatusOK)
	//
	//// Read body
	//content, err := io.ReadAll(r.Body)
	//assert.NoError(t, err)
	//var actual services.ExportBody
	//assert.NoError(t, json.Unmarshal(content, &actual))
	//
	//// Make sure all parameters as expected
	//assert.False(t, *actual.IncludeMetadata)
	//assert.False(t, *actual.Verbose)
	//assert.True(t, *actual.ExcludeContent)
	//assert.Nil(t, actual.CreateArchive)
	//assert.Nil(t, actual.M2)
	//
	//// Create the export-dir in the export path
	//assert.NoError(t, os.Mkdir(filepath.Join(actual.ExportPath, "export-dir"), os.ModePerm))
	//})
	//defer testServer.Close()

	//transferInstallCmd := NewTransferInstallCommand(serverDetails)
}

//func TestGetInstallDirectoryV6(t *testing.T) {
//	testServer, serverDetails, serviceManager := commonTests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
//
//	})
//	defer testServer.Close()
//
//	transferInstallCmd := NewTransferInstallCommand(serverDetails)
//	targetDir, err := transferInstallCmd.getInstallTargetDirectory(serviceManager)
//
//	assert.NoError(t, err)
//	assert.Equal(t, "EXPECTED", targetDir)
//}
//
//func TestGetInstallDirectoryV7Above(t *testing.T) {
//	testServer, serverDetails, serviceManager := commonTests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
//
//	})
//	defer testServer.Close()
//
//	transferInstallCmd := NewTransferInstallCommand(serverDetails)
//	targetDir, err := transferInstallCmd.getInstallTargetDirectory(serviceManager)
//
//	assert.NoError(t, err)
//	assert.Equal(t, "EXPECTED", targetDir)
//}
//
//func TestIsV7Above(t *testing.T) {
//
//	minTestVer := 5
//	maxTestVer := 9
//	numSubVer := 3
//
//	for v := minTestVer; v < maxTestVer; v++ {
//		for sv := 0; sv < numSubVer; sv++ {
//			version := fmt.Sprintf("%d", v)
//			for i := 0; i < sv; i++ {
//				version += fmt.Sprintf(".%d", rand.Int())
//			}
//			isV7Above, err := isArtifactoryV7Above(version)
//			assert.NoError(t, err)
//			if v < 7 {
//				assert.False(t, isV7Above)
//			} else {
//				assert.True(t, isV7Above)
//			}
//		}
//	}
//
//	// Errors
//	_, err := isArtifactoryV7Above("")
//	assert.Error(t, err)
//	_, err = isArtifactoryV7Above("asfs.aa")
//	assert.Error(t, err)
//	_, err = isArtifactoryV7Above("aaf.7")
//	assert.Error(t, err)
//	_, err = isArtifactoryV7Above(".6")
//	assert.Error(t, err)
//}
//
//func assertVersionByString(t *testing.T, shouldBeBigger bool) {
//	isV7Above, err := isArtifactoryV7Above("")
//	assert.NoError(t, err)
//	assert.Equal(t, isV7Above, shouldBeBigger)
//}

func TestVerifyServerConnectivity(t *testing.T) {
	//testServer, serverDetails, _ := commonTests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
	//	if r.RequestURI == "/"+pluginsExecuteRestApi+"verifySourceTargetConnectivity" {
	//		w.WriteHeader(http.StatusOK)
	//	}
	//})
	//defer testServer.Close()
	//srcPluginManager := initSrcUserPluginServiceManager(t, serverDetails)
	//transferFilesCommand, err := NewTransferFilesCommand(serverDetails, serverDetails)
	//assert.NoError(t, err)
	//err = transferFilesCommand.verifySourceTargetConnectivity(srcPluginManager)
	//assert.NoError(t, err)
}

func TestVerifyServerConnectivityError(t *testing.T) {
	//testServer, serverDetails, _ := commonTests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
	//	if r.RequestURI == "/"+pluginsExecuteRestApi+"verifySourceTargetConnectivity" {
	//		w.WriteHeader(http.StatusBadRequest)
	//		_, err := w.Write([]byte("No connection to target"))
	//		assert.NoError(t, err)
	//	}
	//})
	//defer testServer.Close()
	//srcPluginManager := initSrcUserPluginServiceManager(t, serverDetails)
	//transferFilesCommand, err := NewTransferFilesCommand(serverDetails, serverDetails)
	//assert.NoError(t, err)
	//err = transferFilesCommand.verifySourceTargetConnectivity(srcPluginManager)
	//assert.ErrorContains(t, err, "No connection to target")
}
