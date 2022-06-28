package summary

import (
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

func TestWriteToErrorsFile(t *testing.T) {
	//repoName := "repo"
	//path := "repo/dir/file"
	//fileName := "file"
	//statusCode := 404

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(1)
	channel := make(chan ErrorEntity, 5)
	//go func() {
	//for i := 0; i < 10; i++ {
	go func() {
		err := WriteTransferErrorsToFile("repo", 1, 1, channel)
		assert.NoError(t, err)
		// TODO: why?
		//waitGroup.Done()
	}()
	//time.Sleep(100000000)
	go func() {
		channel <- newErrorEntity("repo", "repo/dir/file", "file", "SUCCESS", 1, "error")
	}()
	//time.Sleep(100000000)
	go func() {
		channel <- newErrorEntity("repo", "repo/dir/file", "file", "SUCCESS", 2, "error")
	}()
	//time.Sleep(1000000)
	go func() {
		channel <- newErrorEntity("repo", "repo/dir/file", "file", "SUCCESS", 3, "error")
	}()
	//time.Sleep(1000000)
	go func() {
		channel <- newErrorEntity("repo", "repo/dir/file", "file", "SUCCESS", 4, "error")
	}()
	//}
	//time.Sleep(10000000000)
	go func() {
		channel <- newErrorEntity("repo", "repo/dir/file", "file", "SUCCESS", FINISH, "error")
	}()
	//}()
	waitGroup.Wait()
	//time.Sleep(100000000000000)

}
