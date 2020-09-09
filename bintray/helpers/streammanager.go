package helpers

import (
	"bufio"
	"encoding/json"
	"errors"
	"github.com/jfrog/jfrog-client-go/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

const BINTRAY_RECONNECT_HEADER = "X-Bintray-Stream-Reconnect-Id"

type StreamManager struct {
	HttpClientDetails httputils.HttpClientDetails
	Url               string
	IncludeFilter     map[string]struct{}
	ReconnectId       string
}

func (sm *StreamManager) ReadStream(resp *http.Response, writer io.Writer, lastServerInteraction *time.Time) {
	ioReader := resp.Body
	bodyReader := bufio.NewReader(ioReader)
	sm.handleStream(bodyReader, writer, lastServerInteraction)
}

func (sm *StreamManager) handleStream(ioReader io.Reader, writer io.Writer, lastServerInteraction *time.Time) {
	bodyReader := bufio.NewReader(ioReader)
	pReader, pWriter := io.Pipe()
	defer pWriter.Close()
	go func() {
		defer pReader.Close()
		for {
			line, _, err := bodyReader.ReadLine()
			if err != nil {
				log.Debug(err)
				break
			}
			*lastServerInteraction = time.Now()
			_, err = pWriter.Write(line)
			if err != nil {
				log.Debug(err)
				break
			}
		}
	}()
	streamDecoder := json.NewDecoder(pReader)
	streamEncoder := json.NewEncoder(writer)
	sm.parseStream(streamDecoder, streamEncoder)
}

func (sm *StreamManager) parseStream(streamDecoder *json.Decoder, streamEncoder *json.Encoder) error {
	for {
		var decodedJson map[string]interface{}
		if e := streamDecoder.Decode(&decodedJson); e != nil {
			log.Debug(e)
			return e
		}
		if _, ok := sm.IncludeFilter[decodedJson["type"].(string)]; ok || len(sm.IncludeFilter) == 0 {
			if e := streamEncoder.Encode(&decodedJson); e != nil {
				log.Debug(e)
				return e
			}
		}
	}
}

func (sm *StreamManager) isReconnection() bool {
	return len(sm.ReconnectId) > 0
}

func (sm *StreamManager) setReconnectHeader() {
	if sm.HttpClientDetails.Headers == nil {
		sm.HttpClientDetails.Headers = map[string]string{}
	}
	sm.HttpClientDetails.Headers[BINTRAY_RECONNECT_HEADER] = sm.ReconnectId
}

func (sm *StreamManager) Connect() (bool, *http.Response, error) {
	if sm.isReconnection() {
		sm.setReconnectHeader()
	}
	log.Debug("Connecting...")
	client, err := httpclient.ClientBuilder().Build()
	if err != nil {
		return false, nil, nil
	}
	resp, _, _, e := client.Stream(sm.Url, sm.HttpClientDetails)
	if e != nil {
		return false, resp, nil
	}
	if resp.StatusCode != http.StatusOK {
		errorutils.CheckError(errors.New("response: " + resp.Status))
		msgBody, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode > 400 && resp.StatusCode < 500 {
			return false, nil, errors.New(string(msgBody))
		}
		return false, resp, nil

	}
	sm.ReconnectId = resp.Header.Get(BINTRAY_RECONNECT_HEADER)
	log.Debug("Connected.")
	return true, resp, nil
}
