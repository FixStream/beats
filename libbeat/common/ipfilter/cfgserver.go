package ipfilter

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/boltdb/bolt"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/gorilla/mux"
	"github.com/jchannon/negotiator"
)

// BucketName export to use at publish
const BucketName = "ipFilters"

// RuleDB DB Connection exported to re-use at publish
var roleDB *bolt.DB

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

type ipRule struct {
	IP     string `json:"ip"`
	IsDrop bool   `json:"is_done"`
}

// A (simple) example of our application-wide configuration.
type Env struct {
	DB *bolt.DB
}

// The Handler struct that takes a configured Env and a function matching
// our useful signature.
type Handler struct {
	*Env
	H func(e *Env, w http.ResponseWriter, r *http.Request) error
}

// ServeHTTP allows our Handler type to satisfy http.Handler.
func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := h.H(h.Env, w, r)
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
func InitDB(path string) *bolt.DB {
	// logp.Info("boltdb starting====================")
	var (
		err error
	)
	roleDB, err = bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		logp.Err("fails to read the db: %v", err)
		return nil
	}
	roleDB.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(BucketName))
		if err != nil {
			logp.Err("create bucket: %v", err)
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	logp.Info("boltdb started")
	// defer roleDB.Close()
	return roleDB
}

func bulkRule(env *Env, w http.ResponseWriter, r *http.Request) error {
	var (
		irs         []ipRule
		jsonEncoded []byte
	)
	if r.Body == nil {
		return StatusError{400, errors.New("Please send a request body")}
	}
	if err := json.NewDecoder(r.Body).Decode(&irs); err != nil {
		return StatusError{400, err}
	}
	err := env.DB.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(BucketName))
		if err != nil {
			return err
		}
		for _, ir := range irs {
			jsonEncoded, err = json.Marshal(ir)
			if err != nil {
				return err
			}
			bucket.Put([]byte(ir.IP), jsonEncoded)
		}
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
	if err = negotiator.Negotiate(w, r, irs); err != nil {
		return err
	}
	return nil
}

func createRule(env *Env, w http.ResponseWriter, r *http.Request) error {
	var jsonEncoded []byte
	var ir ipRule
	if r.Body == nil {
		return StatusError{400, errors.New("Please send a request body")}
	}
	if err := json.NewDecoder(r.Body).Decode(&ir); err != nil {
		return StatusError{400, err}
	}
	err := env.DB.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(BucketName))
		if err != nil {
			return err
		}
		jsonEncoded, err = json.Marshal(ir)
		if err != nil {
			return err
		}
		bucket.Put([]byte(ir.IP), jsonEncoded)
		return nil
	})
	if err != nil {
		return err
	}
	if err = negotiator.Negotiate(w, r, ir); err != nil {
		return err
	}
	return nil
}

func deleteRule(env *Env, w http.ResponseWriter, r *http.Request) error {
	pathVars := mux.Vars(r)
	ip, ok := pathVars["ip"]
	if !ok {
		return StatusError{http.StatusBadRequest, errors.New("Rule ip missing")}
	}
	err := env.DB.Update(func(tx *bolt.Tx) error {
		if bucket := tx.Bucket([]byte(BucketName)); bucket != nil {
			bucket.Delete([]byte(ip))
		} else {
			return errors.New("Ip doesn't exists on DB")
		}

		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
	msg := fmt.Sprintf("Successfully deletd ip rule %s", ip)
	if err = negotiator.Negotiate(w, r, msg); err != nil {
		return err
	}
	return nil
}

func dbBackup(env *Env, w http.ResponseWriter, r *http.Request) (err error) {
	err = env.DB.View(func(tx *bolt.Tx) error {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="my.db"`)
		w.Header().Set("Content-Length", strconv.Itoa(int(tx.Size())))
		_, err = tx.WriteTo(w)
		return err
	})
	if err != nil {
		return StatusError{http.StatusInternalServerError, err}
	}
	return
}

func getRules(env *Env, w http.ResponseWriter, r *http.Request) (err error) {
	var datas = make([]map[string]string, 0)
	var m = make(map[string]string)
	err = env.DB.View(func(tx *bolt.Tx) error {
		if bucket := tx.Bucket([]byte(BucketName)); bucket != nil {
			c := bucket.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				m = map[string]string{string(k): string(v)}
				datas = append(datas, m)
			}
		}
		return nil
	})
	if err != nil {
		return
	}
	if err = negotiator.Negotiate(w, r, datas); err != nil {
		return err
	}
	return nil
}

// GetHTTPRoute Prepares http routes
// connect to boltdb
func GetHTTPRoute() (router *mux.Router, err error) {
	env := &Env{roleDB}
	router = mux.NewRouter().StrictSlash(true)
	router.Handle("/rules/ip/", Handler{env, getRules}).Methods("GET")
	router.Handle("/rules/ip/", Handler{env, createRule}).Methods("POST")
	router.Handle("/rules/ip/", Handler{env, bulkRule}).Methods("PATCH")
	router.Handle("/rules/ip/{ip}/", Handler{env, deleteRule}).Methods("DELETE")
	router.Handle("/rules/db/backup/", Handler{env, dbBackup}).Methods("GET")
	return
}
