package formats

import (
	"bytes"
	"encoding/json"
)

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
