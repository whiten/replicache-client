package db

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"roci.dev/diff-server/kv"
	servetypes "roci.dev/diff-server/serve/types"
	"roci.dev/replicache-client/http"

	"github.com/attic-labs/noms/go/types"
	"github.com/attic-labs/noms/go/util/verbose"
	"github.com/pkg/errors"
)

func baseSnapshot(noms types.ValueReadWriter, c Commit) (Commit, error) {
	if c.Type() == CommitTypeSnapshot {
		return c, nil
	}
	basis, err := c.Basis(noms)
	if err != nil {
		return Commit{}, fmt.Errorf("could not find base snapshot of %v: %w", c.NomsStruct.Hash(), err)
	}
	return baseSnapshot(noms, basis)
}

type puller interface {
	Pull(noms types.ValueReadWriter, baseState Commit, url string, diffServerAuth string, clientViewAuth string, clientID string) (Commit, servetypes.ClientViewInfo, error)
}

type defaultPuller struct {
	t time.Duration
}

func (d *defaultPuller) timeout() time.Duration {
	if d.t == 0 {
		d.t = 20 * time.Second // Enough time to download 4MB on a slow connection.
	}
	return d.t
}

// Pull pulls new server state from the client view via the diffserver.
func (d *defaultPuller) Pull(noms types.ValueReadWriter, baseState Commit, url string, diffServerAuth string, clientViewAuth string, clientID string) (Commit, servetypes.ClientViewInfo, error) {
	baseMap := baseState.Data(noms)
	pullReq, err := json.Marshal(servetypes.PullRequest{
		ClientViewAuth: clientViewAuth,
		ClientID:       clientID,
		BaseStateID:    baseState.Meta.Snapshot.ServerStateID,
		Checksum:       baseMap.Checksum(),
	})
	if err != nil {
		return Commit{}, servetypes.ClientViewInfo{}, errors.New("could not marshal PullRequest")
	}
	verbose.Log("Pulling: %s from baseStateID %s with auth %s", url, baseState.Meta.Snapshot.ServerStateID, clientViewAuth)

	resp, err := http.Post(url, pullReq, d.timeout(), diffServerAuth)
	if err != nil {
		return Commit{}, servetypes.ClientViewInfo{}, err
	}

	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		var s string
		if err == nil {
			s = string(body)
		} else {
			s = err.Error()
		}
		return Commit{}, servetypes.ClientViewInfo{}, fmt.Errorf("status code %s: %s", resp.Status, s)
	}

	var pullResp servetypes.PullResponse
	var r io.Reader = resp.Body
	err = json.NewDecoder(r).Decode(&pullResp)
	if err != nil {
		return Commit{}, servetypes.ClientViewInfo{}, fmt.Errorf("response from %s is not valid JSON: %s", url, err.Error())
	}

	if pullResp.LastMutationID < baseState.Meta.Snapshot.LastMutationID {
		return Commit{}, pullResp.ClientViewInfo, fmt.Errorf("client view lastMutationID %d is < previous lastMutationID %d; ignoring", pullResp.LastMutationID, baseState.Meta.Snapshot.LastMutationID)
	}
	patchedMap, err := kv.ApplyPatch(noms, baseMap, pullResp.Patch)
	if err != nil {
		return Commit{}, pullResp.ClientViewInfo, errors.Wrap(err, "couldn't apply patch")
	}
	expectedChecksum, err := kv.ChecksumFromString(pullResp.Checksum)
	if err != nil {
		return Commit{}, pullResp.ClientViewInfo, errors.Wrapf(err, "response checksum malformed: %s", pullResp.Checksum)
	}
	if patchedMap.Checksum() != expectedChecksum.String() {
		return Commit{}, pullResp.ClientViewInfo, fmt.Errorf("checksum mismatch! Expected %s, got %s", expectedChecksum, patchedMap.Checksum())
	}
	newSnapshot := makeSnapshot(noms, baseState.Ref(), pullResp.StateID, noms.WriteValue(patchedMap.NomsMap()), patchedMap.NomsChecksum(), pullResp.LastMutationID)
	return newSnapshot, pullResp.ClientViewInfo, nil
}
