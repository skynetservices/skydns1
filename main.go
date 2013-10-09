package main

import (
	"flag"
	"github.com/goraft/raft"
	"time"
)

var (
	leader, host, dataDir string
	hport, dport          int
	rtimeout, wtimeout    time.Duration
)

func init() {
	flag.IntVar(&hport, "hport", 8080, "HTTP Port")
	flag.IntVar(&dport, "dport", 53, "DNS Port")
	flag.StringVar(&leader, "leader", "", "SkyDNS Leader")
	flag.StringVar(&host, "host", "127.0.0.1", "SkyDNS bind ip")
	flag.StringVar(&dataDir, "data", "./data", "SkyDNS data directory")
	flag.DurationVar(&rtimeout, "rtimeout", 1*time.Second, "Read timeout")
	flag.DurationVar(&wtimeout, "wtimeout", 1*time.Second, "Write timeout")
}

func main() {
	raft.SetLogLevel(0)

	flag.Parse()
	s := NewServer(leader, host, dport, hport, dataDir, rtimeout, wtimeout)

	waiter := s.Start()
	waiter.Wait()
}
