package main

import (
	"flag"
	"github.com/goraft/raft"
	"log"
	"net"
	"strings"
	"time"
)

var (
	join, ldns, lhttp, dataDir, domain string
	rtimeout, wtimeout                 time.Duration
	discover                           bool
)

func init() {
	flag.StringVar(&join, "join", "", "Member of SkyDNS cluster to join can be comma separated list")
	flag.BoolVar(&discover, "discover", false, "Auto discover SkyDNS cluster. Performs an NS lookup on the -domain to find SkyDNS members")
	flag.StringVar(&domain, "domain", "skydns.local", "Domain to anchor requests to")
	flag.StringVar(&ldns, "dns", "127.0.0.1:53", "IP:Port to bind to for DNS")
	flag.StringVar(&lhttp, "http", "127.0.0.1:8080", "IP:Port to bind to for HTTP")
	flag.StringVar(&dataDir, "data", "./data", "SkyDNS data directory")
	flag.DurationVar(&rtimeout, "rtimeout", 2*time.Second, "Read timeout")
	flag.DurationVar(&wtimeout, "wtimeout", 2*time.Second, "Write timeout")
}

func main() {
	members := make([]string, 0)

	raft.SetLogLevel(0)

	flag.Parse()

	if discover {
		ns, err := net.LookupNS(domain)

		if err != nil {
			log.Fatal(err)
			return
		}

		if len(ns) < 1 {
			log.Fatal("No NS records found for ", domain)
			return
		}

		for _, n := range ns {
			members = append(members, n.Host)
		}
	} else if join != "" {
		members = strings.Split(join, ",")
	}

	s := NewServer(members, domain, ldns, lhttp, dataDir, rtimeout, wtimeout)

	waiter := s.Start()
	waiter.Wait()
}
