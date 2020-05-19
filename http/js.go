// +build js,wasm

package http

import (
	"bytes"
	"fmt"
	"sync"
	"syscall/js"
	"time"
)

var (
	evalOnce = sync.Once{}
)

var code = `
var replicache = {};

replicache.fetch = function(method, url, body, auth, callback) {
    var response;
	var init = {"method": method};
	if (auth != "") {
	    init["headers"] = {"Authorization": auth}
	}
	if (body !== undefined) {
	    init["body"] = body
	}
	fetch(url, init
	).then(resp => {
		response = resp;
		return resp.text();
	}
	).then(data => {
		callback(undefined, response, data);
	}).catch(err => {
		callback(err.toString(), undefined, undefined);
	});
}
`

func setup() {
	js.Global().Call("eval", code)
}

// TODO(nate): Implement enforcement of timeout.
func impl(method, url string, body []byte, timeout time.Duration, auth string) (*Response, error) {
	evalOnce.Do(setup)

	c := make(chan bool)
	var jsBody = js.Undefined()
	if len(body) != 0 {
		jsBody = js.Global().Get("Uint8Array").New(len(body))
		if n := js.CopyBytesToJS(jsBody, body); n != len(body) {
			return nil, fmt.Errorf("copied %d, wanted %d", n, len(body))
		}
	}

	var jsErr, resp, text js.Value
	js.Global().Get("replicache").Call(
		"fetch", method, url, jsBody, auth,
		js.FuncOf(func(this js.Value, inputs []js.Value) interface{} {
			jsErr = inputs[0]
			resp = inputs[1]
			text = inputs[2]
			c <- true

			return nil
		}))
	<-c

	if !jsErr.IsUndefined() {
		return nil, fmt.Errorf("Failed POST to %s: %s", url, jsErr.String())
	}

	if !resp.Get("ok").Bool() {
		return &Response{
			Status:     resp.Get("statusText").String(),
			StatusCode: resp.Get("status").Int(),
			Body:       bytes.NewReader([]byte(text.String())),
		}, nil
	}

	return &Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       bytes.NewReader([]byte(text.String())),
	}, nil
}
