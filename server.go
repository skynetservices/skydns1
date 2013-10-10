package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/goraft/raft"
	"github.com/gorilla/mux"
	"github.com/miekg/dns"
	"github.com/skynetservices/skydns/msg"
	"github.com/skynetservices/skydns/registry"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"
)

/* TODO:
   Set Priority based on Region
   Dynamically set Weight/Priority in DNS responses
   Handle API call for setting host statistics
   Handle Errors in DNS
   Master should cleanup expired services
   TTL cleanup thread should shutdown/start based on being elected master
*/

func init() {
	// Register Raft Commands
	raft.RegisterCommand(&AddServiceCommand{})
	raft.RegisterCommand(&UpdateTTLCommand{})
	raft.RegisterCommand(&RemoveServiceCommand{})
}

type Server struct {
	leader       string
	host         string
	dnsPort      int
	httpPort     int
	readTimeout  time.Duration
	writeTimeout time.Duration
	waiter       *sync.WaitGroup

	registry registry.Registry

	dnsUDPServer *dns.Server
	dnsTCPServer *dns.Server
	dnsHandler   *dns.ServeMux

	httpServer *http.Server
	router     *mux.Router

	raftServer *raft.Server
	dataDir    string
}

// Create a new Server
func NewServer(leader string, host string, dnsPort int, httpPort int, dataDir string, rt, wt time.Duration) (s *Server) {
	s = &Server{
		leader:       leader,
		host:         host,
		dnsPort:      dnsPort,
		httpPort:     httpPort,
		readTimeout:  rt,
		writeTimeout: wt,
		router:       mux.NewRouter(),
		registry:     registry.New(),
		dataDir:      dataDir,
		dnsHandler:   dns.NewServeMux(),
		waiter:       new(sync.WaitGroup),
	}

	if _, err := os.Stat(s.dataDir); os.IsNotExist(err) {
		log.Fatal("Data directory does not exist: ", dataDir)
		return
	}

	// DNS
	s.dnsHandler.Handle(".", s)

	// API Routes
	s.router.HandleFunc("/skydns/services", s.addServiceHTTPHandler).Methods("POST")
	s.router.HandleFunc("/skydns/services/{uuid}", s.getServiceHTTPHandler).Methods("GET")
	s.router.HandleFunc("/skydns/services/{uuid}", s.removeServiceHTTPHandler).Methods("DELETE")
	s.router.HandleFunc("/skydns/services/{uuid}", s.updateServiceHTTPHandler).Methods("PUT")

	// Raft Routes
	s.router.HandleFunc("/raft/join", s.joinHandler).Methods("POST")

	return
}

// Returns IP:Port of DNS Server
func (s *Server) DNSAddr() string {
	return s.host + ":" + strconv.Itoa(s.dnsPort)
}

// Returns IP:Port of HTTP Server
func (s *Server) HTTPAddr() string {
	return s.host + ":" + strconv.Itoa(s.httpPort)
}

// Starts DNS server and blocks waiting to be killed
func (s *Server) Start() *sync.WaitGroup {
	var err error
	log.Printf("Initializing Raft Server: %s", s.dataDir)

	// Initialize and start Raft server.
	transporter := raft.NewHTTPTransporter("/raft")
	s.raftServer, err = raft.NewServer(s.HTTPAddr(), s.dataDir, transporter, nil, s.registry, "")
	if err != nil {
		log.Fatal(err)
	}
	transporter.Install(s.raftServer, s)
	s.raftServer.Start()

	// Join to leader if specified.
	if s.leader != "" {
		log.Println("Attempting to join leader:", s.leader)

		if !s.raftServer.IsLogEmpty() {
			log.Fatal("Cannot join with an existing log")
		}
		if err := s.Join(s.leader); err != nil {
			log.Fatal(err)
		}

		log.Println("Joined leader:", s.leader)

		// Initialize the server by joining itself.
	} else if s.raftServer.IsLogEmpty() {
		log.Println("Initializing new cluster")

		_, err := s.raftServer.Do(&raft.DefaultJoinCommand{
			Name:             s.raftServer.Name(),
			ConnectionString: s.connectionString(),
		})

		if err != nil {
			log.Fatal(err)
		}

	} else {
		log.Println("Recovered from log")
	}

	s.dnsTCPServer = &dns.Server{
		Addr:         s.DNSAddr(),
		Net:          "tcp",
		Handler:      s.dnsHandler,
		ReadTimeout:  s.readTimeout,
		WriteTimeout: s.writeTimeout,
	}

	s.dnsUDPServer = &dns.Server{
		Addr:         s.DNSAddr(),
		Net:          "udp",
		Handler:      s.dnsHandler,
		UDPSize:      65535,
		ReadTimeout:  s.readTimeout,
		WriteTimeout: s.writeTimeout,
	}

	s.httpServer = &http.Server{
		Addr:           s.HTTPAddr(),
		Handler:        s.router,
		ReadTimeout:    s.readTimeout,
		WriteTimeout:   s.writeTimeout,
		MaxHeaderBytes: 1 << 20,
	}

	go s.listenAndServe()

	s.waiter.Add(1)
	go s.run()

	return s.waiter
}

