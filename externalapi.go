package main

import (
	"bytes"
	"encoding/json"
	"github.com/skynetservices/skydns/registry"
	"log"
	"net/http"
)

// Handle API get services requests
func (s *Server) getServicesHTTPHandler(w http.ResponseWriter, req *http.Request) {
	log.Println(req.URL.Path)
	log.Println(s.raftServer.Leader())

	log.Println("Retrieving All Services")
	serv, err := s.registry.Get("any")

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
	json.NewEncoder(&b).Encode(serv)
	w.Write(b.Bytes())
}
