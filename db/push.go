package db

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	nomsjson "roci.dev/diff-server/util/noms/json"

	"roci.dev/replicache-client/http"
)

type BatchPushRequest struct {
	ClientID  string     `json:"clientId"`
	Mutations []Mutation `json:"mutations"`
}

// Public because returned in the MaybeEndSyncResponse.
type Mutation struct {
	ID   uint64          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type ReplayMutation struct {
	Mutation
	Original *nomsjson.Hash `json:"original,omitempty"`
}

type BatchPushResponse struct {
	// Should log this in the client
	MutationInfos []MutationInfo `json:"mutationInfos,omitempty"`
}

// Should log this in the client
type MutationInfo struct {
	ID    uint64 `json:"id"`
	Error string `json:"error"`
}

type BatchPushInfo struct {
	HTTPStatusCode    int               `json:"httpStatusCode"`
	ErrorMessage      string            `json:"errorMessage"`
	BatchPushResponse BatchPushResponse `json:"batchPushResponse"`
}

type pusher interface {
	Push(pending []Local, url string, dataLayerAuth string, obfuscatedClientID string) BatchPushInfo
}

type defaultPusher struct {
	t time.Duration
}

func (d *defaultPusher) timeout() time.Duration {
	if d.t == 0 {
		d.t = 20 * time.Second // Enough time to upload 4MB on a slow connection.
	}
	return d.t
}

// Push sends pending local commits to the batch endpoint. If the request was made
// the (maybe non-200) status code will be returned in the BatchPushInfo. The BatchPushInfo.ErrorMessage
// will contain any error message, eg the batch endpoint response body for non-200 status codes or an
// internal error message if for example the reqeust could not be sent or the response not be parsed.
func (d *defaultPusher) Push(pending []Local, url string, dataLayerAuth string, obfuscatedClientID string) BatchPushInfo {
	var info BatchPushInfo
	withErrMsg := func(msg string) BatchPushInfo {
		info.ErrorMessage = fmt.Sprintf("during request to %s: %s", url, msg)
		return info
	}

	var req BatchPushRequest
	req.ClientID = obfuscatedClientID
	for _, p := range pending {
		var args bytes.Buffer
		if err := nomsjson.ToJSON(p.Args, &args); err != nil {
			return withErrMsg(err.Error())
		}
		req.Mutations = append(req.Mutations, Mutation{p.MutationID, p.Name, args.Bytes()})
	}
	reqBody, err := json.Marshal(req)
	if err != nil {
		return withErrMsg(err.Error())
	}

	httpResp, err := http.Post(url, reqBody, d.timeout(), dataLayerAuth)
	if err != nil {
		return withErrMsg(err.Error())
	}

	info.HTTPStatusCode = httpResp.StatusCode
	if httpResp.StatusCode == http.StatusOK {
		var resp BatchPushResponse
		if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
			return withErrMsg(fmt.Sprintf("error decoding batch push response: %s", err))
		}
		info.BatchPushResponse = resp
	} else {
		body, err := ioutil.ReadAll(httpResp.Body)
		var s string
		if err == nil {
			s = string(body)
		} else {
			s = err.Error()
		}
		info.ErrorMessage = s
	}

	return info
}
