// Copyright (c) 2013 Erik St. Martin, Brian Ketelsen. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package main

import (
	"errors"
	"flag"
	"github.com/goraft/raft"
	"github.com/miekg/dns"
	"github.com/rcrowley/go-metrics"
	"github.com/rcrowley/go-metrics/stathat"
	"github.com/skynetservices/skydns/server"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

var (
	join, ldns, lhttp, dataDir, domain string
	rtimeout, wtimeout                 time.Duration
	discover                           bool
	metricsToStdErr                    bool
	graphiteServer, stathatUser        string
	secret                             string
	nameserver                         string
	dnssec                             string
)

func init() {
	flag.StringVar(&join, "join", "", "Member of SkyDNS cluster to join can be comma separated list")
	flag.BoolVar(&discover, "discover", false, "Auto discover SkyDNS cluster. Performs an NS lookup on the -domain to find SkyDNS members")
	flag.StringVar(&domain, "domain",
		func() string {
			if x := os.Getenv("SKYDNS_DOMAIN"); x != "" {
				return x
			}
			return "skydns.local"
		}(), "Domain to anchor requests to or env. var. SKYDNS_DOMAIN")
	flag.StringVar(&ldns, "dns",
		func() string {
			if x := os.Getenv("SKYDNS_DNS"); x != "" {
				return x
			}
			return "127.0.0.1:53"
		}(), "IP:Port to bind to for DNS or env. var SKYDNS_DNS")
	flag.StringVar(&lhttp, "http",
		func() string {
			if x := os.Getenv("SKYDNS"); x != "" {
				// get rid of http or https
				x1 := strings.TrimPrefix(x, "https://")
				x1 = strings.TrimPrefix(x1, "http://")
				return x1
			}
			return "127.0.0.1:8080"
		}(), "IP:Port to bind to for HTTP or env. var. SKYDNS")
	flag.StringVar(&dataDir, "data", "./data", "SkyDNS data directory")
	flag.DurationVar(&rtimeout, "rtimeout", 2*time.Second, "Read timeout")
	flag.DurationVar(&wtimeout, "wtimeout", 2*time.Second, "Write timeout")
	flag.BoolVar(&metricsToStdErr, "metricsToStdErr", false, "Write metrics to stderr periodically")
	flag.StringVar(&graphiteServer, "graphiteServer", "", "Graphite Server connection string e.g. 127.0.0.1:2003")
	flag.StringVar(&stathatUser, "stathatUser", "", "StatHat account for metrics")
	flag.StringVar(&secret, "secret", "", "Shared secret for use with http api")
	flag.StringVar(&nameserver, "nameserver", "", "Nameserver address to forward (non-local) queries to e.g. 8.8.8.8:53,8.8.4.4:53")
	flag.StringVar(&dnssec, "dnssec", "", "Basename of DNSSEC key file e.q. Kskydns.local.+005+38250")
}

func main() {
	members := make([]string, 0)
	raft.SetLogLevel(0)
	flag.Parse()
	nameservers := strings.Split(nameserver, ",")
	// empty argument given
	if len(nameservers) == 1 && nameservers[0] == "" {
		nameservers = make([]string, 0)
		config, err := dns.ClientConfigFromFile("/etc/resolv.conf")
		if err == nil {
			for _, s := range config.Servers {
				nameservers = append(nameservers, net.JoinHostPort(s, config.Port))
			}
		} else {
			log.Fatal(err)
			return
		}
	}

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
			members = append(members, strings.TrimPrefix(n.Host, "."))
		}
	} else if join != "" {
		members = strings.Split(join, ",")
	}

	s := server.NewServer(members, domain, ldns, lhttp, dataDir, rtimeout, wtimeout, secret, nameservers)

	if dnssec != "" {
		k, p, e := parseKeyFile(dnssec)
		if e != nil {
			log.Fatal(e)
		}
		if k.Header().Name != dns.Fqdn(domain) {
			log.Fatal(errors.New("Owner name of DNSKEY must match SkyDNS domain"))
		}
		s.Dnskey = k
		s.Privkey = p
	}

	// Set up metrics if specified on the command line
	if metricsToStdErr {
		go metrics.Log(metrics.DefaultRegistry, 60e9, log.New(os.Stderr, "metrics: ", log.Lmicroseconds))
	}

	if len(graphiteServer) > 1 {
		addr, err := net.ResolveTCPAddr("tcp", graphiteServer)
		if err != nil {
			go metrics.Graphite(metrics.DefaultRegistry, 10e9, "skydns", addr)
		}
	}

	if len(stathatUser) > 1 {
		go stathat.Stathat(metrics.DefaultRegistry, 10e9, stathatUser)
	}

	waiter, err := s.Start()
	if err != nil {
		log.Fatal(err)
		return
	}
	waiter.Wait()
}