func (s *Server) Stop() {
	log.Println("Stopping server")
	s.waiter.Done()
}

func (s *Server) run() {
	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt)

	tick := time.Tick(1 * time.Second)

run:
	for {
		select {
		case <-tick:
			// We are the leader, we are responsible for managing TTLs
			if s.raftServer.State() == raft.Leader {
				expired := s.registry.GetExpired()

				// TODO: Possible race condition? We could be demoted while iterating
				// probably minimal chance of this happening, this will just cause commands to fail,
				// and new leader will take over anyway
				for _, uuid := range expired {
					s.raftServer.Do(NewRemoveServiceCommand(uuid))
				}
			}
		case <-sig:
			break run
		}
	}

	s.Stop()
}

// Joins an existing skydns cluster
func (s *Server) Join(leader string) error {
	command := &raft.DefaultJoinCommand{
		Name:             s.raftServer.Name(),
		ConnectionString: s.connectionString(),
	}

	var b bytes.Buffer
	json.NewEncoder(&b).Encode(command)
	resp, err := http.Post(fmt.Sprintf("http://%s/raft/join", leader), "application/json", &b)
	if err != nil {
		return err
	}

	resp.Body.Close()

	return nil
}

// Proxy HTTP handlers to Gorilla's mux.Router
func (s *Server) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	s.router.HandleFunc(pattern, handler)
}

