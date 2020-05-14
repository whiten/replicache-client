package repm

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	diffserve "roci.dev/diff-server/serve"
	servetypes "roci.dev/diff-server/serve/types"
	jsnoms "roci.dev/diff-server/util/noms/json"
	"roci.dev/replicache-client/db"
)

// dataLayer is a simple in-proceess data layer. It implements a single
// mutation 'myPut' which sets a key to a value.
// dataLayer is NOT safe for concurrent access.
type dataLayer struct {
	stores           map[string]*store // keyed by clientID
	authTokens       map[string]string // keyed by clientID
	batchServer      *httptest.Server
	clientViewServer *httptest.Server
}

// store is the data for a single client.
type store struct {
	lastMutationID uint64
	data           map[string]json.RawMessage
}

// newDataLayer returns a new dataLayer. Caller must call stop() to clean up.
func newDataLayer() *dataLayer {
	d := &dataLayer{stores: map[string]*store{}, authTokens: map[string]string{}}
	d.batchServer = httptest.NewServer(http.HandlerFunc(d.push))
	d.clientViewServer = httptest.NewServer(http.HandlerFunc(d.clientView))
	return d
}

func (d *dataLayer) stop() {
	d.batchServer.Close()
	d.clientViewServer.Close()
}

func (d *dataLayer) getStore(clientID string) *store {
	s := d.stores[clientID]
	if s != nil {
		return s
	}
	s = &store{0, make(map[string]json.RawMessage)}
	d.stores[clientID] = s
	return s
}

func (d *dataLayer) setAuthToken(clientID, authToken string) {
	d.authTokens[clientID] = authToken
}

func (d dataLayer) auth(clientID, authToken string) bool {
	return d.authTokens[clientID] == authToken
}

// push implements the batch push endpoint. It treats any error encountered while
// processing a mutation as permanent.
func (d *dataLayer) push(w http.ResponseWriter, r *http.Request) {
	var req db.BatchPushRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.ClientID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !d.auth(req.ClientID, r.Header.Get("Authorization")) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	s := d.getStore(req.ClientID)

	var resp db.BatchPushResponse
	for _, m := range req.Mutations {
		if m.ID <= s.lastMutationID {
			resp.MutationInfos = append(resp.MutationInfos, db.MutationInfo{ID: m.ID, Error: fmt.Sprintf("skipping this mutation: ID is less than %d", s.lastMutationID)})
			continue
		}
		s.lastMutationID = m.ID

		switch m.Name {
		case "myPut":
			var args myPutArgs
			err := json.Unmarshal(m.Args, &args)
			if err != nil {
				resp.MutationInfos = append(resp.MutationInfos, db.MutationInfo{ID: m.ID, Error: fmt.Sprintf("skipping this mutation: couldn't unmarshal putArgs: %s", err.Error())})
				continue
			}
			if len(args.Value) == 0 {
				resp.MutationInfos = append(resp.MutationInfos, db.MutationInfo{ID: m.ID, Error: fmt.Sprintf("skipping this mutation: value must be non-empty")})
			} else {
				s.data[args.Key] = args.Value
			}
		default:
			resp.MutationInfos = append(resp.MutationInfos, db.MutationInfo{ID: m.ID, Error: fmt.Sprintf("skipping this mutation: mutation '%s' not supported", m.Name)})
		}
	}

	b, err := json.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(b)
	return
}

// change is used by tests to change a client's data behind its back.
func (d *dataLayer) change(clientID string, key string, value json.RawMessage) {
	s := d.getStore(clientID)
	s.data[key] = value
}

type myPutArgs struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

func (d *dataLayer) clientView(w http.ResponseWriter, r *http.Request) {
	var req servetypes.ClientViewRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.ClientID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !d.auth(req.ClientID, r.Header.Get("Authorization")) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	s := d.getStore(req.ClientID)

	var resp servetypes.ClientViewResponse
	resp.LastMutationID = s.lastMutationID
	resp.ClientView = make(map[string]json.RawMessage)
	for k, v := range s.data {
		resp.ClientView[k] = v
	}

	b, err := json.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(b)
	return
}

