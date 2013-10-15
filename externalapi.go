package main

import (
	"bytes"
	"encoding/json"
	"github.com/skynetservices/skydns/registry"
	"log"
	"net/http"
)

func (s *Server) getServicesHTTPHandler(w http.ResponseWriter, req *http.Request) {
	log.Println(req.URL.Path)
	log.Println(s.raftServer.Leader())

	var q string

	if q = req.URL.Query().Get("query"); q == "" {
		q = "any"
	}

	log.Println("Retrieving All Services for query", q)

	srv, err := s.registry.Get(q)

	if err != nil {
		switch err {
		case registry.ErrNotExists:
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			log.Println("Error: ", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		return
	}

	var b bytes.Buffer
	json.NewEncoder(&b).Encode(srv)
	w.Write(b.Bytes())

}
