// +build js,wasm

package idb

import (
	"encoding/base32"
	"fmt"
	"sync"
	"syscall/js"
)

const (
	objectStore = "chunks"
)

var (
	evalOnce = sync.Once{}
	openIDB  js.Value
	idb      js.Value
)

func eval(code string) js.Value {
	return js.Global().Call("eval", code)
}

func setup() {
	idb = eval(idbCode)
	openIDB = eval(`
(idb, dbName) => 
	idb.openDB(dbName, 1, {
		upgrade(db) {
		    db.createObjectStore("chunks");
		},
	});
`)
}

type indexedDB struct {
	db js.Value
}

func has(v js.Value, store string, key []byte) (bool, error) {
	var call js.Value
	if store == "" {
		call = v.Call("count", string(key))
	} else {
		call = v.Call("count", store, string(key))
	}
	res, err := await(call)
	if err != nil {
		return false, err
	}
	return res.Int() != 0, nil
}

func get(v js.Value, store string, key []byte) ([]byte, error) {
	var call js.Value
	if store == "" {
		call = v.Call("get", string(key))
	} else {
		call = v.Call("get", store, string(key))
	}
	res, err := await(call)
	if err != nil {
		return nil, err
	}
	if res.IsUndefined() {
		// NOCOMMIT: Should this return an error?
		return nil, nil
	}
	dst := make([]byte, res.Length())
	n := js.CopyBytesToGo(dst, res)
	if n == 0 {
		fmt.Printf("copied %d, want %d\n", n, 100) // NOCOMMIT
	}

	// NOCOMMIT: How should this type transformation happen?
	return dst, nil
}

func NewIndexedDB(name string) (IndexedDB, error) {
	evalOnce.Do(setup)

	res, err := await(openIDB.Invoke(idb, name))
	if err != nil {
		return nil, err
	}
	return &indexedDB{res}, nil
}

func (i *indexedDB) Has(key []byte) (bool, error) {
	return has(i.db, objectStore, key)
}

func (i *indexedDB) Get(key []byte) ([]byte, error) {
	return get(i.db, objectStore, key)
}

func (i *indexedDB) Close() error {
	// TODO(nate): Implement.
	return nil
}

type keyValue struct {
	key,
	value []byte
}

type transaction struct {
	db    js.Value // NOCOMMIT: Remove me.
	tx    js.Value
	store js.Value
}

func (i *indexedDB) OpenTransaction() (Transaction, error) {
	tx := i.db.Call("transaction", objectStore, "readwrite")
	return &transaction{i.db, tx, tx.Get("store")}, nil
}

func (t *transaction) Has(key []byte) (bool, error) {
	// NOCOMMIT: Need to look at pending.
	return has(t.store, "", key)
}

func (t *transaction) Get(key []byte) ([]byte, error) {
	// NOCOMMIT: Need to look at pending.
	value, err := get(t.store, "", key)
	if err != nil {
		return nil, err
	}
	return value, nil
}

var encoding = base32.NewEncoding("0123456789abcdefghijklmnopqrstuv")

func (t *transaction) Put(key, value []byte) error {
	// Avoid type coersion here.
	dst := js.Global().Get("Uint8Array").New(len(value))
	if n := js.CopyBytesToJS(dst, value); n != len(value) {
		return fmt.Errorf("copied %d, wanted %d", n, len(value))
	}
	_, err := await(t.store.Call("put", dst, string(key)))
	return err
}

func (t *transaction) Commit() error {
	_, err := await(t.tx.Get("done"))
	return err
}

func await(v js.Value) (js.Value, error) {
	done := make(chan struct{})
	var r struct {
		res js.Value
		err error
	}
	var then, catch js.Func
	then = js.FuncOf(func(this js.Value, v []js.Value) interface{} {
		then.Release()
		catch.Release()
		r.res = v[0]
		close(done)
		return nil
	})
	catch = js.FuncOf(func(this js.Value, v []js.Value) interface{} {
		then.Release()
		catch.Release()
		var e js.Value
		if len(v) != 0 {
			e = v[0]
		}
		if e.IsUndefined() {
			e = js.Value{}
		}
		r.err = js.Error{e}
		close(done)
		return nil
	})
	v.Call("then", then).Call("catch", catch)
	<-done
	return r.res, r.err
}

