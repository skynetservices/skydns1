package main

import (
	"flag"
	"github.com/goraft/raft"
	"time"
)

var (
	leader, ldns, lhttp, dataDir, domain string
	rtimeout, wtimeout                   time.Duration
)

func init() {
	flag.StringVar(&leader, "leader", "", "SkyDNS Leader")
	flag.StringVar(&domain, "domain", "skydns.local", "Domain to anchor requests to")
	flag.StringVar(&ldns, "dns", "127.0.0.1:53", "IP:Port to bind to for DNS")
	flag.StringVar(&lhttp, "http", "127.0.0.1:8080", "IP:Port to bind to for HTTP")
	flag.StringVar(&dataDir, "data", "./data", "SkyDNS data directory")
	flag.DurationVar(&rtimeout, "rtimeout", 2*time.Second, "Read timeout")
	flag.DurationVar(&wtimeout, "wtimeout", 2*time.Second, "Write timeout")
}

func main() {
	raft.SetLogLevel(0)

	flag.Parse()
	s := NewServer(leader, domain, ldns, lhttp, dataDir, rtimeout, wtimeout)

	waiter := s.Start()
	waiter.Wait()
}
