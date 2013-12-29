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
	addServiceCount.Inc(1)
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

	var serv msg.Service

	if err := json.NewDecoder(req.Body).Decode(&serv); err != nil {
		log.Println("Error: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	serv.UUID = uuid

	if _, err := s.raftServer.Do(NewAddServiceCommand(serv)); err != nil {
		switch err {
		case registry.ErrExists:
			http.Error(w, err.Error(), http.StatusConflict)
		case raft.NotLeaderError:
			s.redirectToLeader(w, req)
		default:
			log.Println("Error: ", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		return
	}

	w.WriteHeader(http.StatusCreated)
}