var idbCode = `
(function(e){"use strict";let t,n;const r=new WeakMap,o=new WeakMap,s=new WeakMap,a=new WeakMap,i=new WeakMap;let c={get(e,t,n){if(e instanceof IDBTransaction){if("done"===t)return o.get(e);if("objectStoreNames"===t)return e.objectStoreNames||s.get(e);if("store"===t)return n.objectStoreNames[1]?void 0:n.objectStore(n.objectStoreNames[0])}return p(e[t])},set:(e,t,n)=>(e[t]=n,!0),has:(e,t)=>e instanceof IDBTransaction&&("done"===t||"store"===t)||t in e};function u(e){return e!==IDBDatabase.prototype.transaction||"objectStoreNames"in IDBTransaction.prototype?(n||(n=[IDBCursor.prototype.advance,IDBCursor.prototype.continue,IDBCursor.prototype.continuePrimaryKey])).includes(e)?function(...t){return e.apply(f(this),t),p(r.get(this))}:function(...t){return p(e.apply(f(this),t))}:function(t,...n){const r=e.call(f(this),t,...n);return s.set(r,t.sort?t.sort():[t]),p(r)}}function d(e){return"function"==typeof e?u(e):(e instanceof IDBTransaction&&function(e){if(o.has(e))return;const t=new Promise((t,n)=>{const r=()=>{e.removeEventListener("complete",o),e.removeEventListener("error",s),e.removeEventListener("abort",s)},o=()=>{t(),r()},s=()=>{n(e.error||new DOMException("AbortError","AbortError")),r()};e.addEventListener("complete",o),e.addEventListener("error",s),e.addEventListener("abort",s)});o.set(e,t)}(e),n=e,(t||(t=[IDBDatabase,IDBObjectStore,IDBIndex,IDBCursor,IDBTransaction])).some(e=>n instanceof e)?new Proxy(e,c):e);var n}function p(e){if(e instanceof IDBRequest)return function(e){const t=new Promise((t,n)=>{const r=()=>{e.removeEventListener("success",o),e.removeEventListener("error",s)},o=()=>{t(p(e.result)),r()},s=()=>{n(e.error),r()};e.addEventListener("success",o),e.addEventListener("error",s)});return t.then(t=>{t instanceof IDBCursor&&r.set(t,e)}).catch(()=>{}),i.set(t,e),t}(e);if(a.has(e))return a.get(e);const t=d(e);return t!==e&&(a.set(e,t),i.set(t,e)),t}const f=e=>i.get(e);const l=["get","getKey","getAll","getAllKeys","count"],D=["put","add","delete","clear"],v=new Map;function b(e,t){if(!(e instanceof IDBDatabase)||t in e||"string"!=typeof t)return;if(v.get(t))return v.get(t);const n=t.replace(/FromIndex$/,""),r=t!==n,o=D.includes(n);if(!(n in(r?IDBIndex:IDBObjectStore).prototype)||!o&&!l.includes(n))return;const s=async function(e,...t){const s=this.transaction(e,o?"readwrite":"readonly");let a=s.store;r&&(a=a.index(t.shift()));const i=a[n](...t);return o&&await s.done,i};return v.set(t,s),s}return c=(e=>({...e,get:(t,n,r)=>b(t,n)||e.get(t,n,r),has:(t,n)=>!!b(t,n)||e.has(t,n)}))(c),e.deleteDB=function(e,{blocked:t}={}){const n=indexedDB.deleteDatabase(e);return t&&n.addEventListener("blocked",()=>t()),p(n).then(()=>{})},e.openDB=function(e,t,{blocked:n,upgrade:r,blocking:o,terminated:s}={}){const a=indexedDB.open(e,t),i=p(a);return r&&a.addEventListener("upgradeneeded",e=>{r(p(a.result),e.oldVersion,e.newVersion,p(a.transaction))}),n&&a.addEventListener("blocked",()=>n()),i.then(e=>{s&&e.addEventListener("close",()=>s()),o&&e.addEventListener("versionchange",()=>o())}).catch(()=>{}),i},e.unwrap=f,e.wrap=p,e}({}))`
