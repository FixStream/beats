package ipfilter

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/elastic/beats/libbeat/logp"
	"github.com/gorilla/mux"
	"github.com/jchannon/negotiator"
)

// BucketName export to use at publish
const BucketName = "ipFilters"

// RuleDB DB Connection exported to re-use at publish
var roleDB *DBorm

// Error represents a handler error. It provides methods for a HTTP status
// code and embeds the built-in error interface.
type Error interface {
	error
	Status() int
}

// StatusError represents an error with an associated HTTP status code.
type StatusError struct {
	Code int
	Err  error
}

// Allows StatusError to satisfy the error interface.
func (se StatusError) Error() string {
	return se.Err.Error()
}

// Returns our HTTP status code.
func (se StatusError) Status() int {
	return se.Code
}

type IpRule struct {
	IP     string `json:"ip"`
	IsDrop bool   `json:"is_done"`
	OrgID  string `json:"org_id"`
}

// A (simple) example of our application-wide configuration.
type env struct {
	orm *DBorm
}

// The Handler struct that takes a configured Env and a function matching
// our useful signature.
type Handler struct {
	*env
	H func(e *env, w http.ResponseWriter, r *http.Request) error
}

// ServeHTTP allows our Handler type to satisfy http.Handler.
func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := h.H(h.env, w, r)
	if err != nil {
		switch e := err.(type) {
		case Error:
			// We can retrieve the status here and write out a specific
			// HTTP status code.
			logp.Err("HTTP %d - %s", e.Status(), e)
			http.Error(w, e.Error(), e.Status())
		default:
			// Any error types we don't specifically look out for default
			// to serving a HTTP 500
			http.Error(w, http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError)
		}
	}
}

// InitDB to initilize the db to store and retrive
// ip filter information
func InitDB(path string) *DBorm {
	// logp.Info("boltdb starting====================")

	var (
		err error
	)
	// roleDB, err = bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	roleDB, err = NewDBORM(path)
	if err != nil {
		logp.Err("fails to read the db: %v", err)
		return nil
	}
	logp.Info("boltdb started")
	// defer roleDB.Close()
	return roleDB
}

func bulkRule(e *env, w http.ResponseWriter, r *http.Request) error {
	var (
		irs         []IpRule
		jsonEncoded []byte
		err         error
	)
	if r.Body == nil {
		return StatusError{400, errors.New("Please send a request body")}
	}
	if err = json.NewDecoder(r.Body).Decode(&irs); err != nil {
		return StatusError{400, err}
	}
	for _, ir := range irs {
		if jsonEncoded, err = json.Marshal(ir); err == nil {
			e.orm.Put(BucketName, ir.IP, jsonEncoded)
		}
	}
	if err = negotiator.Negotiate(w, r, irs); err != nil {
		return err
	}
	return nil
}

func createRule(e *env, w http.ResponseWriter, r *http.Request) error {
	var (
		ir IpRule
	)
	if r.Body == nil {
		return StatusError{400, errors.New("Please send a request body")}
	}
	if err := json.NewDecoder(r.Body).Decode(&ir); err != nil {
		return StatusError{400, err}
	}
	if jsonEncoded, err := json.Marshal(ir); err == nil {
		e.orm.Put(BucketName, ir.IP, jsonEncoded)
	} else {
		return err
	}
	if err := negotiator.Negotiate(w, r, ir); err != nil {
		return err
	}
	return nil
}

func deleteRule(e *env, w http.ResponseWriter, r *http.Request) error {
	pathVars := mux.Vars(r)
	ip, ok := pathVars["ip"]
	if !ok {
		return StatusError{http.StatusBadRequest, errors.New("Rule ip missing")}
	}
	err := e.orm.Delete(BucketName, ip)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
	msg := fmt.Sprintf("Successfully deletd ip rule %s", ip)
	if err = negotiator.Negotiate(w, r, msg); err != nil {
		return err
	}
	return nil
}

func getRules(e *env, w http.ResponseWriter, r *http.Request) (err error) {
	if datas, err := e.orm.GetAll(BucketName); err == nil {
		if err = negotiator.Negotiate(w, r, datas); err != nil {
			return err
		}
	}
	return nil
}

// GetHTTPRoute Prepares http routes
// connect to boltdb
func GetHTTPRoute() (router *mux.Router, err error) {
	env := &env{roleDB}
	router = mux.NewRouter().StrictSlash(true)
	router.Handle("/rules/ip/", Handler{env, getRules}).Methods("GET")
	router.Handle("/rules/ip/", Handler{env, createRule}).Methods("POST")
	router.Handle("/rules/ip/", Handler{env, bulkRule}).Methods("PATCH")
	router.Handle("/rules/ip/{ip}/", Handler{env, deleteRule}).Methods("DELETE")
	return
}
