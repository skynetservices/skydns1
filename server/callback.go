package server

import (
	//	"bytes"
	"encoding/json"
	//	"errors"
	//	"fmt"
	"github.com/goraft/raft"
	"github.com/gorilla/mux"
	//	"github.com/miekg/dns"
	//	"github.com/rcrowley/go-metrics"
	"github.com/skynetservices/skydns/msg"
	"github.com/skynetservices/skydns/registry"
	"log"
	//	"math"
	//	"net"
	"net/http"
	//	"net/url"
	//	"os"
	//	"os/signal"
	//	"strings"
	//	"sync"
	//	"time"
)

// Handle API add callback requests
func (s *Server) addCallbackHTTPHandler(w http.ResponseWriter, req *http.Request) {
	//	addServiceCount.Inc(1)
	vars := mux.Vars(req)

	var uuid string
	var ok bool
	var secret string

	//read the authorization header to get the secret.
	secret = req.Header.Get("Authorization")

	if err := s.authenticate(secret); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if uuid, ok = vars["uuid"]; !ok {
		http.Error(w, "UUID required", http.StatusBadRequest)
		return
	}

	// find service

	var cb msg.Callback

	if err := json.NewDecoder(req.Body).Decode(&cb); err != nil {
		log.Println("Error: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cb.UUID = uuid
	// We don't care about the other values once we have the 
	// key, set them to zero to save some memory.

	// Lookup the service(s)
	// TODO: getRegistryKey(s) isn't exported.
	// TODO: version is thus not correctly formatted
	key := cb.Name + "." + cb.Version + "." + cb.Environment + "." + cb.Region +
		"." + cb.Host
	services, err := s.registry.Get(key)
	if err != nil || len(services) == 0 {
		http.Error(w, err.Error(), http.StatusNotFound)
	}
	cb.Name = ""
	cb.Version = ""
	cb.Environment = ""
	cb.Region = ""
	cb.Host = ""

	for _, serv := range services {
		if _, err := s.raftServer.Do(NewAddCallbackCommand(serv, cb)); err != nil {
			switch err {
			case registry.ErrNotExists:
				http.Error(w, err.Error(), http.StatusNotFound)
				// Don't return here, other services might exist
				// TODO(miek): set error in var and check afterwards?
			case raft.NotLeaderError:
				s.redirectToLeader(w, req)
				return
			default:
				log.Println("Error: ", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}
	w.WriteHeader(http.StatusCreated)
}
