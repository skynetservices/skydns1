// Copyright (c) 2013 Erik St. Martin, Brian Ketelsen. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package server

import (
	"github.com/miekg/dns"
	"strings"
	"testing"
)

func TestDNSSEC(t *testing.T) {
	s := newTestServer("", "", "")
	defer s.Stop()

	for _, m := range services {
		s.registry.Add(m)
	}
	c := new(dns.Client)
	for _, tc := range dnsTestCases {
		m := new(dns.Msg)
		m.SetQuestion(tc.Question, dns.TypeSRV)
		m.SetEdns0(4096, true)
		resp, _, err := c.Exchange(m, "localhost:"+StrPort)

		if err != nil {
			t.Fatal(err)
		}
		resp = resp // TODO(miek): fix test
	}
}

type dnssecTestCase struct {
	Question     string
	AnswerSRV    []*dns.SRV
	AnswerRRSIG  []*dns.RRSIG
	AnswerDNSKEY []*dns.DNSKEY
	NsSOA        []*dns.SOA
	NsRRSIG      []*dns.RRSIG
}

/*
var dnssecTestCases = []dnssecTestCase{
	// Generic Test
	{
		Question: "testservice.production.skydns.local.",
		Answer: []dns.SRV{
			{
				Hdr: dns.RR_Header{
					Name:   "testservice.production.skydns.local.",
					Ttl:    30,
					Rrtype: dns.TypeSRV,
				},
				Priority: 10,
				Weight:   33,
				Target:   "server2.",
				Port:     9001,
			},
			{
				Hdr: dns.RR_Header{
					Name:   "testservice.production.skydns.local.",
					Ttl:    33,
					Rrtype: dns.TypeSRV,
				},
				Priority: 10,
				Weight:   33,
				Target:   "server5.",
				Port:     9004,
			},
			{
				Hdr: dns.RR_Header{
					Name:   "testservice.production.skydns.local.",
					Ttl:    34,
					Rrtype: dns.TypeSRV,
				},
				Priority: 10,
				Weight:   33,
				Target:   "server6.",
				Port:     9005,
			},
		},
	},

	// Region Priority Test
	{
		Question: "region1.*.testservice.production.skydns.local.",
		Answer: []dns.SRV{
			{
				Hdr: dns.RR_Header{
					Name:   "region1.*.testservice.production.skydns.local.",
					Ttl:    30,
					Rrtype: dns.TypeSRV,
				},
				Priority: 10,
				Weight:   100,
				Target:   "server2.",
				Port:     9001,
			},
			{
				Hdr: dns.RR_Header{
					Name:   "region1.*.testservice.production.skydns.local.",
					Ttl:    33,
					Rrtype: dns.TypeSRV,
				},
				Priority: 20,
				Weight:   50,
				Target:   "server5.",
				Port:     9004,
			},
			{
				Hdr: dns.RR_Header{
					Name:   "region1.*.testservice.production.skydns.local.",
					Ttl:    34,
					Rrtype: dns.TypeSRV,
				},
				Priority: 20,
				Weight:   50,
				Target:   "server6.",
				Port:     9005,
			},
		},
	},
}
*/

func newTestServerDNSSEC(leader, secret, nameserver string) *Server {
	s := newTestServer(leader, secret, nameserver)
	key, _ := dns.NewRR("skydns.local. IN DNSKEY 256 3 5 AwEAAaXfO+DOBMJsQ5H4TfiabwSpqE4cGL0Qlvh5hrQumrjr9eNSdIOjIHJJKCe56qBU5mH+iBlXP29SVf6UiiMjIrAPDVhClLeWFe0PC+XlWseAyRgiLHdQ8r95+AfkhO5aZgnCwYf9FGGSaT0+CRYN+PyDbXBTLK5FN+j5b6bb7z+d")
	s.Dnskey = key.(*dns.DNSKEY)
	s.KeyTag = s.Dnskey.KeyTag()
	s.Privkey, _ = s.Dnskey.ReadPrivateKey(strings.NewReader(`
Private-key-format: v1.3
Algorithm: 5 (RSASHA1)
Modulus: pd874M4EwmxDkfhN+JpvBKmoThwYvRCW+HmGtC6auOv141J0g6MgckkoJ7nqoFTmYf6IGVc/b1JV/pSKIyMisA8NWEKUt5YV7Q8L5eVax4DJGCIsd1Dyv3n4B+SE7lpmCcLBh/0UYZJpPT4JFg34/INtcFMsrkU36PlvptvvP50=
PublicExponent: AQAB
PrivateExponent: C6e08GXphbPPx6j36ZkIZf552gs1XcuVoB4B7hU8P/Qske2QTFOhCwbC8I+qwdtVWNtmuskbpvnVGw9a6X8lh7Z09RIgzO/pI1qau7kyZcuObDOjPw42exmjqISFPIlS1wKA8tw+yVzvZ19vwRk1q6Rne+C1romaUOTkpA6UXsE=
Prime1: 2mgJ0yr+9vz85abrWBWnB8Gfa1jOw/ccEg8ZToM9GLWI34Qoa0D8Dxm8VJjr1tixXY5zHoWEqRXciTtY3omQDQ==
Prime2: wmxLpp9rTzU4OREEVwF43b/TxSUBlUq6W83n2XP8YrCm1nS480w4HCUuXfON1ncGYHUuq+v4rF+6UVI3PZT50Q==
Exponent1: wkdTngUcIiau67YMmSFBoFOq9Lldy9HvpVzK/R0e5vDsnS8ZKTb4QJJ7BaG2ADpno7pISvkoJaRttaEWD3a8rQ==
Exponent2: YrC8OglEXIGkV3tm2494vf9ozPL6+cBkFsPPg9dXbvVCyyuW0pGHDeplvfUqs4nZp87z8PsoUL+LAUqdldnwcQ==
Coefficient: mMFr4+rDY5V24HZU3Oa5NEb55iQ56ZNa182GnNhWqX7UqWjcUUGjnkCy40BqeFAQ7lp52xKHvP5Zon56mwuQRw==
Created: 20140126132645
Publish: 20140126132645
Activate: 20140126132645`), "stdin")
	return s
}
