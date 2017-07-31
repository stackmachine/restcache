package cas

import (
	"fmt"
	"io"
	"net/http"

	"stackmachine.com/blobstore"

	"goji.io"
	"goji.io/pat"
)

type CAS struct {
	AccessKey string
	SecretKey string

	store   blobstore.Client
	handler http.Handler
}

func NewServer(store blobstore.Client) *CAS {
	c := CAS{store: store}
	mux := goji.NewMux()

	auth := func(next http.HandlerFunc) http.HandlerFunc {
		// TODO: Use basic authentication once Bazel support lands
		return func(w http.ResponseWriter, r *http.Request) {
			user := pat.Param(r, "user")
			pass := pat.Param(r, "pass")
			if user != c.AccessKey || pass != c.SecretKey {
				http.Error(w, "Unauthorized.", 401)
				return
			}
			next(w, r)
		}
	}

	mux.HandleFunc(pat.Get("/:user/:pass/:key"), auth(c.Get))
	mux.HandleFunc(pat.Put("/:user/:pass/:key"), auth(c.Put))
	mux.Handle(pat.Get("/*"), http.NotFoundHandler())
	c.handler = mux
	return &c
}

func (c *CAS) Get(w http.ResponseWriter, r *http.Request) {
	key := pat.Param(r, "key")
	if key == "" {
		http.Error(w, fmt.Sprintf("No key provided"), http.StatusBadRequest)
		return
	}
	exists, err := c.store.Contains(key)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error talking to blobstore: %s", err), http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, fmt.Sprintf("Key %s does not exist", key), http.StatusNotFound)
		return
	}
	reader, n, err := c.store.Get(key)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error talking to blobstore: %s", err), http.StatusInternalServerError)
		return
	}
	if _, err := io.CopyN(w, reader, n); err != nil {
		http.Error(w, fmt.Sprintf("Error writing out response: %s", err), http.StatusInternalServerError)
		return
	}
}

func (c *CAS) Put(w http.ResponseWriter, r *http.Request) {
	key := pat.Param(r, "key")
	if key == "" {
		http.Error(w, fmt.Sprintf("No key provided"), http.StatusBadRequest)
		return
	}
	if r.Body == nil {
		http.Error(w, fmt.Sprintf("No body provided"), http.StatusBadRequest)
		return
	}
	err := c.store.Put(key, r.Body, r.ContentLength)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error talking to blobstore: %s", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *CAS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.handler.ServeHTTP(w, r)
}
