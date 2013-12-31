package server

import (
	"encoding/json"
	"github.com/goraft/raft"
	"github.com/gorilla/mux"
	"github.com/skynetservices/skydns/msg"
	"github.com/skynetservices/skydns/registry"
	"log"
	"net/http"
)

// Handle API add callback requests.
func (s *Server) addCallbackHTTPHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	var uuid string
	var ok bool
	var secret string

	secret = req.Header.Get("Authorization")
	if err := s.authenticate(secret); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if uuid, ok = vars["uuid"]; !ok {
		http.Error(w, "UUID required", http.StatusBadRequest)
		return
	}

	var cb msg.Callback

	if err := json.NewDecoder(req.Body).Decode(&cb); err != nil {
		log.Println("Error: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cb.UUID = uuid
	// Lookup the service(s)
	// TODO: getRegistryKey(s) isn't exported, or what should I (miek)
	//  use to go from string to a domain.
	// TODO: version is thus not correctly formatted
	key := cb.Name + "." + cb.Version + "." + cb.Environment + "." + cb.Region +
		"." + cb.Host
	services, err := s.registry.Get(key)
	if err != nil || len(services) == 0 {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	// Reset to save memory.
	cb.Name = ""
	cb.Version = ""
	cb.Environment = ""
	cb.Region = ""
	cb.Host = ""

	notExists := 0
	for _, serv := range services {
		if _, err := s.raftServer.Do(NewAddCallbackCommand(serv, cb)); err != nil {
			switch err {
			case registry.ErrNotExists:
				notExists++
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
	if notExists == len(services) - 1 {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusCreated)
}