// Handles incomming RAFT joins
func (s *Server) joinHandler(w http.ResponseWriter, req *http.Request) {
	command := &raft.DefaultJoinCommand{}

	if err := json.NewDecoder(req.Body).Decode(&command); err != nil {
		log.Println("Error decoding json message:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := s.raftServer.Do(command); err != nil {
		log.Println("Error procssing join:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Handler for DNS requests, responsible for parsing DNS request and returning response
func (s *Server) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(req)
	m.Answer = make([]dns.RR, 0, 10)

	defer w.WriteMsg(m)

	if len(req.Question) < 1 {
		return
	}

	q := req.Question[0]

	var weight uint16

	if q.Qtype == dns.TypeANY || q.Qtype == dns.TypeSRV {
		log.Printf("Received DNS Request for %q from %q", q.Name, w.RemoteAddr())
		services, err := s.registry.Get(q.Name)

		if err != nil {
			log.Println("Error: ", err)
			return
		}

		if len(services) > 0 {
			weight = uint16(math.Floor(float64(100 / len(services))))
		} else {
			weight = 0
		}

		for _, serv := range services {
			// TODO: Dynamically set weight
			m.Answer = append(m.Answer, &dns.SRV{Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeSRV, Class: dns.ClassINET, Ttl: serv.TTL}, Priority: 10, Weight: weight, Port: serv.Port, Target: serv.Host + "."})
		}

		// Append matching entries in different region than requested with a higher priority
		labels := dns.SplitDomainName(q.Name)

		pos := len(labels) - 4
		if len(labels) >= 4 && labels[pos] != "any" && labels[pos] != "all" {
			region := labels[pos]
			labels[pos] = "any"

			// TODO: This is pretty much a copy of the above, and should be abstracted
			additionalServices, err := s.registry.Get(strings.Join(labels, "."))

			if err != nil {
				log.Println("Error: ", err)
				return
			} else {
				if len(additionalServices) > 0 {
					weight = uint16(math.Floor(float64(100 / (len(additionalServices) - len(services)))))
				} else {
					weight = 0
				}

				for _, serv := range additionalServices {
					// Exclude entries we already have
					if strings.ToLower(serv.Region) == region {
						continue
					}

					// TODO: Dynamically set priority and weight
					m.Answer = append(m.Answer, &dns.SRV{Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeSRV, Class: dns.ClassINET, Ttl: serv.TTL}, Priority: 20, Weight: weight, Port: serv.Port, Target: serv.Host + "."})
				}
			}
		}
	}
}

// Returns the connection string.
func (s *Server) connectionString() string {
	return fmt.Sprintf("http://%s:%d", s.host, s.httpPort)
}

// Binds to DNS and HTTP ports and starts accepting connections
func (s *Server) listenAndServe() {
	go func() {
		err := s.dnsTCPServer.ListenAndServe()
		if err != nil {
			log.Fatalf("Start %s listener on %s failed:%s", s.dnsTCPServer.Net, s.dnsTCPServer.Addr, err.Error())
		}
	}()

	go func() {
		err := s.dnsUDPServer.ListenAndServe()
		if err != nil {
			log.Fatalf("Start %s listener on %s failed:%s", s.dnsUDPServer.Net, s.dnsUDPServer.Addr, err.Error())
		}
	}()

	go func() {
		err := s.httpServer.ListenAndServe()
		if err != nil {
			log.Fatalf("Start http listener on %s failed:%s", s.httpServer.Addr, err.Error())
		}
	}()
}

// Handle API add service requests
func (s *Server) addServiceHTTPHandler(w http.ResponseWriter, req *http.Request) {
	var serv msg.Service

	if err := json.NewDecoder(req.Body).Decode(&serv); err != nil {
		log.Println("Error: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := s.raftServer.Do(NewAddServiceCommand(serv)); err != nil {
		switch err {
		case registry.ErrExists:
			http.Error(w, err.Error(), http.StatusConflict)
		case raft.NotLeaderError:
			if s.raftServer.Leader() != "" {
				http.Redirect(w, req, "http://"+s.raftServer.Leader()+req.URL.Path, http.StatusMovedPermanently)
			} else {
				http.Error(w, "Leader unknown", http.StatusInternalServerError)
			}
		default:
			log.Println("Error: ", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		return
	}

	w.WriteHeader(http.StatusCreated)
}

// Handle API remove service requests
func (s *Server) removeServiceHTTPHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	var uuid string
	var ok bool
	if uuid, ok = vars["uuid"]; !ok {
		http.Error(w, "UUID required", http.StatusBadRequest)
		return
	}

	if _, err := s.raftServer.Do(NewRemoveServiceCommand(uuid)); err != nil {

		switch err {
		case registry.ErrNotExists:
			http.Error(w, err.Error(), http.StatusNotFound)
		case raft.NotLeaderError:
			if s.raftServer.Leader() != "" {
				http.Redirect(w, req, "http://"+s.raftServer.Leader()+req.URL.Path, http.StatusMovedPermanently)
			} else {
				http.Error(w, "Leader unknown", http.StatusInternalServerError)
			}
		default:
			log.Println("Error: ", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// Handle API update service requests
func (s *Server) updateServiceHTTPHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	var uuid string
	var ok bool
	if uuid, ok = vars["uuid"]; !ok {
		http.Error(w, "UUID required", http.StatusBadRequest)
		return
	}

	var serv msg.Service
	if err := json.NewDecoder(req.Body).Decode(&serv); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := s.raftServer.Do(NewUpdateTTLCommand(uuid, serv.TTL)); err != nil {
		switch err {
		case registry.ErrNotExists:
			http.Error(w, err.Error(), http.StatusNotFound)
		case raft.NotLeaderError:
			if s.raftServer.Leader() != "" {
				http.Redirect(w, req, "http://"+s.raftServer.Leader()+req.URL.Path, http.StatusMovedPermanently)
			} else {
				http.Error(w, "Leader unknown", http.StatusInternalServerError)
			}
		default:
			log.Println("Error: ", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// Handle API get service requests
func (s *Server) getServiceHTTPHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	log.Println(req.URL.Path)
	log.Println(s.raftServer.Leader())

	var uuid string
	var ok bool
	if uuid, ok = vars["uuid"]; !ok {
		http.Error(w, "UUID required", http.StatusBadRequest)
		return
	}

	log.Println("Retrieving Service ", uuid)
	serv, err := s.registry.GetUUID(uuid)

	if err != nil {
		switch err {
		case registry.ErrNotExists:
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		return
	}

	var b bytes.Buffer
	json.NewEncoder(&b).Encode(serv)
	w.Write(b.Bytes())
}
