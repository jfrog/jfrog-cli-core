package transferfiles

import (
	"github.com/jfrog/gofrog/parallel"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
)

type producerConsumerWrapper struct {
	chunkUploaderProducerConsumer parallel.Runner
	chunkBuilderProducerConsumer  parallel.Runner
	errorsQueue                   *clientUtils.ErrorsQueue
}
