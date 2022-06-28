package transferfiles

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestWriteToErrorsFile(t *testing.T) {
	//repoName := "repo"
	//path := "repo/dir/file"
	//fileName := "file"
	//statusCode := 404

	//waitGroup := sync.WaitGroup{}
	//waitGroup.Add(1)
	channel := make(chan PropertiesHandlingError, 5)
	//go func() {
	//for i := 0; i < 10; i++ {
	go func() {
		err := WriteTransferErrorsToFile("repo", 1, "67576575675", channel)
		assert.NoError(t, err)
		// TODO: why?
		//waitGroup.Done()
	}()
	//time.Sleep(100000000)
	go func() {
		channel <- PropertiesHandlingError{FileRepresentation: FileRepresentation{Repo: "repo", Path: "repo/dir/file", Name: "file"}, StatusCode: "SKIPPED_LARGE_PROPS", Reason: "1"}
		channel <- PropertiesHandlingError{FileRepresentation: FileRepresentation{Repo: "repo", Path: "repo/dir/file", Name: "file"}, StatusCode: "FAIL", Reason: "01"}
	}()
	//time.Sleep(100000000)
	go func() {
		channel <- PropertiesHandlingError{FileRepresentation: FileRepresentation{Repo: "repo", Path: "repo/dir/file", Name: "file"}, StatusCode: "SKIPPED_LARGE_PROPS", Reason: "2"}
		channel <- PropertiesHandlingError{FileRepresentation: FileRepresentation{Repo: "repo", Path: "repo/dir/file", Name: "file"}, StatusCode: "FAIL", Reason: "02"}
	}()
	//time.Sleep(1000000)
	go func() {
		channel <- PropertiesHandlingError{FileRepresentation: FileRepresentation{Repo: "repo", Path: "repo/dir/file", Name: "file"}, StatusCode: "SKIPPED_LARGE_PROPS", Reason: "3"}
		channel <- PropertiesHandlingError{FileRepresentation: FileRepresentation{Repo: "repo", Path: "repo/dir/file", Name: "file"}, StatusCode: "FAIL", Reason: "03"}
	}()
	//time.Sleep(1000000)
	go func() {
		channel <- PropertiesHandlingError{FileRepresentation: FileRepresentation{Repo: "repo", Path: "repo/dir/file", Name: "file"}, StatusCode: "SKIPPED_LARGE_PROPS", Reason: "4"}
		channel <- PropertiesHandlingError{FileRepresentation: FileRepresentation{Repo: "repo", Path: "repo/dir/file", Name: "file"}, StatusCode: "FAIL", Reason: "04"}
	}()
	//}
	//time.Sleep(10000000000)
	go func() {
		channel <- PropertiesHandlingError{FileRepresentation: FileRepresentation{Repo: "repo", Path: "repo/dir/file", Name: "file"}, StatusCode: "SKIPPED_LARGE_PROPS", Reason: "5"}
		channel <- PropertiesHandlingError{FileRepresentation: FileRepresentation{Repo: "repo", Path: "repo/dir/file", Name: "file"}, StatusCode: "FAIL", Reason: "05"}
	}()
	time.Sleep(5 * time.Second)
	close(channel)
	time.Sleep(5 * time.Hour)
	//}()
	//waitGroup.Wait()
	//time.Sleep(100000000000000)

}
