package formats

import (
	"bytes"
	"encoding/json"
)

// Structs in this file should NOT be changed!
// The structs are used as an API for the build-publish command, thus changing their structure or the 'json' annotation will break the API.

type BuildPublishOutput struct {
	BuildInfoUiUrl string `json:"buildInfoUiUrl,omitempty"`
}

// This function is similar to json.Marshal with EscapeHTML false.
func (bpo *BuildPublishOutput) JSON() ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(bpo)
	return buffer.Bytes(), err
}