// testEnv is an integration test environment. Careful: repm.connections[] is
// a global resource so we need to be sure to call deinit() and not attempt to
// run tests in parallel.
type testEnv struct {
	dbName        string
	api           api
	dataLayer     *dataLayer
	batchPushURL  string
	clientViewURL string
	account       diffserve.Account
	diffServer    *httptest.Server
	diffServerURL string
	teardowns     []func()
}

func (t testEnv) teardown() {
	for _, f := range t.teardowns {
		f()
	}
	deinit()
}

func newTestEnv(assert *assert.Assertions) testEnv {
	env := testEnv{dbName: "db1"}

	// Client
	clientDir, err := ioutil.TempDir("", "")
	Init(clientDir, "", nil)
	ret, err := Dispatch(env.dbName, "open", nil)
	assert.Nil(ret)
	assert.NoError(err)
	env.api = api{dbName: env.dbName, assert: assert}

	// Data layer
	env.dataLayer = newDataLayer()
	env.teardowns = append(env.teardowns, env.dataLayer.stop)
	env.batchPushURL = env.dataLayer.batchServer.URL
	env.clientViewURL = env.dataLayer.clientViewServer.URL

	// Diff server
	diffDir, _ := ioutil.TempDir("", "")
	// TODO phritz undo sandbox auth hardcoding ugh
	accounts := []diffserve.Account{{ID: "sandbox", Name: "sandbox", Pubkey: nil, ClientViewURL: env.clientViewURL}}
	env.account = accounts[0]
	diffService := diffserve.NewService(diffDir, accounts, "", diffserve.ClientViewGetter{}, false)
	diffServer := httptest.NewServer(diffService)
	env.diffServer = diffServer
	env.diffServerURL = fmt.Sprintf("%s/pull", env.diffServer.URL)
	env.teardowns = append(env.teardowns, diffServer.Close)

	return env
}

// myPut is the local (customer app) implementation of the myPut mutation.
func myPut(a api, key string, value json.RawMessage) commitTransactionResponse {
	putArgs := myPutArgs{Key: key, Value: value}
	otr := a.openTransaction("myPut", a.marshal(putArgs))
	putReq := putRequest{transactionRequest: transactionRequest{TransactionID: otr.TransactionID}, Key: key, Value: value}
	_, err := Dispatch(a.dbName, "put", a.marshal(putReq))
	a.assert.NoError(err)
	ctr := a.commitTransaction(otr.TransactionID)
	return ctr
}

// api calls the repm API.
type api struct {
	dbName string
	assert *assert.Assertions
}

func (a api) getRoot() getRootResponse {
	req := getRequest{}
	b, err := Dispatch(a.dbName, "getRoot", a.marshal(req))
	a.assert.NoError(err)
	a.assert.NotNil(b)
	var res getRootResponse
	a.unmarshal(b, &res)
	return res
}

func (a api) get(key string) getResponse {
	otr := a.openTransaction("", nil)
	req := getRequest{transactionRequest: transactionRequest{TransactionID: otr.TransactionID}, Key: key}
	b, err := Dispatch(a.dbName, "get", a.marshal(req))
	a.assert.NoError(err)
	a.assert.NotNil(b)
	a.closeTransaction(otr.TransactionID)
	var res getResponse
	a.unmarshal(b, &res)
	return res
}

func (a api) openTransaction(name string, args json.RawMessage) openTransactionResponse {
	req := openTransactionRequest{Name: name, Args: args}
	b, err := Dispatch(a.dbName, "openTransaction", a.marshal(req))
	a.assert.NoError(err)
	var res openTransactionResponse
	a.unmarshal(b, &res)
	return res
}

func (a api) commitTransaction(txID int) commitTransactionResponse {
	req := commitTransactionRequest{TransactionID: txID}
	b, err := Dispatch(a.dbName, "commitTransaction", a.marshal(req))
	a.assert.NoError(err)
	var res commitTransactionResponse
	a.unmarshal(b, &res)
	return res
}

func (a api) closeTransaction(txID int) {
	req := closeTransactionRequest{TransactionID: txID}
	_, err := Dispatch(a.dbName, "closeTransaction", a.marshal(req))
	a.assert.NoError(err)
	return
}

func (a api) beginSync(batchPushURL, diffServerURL, dataLayerAuth string) (beginSyncResponse, error) {
	req := beginSyncRequest{batchPushURL, diffServerURL, dataLayerAuth}
	b, err := Dispatch(a.dbName, "beginSync", a.marshal(req))
	if err != nil {
		return beginSyncResponse{}, err
	}
	var res beginSyncResponse
	a.unmarshal(b, &res)
	return res, nil
}

func (a api) maybeEndSync(syncHead *jsnoms.Hash) maybeEndSyncResponse {
	req := maybeEndSyncRequest{syncHead}
	b, err := Dispatch(a.dbName, "maybeEndSync", a.marshal(req))
	a.assert.NoError(err)
	a.assert.NotNil(b)
	var res maybeEndSyncResponse
	a.unmarshal(b, &res)
	return res
}

func (a api) marshal(v interface{}) []byte {
	m, err := json.Marshal(v)
	a.assert.NoError(err)
	return m
}

func (a api) unmarshal(m []byte, v interface{}) {
	a.assert.NoError(json.Unmarshal(m, v))
	return
}

func (a api) clientID() string {
	return connections[a.dbName].db.ClientID()
}

func TestNopRoundTrip(t *testing.T) {
	assert := assert.New(t)
	env := newTestEnv(assert)
	defer env.teardown()
	api := env.api
	authToken := "opensaysme"
	env.dataLayer.setAuthToken(api.clientID(), authToken)

	getRootResponse := api.getRoot()
	head := getRootResponse.Root.Hash

	// We expect a new snapshot on the first sync because there is no snapshot on master
	// and thus no server state id.
	beingSyncResponse, err := api.beginSync(env.batchPushURL, env.diffServerURL, authToken)
	assert.NoError(err)
	api.maybeEndSync(&beingSyncResponse.SyncHead)
	getRootResponse = api.getRoot()
	secondHead := getRootResponse.Root.Hash
	assert.NotEqual(head, secondHead)

	// We do not expect a new snapshot on subsequent syncs.
	_, err = api.beginSync(env.batchPushURL, env.diffServerURL, authToken)
	assert.Error(err)
	getRootResponse = api.getRoot()
	thirdHead := getRootResponse.Root.Hash
	assert.Equal(secondHead, thirdHead)
}

func TestRoundTrip(t *testing.T) {
	assert := assert.New(t)
	env := newTestEnv(assert)
	defer env.teardown()
	api := env.api
	authToken := "opensaysme"
	env.dataLayer.setAuthToken(api.clientID(), authToken)

	myPut(api, "key", []byte("true"))
	getRootResponse := api.getRoot()
	head := getRootResponse.Root.Hash
	beingSyncResponse, err := api.beginSync(env.batchPushURL, env.diffServerURL, authToken)
	assert.NoError(err)
	api.maybeEndSync(&beingSyncResponse.SyncHead)
	getResponse := api.get("key")
	assert.True(getResponse.Has)
	assert.Equal(json.RawMessage([]byte("true")), getResponse.Value)
	getRootResponse = api.getRoot()
	newHead := getRootResponse.Root.Hash
	assert.NotEqual(head, newHead)
}

func TestPull(t *testing.T) {
	assert := assert.New(t)
	env := newTestEnv(assert)
	defer env.teardown()
	api := env.api
	authToken := "opensaysme"
	env.dataLayer.setAuthToken(api.clientID(), authToken)

	getResponse := api.get("key")
	assert.False(getResponse.Has)
	env.dataLayer.change(api.clientID(), "key", []byte("true"))
	beginSyncResponse, err := api.beginSync(env.batchPushURL, env.diffServerURL, authToken)
	assert.NoError(err)
	api.maybeEndSync(&beginSyncResponse.SyncHead)
	getResponse = api.get("key")
	assert.True(getResponse.Has)
	assert.Equal(json.RawMessage([]byte("true")), getResponse.Value)
}

// TODO: test replay, concurrent syncs.
