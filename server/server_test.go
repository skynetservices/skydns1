// Copyright (c) 2013 The SkyDNS Authors. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package server

import (
	"bytes"
	"encoding/json"
	"github.com/miekg/dns"
	"github.com/skynetservices/skydns1/msg"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// keep global port counter that increments with 10 for each
// new call to newTestServer. The dns server is started on port 'port'
// the http server is started on 'port+1'.
var Port = 9490
var StrPort = "9490" // string equivalent of Port

/* TODO: Tests
   Test Cluster
   Test TTL expiration
   Benchmarks
*/

func TestAddService(t *testing.T) {
	s := newTestServer("", "", "")
	defer s.Stop()

	m := msg.Service{
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Test",
		Host:        "localhost",
		Environment: "Production",
		Port:        9000,
		TTL:         4,
	}

	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("PUT", "/skydns/services/123", bytes.NewBuffer(b))
	resp := httptest.NewRecorder()

	s.router.ServeHTTP(resp, req)

	if resp.Code != 201 || s.registry.Len() != 1 {
		t.Fatal("Failed to add service")
	}
}

func TestAddServiceDuplicate(t *testing.T) {
	s := newTestServer("", "", "")
	defer s.Stop()

	m := msg.Service{
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Test",
		Host:        "localhost",
		Environment: "Production",
		Port:        9000,
		TTL:         4,
	}

	b, err := json.Marshal(m)

	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("PUT", "/skydns/services/123", bytes.NewBuffer(b))
	resp := httptest.NewRecorder()

	s.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated || s.registry.Len() != 1 {
		t.Fatal("Failed to add service")
	}

	req, _ = http.NewRequest("PUT", "/skydns/services/123", bytes.NewBuffer(b))
	resp = httptest.NewRecorder()
	s.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusConflict || s.registry.Len() != 1 {
		t.Fatal("Duplicates should return error code 407", resp.Code)
	}
}

func TestRemoveService(t *testing.T) {
	s := newTestServer("", "", "")
	defer s.Stop()

	m := msg.Service{
		UUID:        "123",
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Test",
		Host:        "localhost",
		Environment: "Production",
		Port:        9000,
		TTL:         4,
	}

	s.registry.Add(m)

	req, _ := http.NewRequest("DELETE", "/skydns/services/"+m.UUID, nil)
	resp := httptest.NewRecorder()

	s.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK || s.registry.Len() != 0 {
		t.Fatal("Failed to remove service")
	}
}

func TestRemoveUnknownService(t *testing.T) {
	s := newTestServer("", "", "")
	defer s.Stop()

	m := msg.Service{
		UUID:        "123",
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Test",
		Host:        "localhost",
		Environment: "Production",
		Port:        9000,
		TTL:         4,
	}
	s.registry.Add(m)

	req, _ := http.NewRequest("DELETE", "/skydns/services/54321", nil)
	resp := httptest.NewRecorder()

	s.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusNotFound || s.registry.Len() != 1 {
		t.Fatal("API should return 404 when removing unknown service")
	}
}

func TestUpdateTTL(t *testing.T) {
	s := newTestServer("", "", "")
	defer s.Stop()

	m := msg.Service{
		UUID:        "123",
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Test",
		Host:        "localhost",
		Environment: "Production",
		Port:        9000,
		TTL:         4,
	}

	s.registry.Add(m)

	m.TTL = 25
	b, err := json.Marshal(m)

	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("PATCH", "/skydns/services/"+m.UUID, bytes.NewBuffer(b))
	resp := httptest.NewRecorder()

	s.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatal("Failed to update TTL")
	}

	if serv, err := s.registry.GetUUID(m.UUID); err != nil || serv.TTL != 24 {
		t.Fatal("Failed to update TTL", err, serv.TTL)
	}
}

func TestUpdateTTLUnknownService(t *testing.T) {
	s := newTestServer("", "", "")
	defer s.Stop()

	m := msg.Service{
		UUID:        "54321",
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Test",
		Host:        "localhost",
		Environment: "Production",
		Port:        9000,
		TTL:         4,
	}

	b, err := json.Marshal(m)

	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("PATCH", "/skydns/services/"+m.UUID, bytes.NewBuffer(b))
	resp := httptest.NewRecorder()

	s.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound || s.registry.Len() != 0 {
		t.Fatal("API should return 404 when updating unknown service")
	}
}

func TestGetService(t *testing.T) {
	s := newTestServer("", "", "")
	defer s.Stop()

	m := msg.Service{
		UUID:        "123",
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Test",
		Host:        "localhost",
		Environment: "Production",
		Port:        9000,
		TTL:         4,
		Expires:     getExpirationTime(4),
	}

	s.registry.Add(m)

	req, _ := http.NewRequest("GET", "/skydns/services/"+m.UUID, nil)
	resp := httptest.NewRecorder()

	s.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatal("Failed to retrieve service")
	}

	m.TTL = 3 // TTL will be lower as time has passed
	expected, err := json.Marshal(m)

	if err != nil {
		t.Fatal(err)
	}

	// Newline is expected
	expected = append(expected, []byte("\n")...)

	if !bytes.Equal(resp.Body.Bytes(), expected) {
		t.Fatalf("Returned service is invalid. Expected %q but received %q", string(expected), resp.Body.String())
	}
}

func TestGetEnvironments(t *testing.T) {
	s := newTestServer("", "", "")
	defer s.Stop()

	for _, m := range services {
		s.registry.Add(m)
	}

	req, _ := http.NewRequest("GET", "/skydns/environments/", nil)
	resp := httptest.NewRecorder()

	s.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatal("Failed to retrieve environment list")
	}

	//server sends \n at the end, account for this
	expected := `{"Development":2,"Production":5}`
	expected = expected + "\n"

	if !bytes.Equal(resp.Body.Bytes(), []byte(expected)) {
		t.Fatal("Expected ", expected, " got %s", string(resp.Body.Bytes()))
	}
}

func TestGetRegions(t *testing.T) {
	s := newTestServer("", "", "")
	defer s.Stop()

	for _, m := range services {
		s.registry.Add(m)
	}

	req, _ := http.NewRequest("GET", "/skydns/regions/", nil)
	resp := httptest.NewRecorder()

	s.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatal("Failed to retrieve region list")
	}

	//server sends \n at the end, account for this
	expected := `{"Region1":3,"Region2":2,"Region3":2}`
	expected = expected + "\n"

	if !bytes.Equal(resp.Body.Bytes(), []byte(expected)) {
		t.Fatal("Expected ", expected, " got %s", string(resp.Body.Bytes()))
	}
}

func TestAuthenticationFailure(t *testing.T) {
	s := newTestServer("", "supersecretpassword", "")
	defer s.Stop()

	m := msg.Service{
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Test",
		Host:        "localhost",
		Environment: "Production",
		Port:        9000,
		TTL:         4,
	}

	b, err := json.Marshal(m)

	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("PUT", "/skydns/services/123", bytes.NewBuffer(b))
	resp := httptest.NewRecorder()

	s.router.ServeHTTP(resp, req)
	if resp.Code != 401 {
		t.Fatal("Authentication should have failed and it worked.")
	}
}

func TestAuthenticationSuccess(t *testing.T) {
	secret := "myimportantsecret"
	s := newTestServer("", secret, "")
	defer s.Stop()

	m := msg.Service{
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Test",
		Host:        "localhost",
		Environment: "Production",
		Port:        9000,
		TTL:         4,
	}

	b, err := json.Marshal(m)

	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("PUT", "/skydns/services/123", bytes.NewBuffer(b))
	req.Header.Set("Authorization", secret)
	resp := httptest.NewRecorder()

	s.router.ServeHTTP(resp, req)
	if resp.Code != 201 {
		t.Fatal("Auth Should have worked and it failed")
	}
}

func TestHostFailure(t *testing.T) {
	s := newTestServer("", "", "")
	defer s.Stop()

	m := msg.Service{
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Test",
		Environment: "Production",
		Port:        9000,
		TTL:         4,
	}

	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("PUT", "/skydns/services/123", bytes.NewBuffer(b))
	resp := httptest.NewRecorder()

	s.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatal("Failed to detect empty Host.")
	}
}

func TestCallback(t *testing.T) {
	s := newTestServer("", "", "")
	defer s.Stop()

	m := msg.Service{
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Test",
		Environment: "Production",
		Host:        "localhost",
		Port:        9000,
		TTL:         4,
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("PUT", "/skydns/services/123", bytes.NewBuffer(b))
	resp := httptest.NewRecorder()
	s.router.ServeHTTP(resp, req)

	c := msg.Callback{
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Test",
		Environment: "Production",
		Host:        "localhost",
		Reply:       "localhost",
		Port:        9650,
	}
	b, err = json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	req, _ = http.NewRequest("PUT", "/skydns/callbacks/101", bytes.NewBuffer(b))
	resp = httptest.NewRecorder()

	s.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("Failed to perform callback: %d", resp.Code)
	}
	req, _ = http.NewRequest("DELETE", "/skydns/services/123", nil)
	resp = httptest.NewRecorder()
	s.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatal("Failed to remove service")
		// TODO(miek): check for the callback to be performed
	}
}

func TestCallbackFailure(t *testing.T) {
	s := newTestServer("", "", "")
	defer s.Stop()

	m := msg.Service{
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Test",
		Environment: "Production",
		Host:        "localhost",
		Port:        9000,
		TTL:         4,
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("PUT", "/skydns/services/123", bytes.NewBuffer(b))
	resp := httptest.NewRecorder()
	s.router.ServeHTTP(resp, req)

	c := msg.Callback{
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Test",
		Environment: "Testing", // should result in notFound
		Host:        "localhost",
		Reply:       "localhost",
		Port:        9650,
	}
	b, err = json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	req, _ = http.NewRequest("PUT", "/skydns/callbacks/101", bytes.NewBuffer(b))
	resp = httptest.NewRecorder()

	s.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatal("Callback should result in service not found.")
	}
}

var services = []msg.Service{
	{
		UUID:        "100",
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Region1",
		Host:        "server1",
		Environment: "Development",
		Port:        9000,
		TTL:         30,
		Expires:     getExpirationTime(30),
	},
	{
		UUID:        "101",
		Name:        "TestService",
		Version:     "1.0.1",
		Region:      "Region1",
		Host:        "server2",
		Environment: "Production",
		Port:        9001,
		TTL:         31,
		Expires:     getExpirationTime(31),
	},
	{
		UUID:        "102",
		Name:        "OtherService",
		Version:     "1.0.0",
		Region:      "Region2",
		Host:        "server3",
		Environment: "Production",
		Port:        9002,
		TTL:         32,
		Expires:     getExpirationTime(32),
	},
	{
		UUID:        "103",
		Name:        "TestService",
		Version:     "1.0.1",
		Region:      "Region1",
		Host:        "server4",
		Environment: "Development",
		Port:        9003,
		TTL:         33,
		Expires:     getExpirationTime(33),
	},
	{
		UUID:        "104",
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Region3",
		Host:        "server5",
		Environment: "Production",
		Port:        9004,
		TTL:         34,
		Expires:     getExpirationTime(34),
	},
	{
		UUID:        "105",
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Region3",
		Host:        "server6",
		Environment: "Production",
		Port:        9005,
		TTL:         35,
		Expires:     getExpirationTime(35),
	},
	{
		UUID:        "106",
		Name:        "OtherService",
		Version:     "1.0.0",
		Region:      "Region2",
		Host:        "server7",
		Environment: "Production",
		Port:        9006,
		TTL:         36,
		Expires:     getExpirationTime(36),
	},
}

type dnsTestCase struct {
	Question string
	Answer   []dns.SRV
}

var dnsTestCases = []dnsTestCase{
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

	// TODO: Test for case insensitive, this causes an error within the dns package though
}

type servicesTest struct {
	query string
	count int
}

var serviceTestArray []servicesTest = []servicesTest{
	{"*", 7},
	{"production", 5},
	{"testservice.production", 3},
	{"region1.*.*.production", 1},
	{"region1.*.testservice.production", 1},
	{"region1.*.TestService.production", 1},
}

func TestGetServicesWithQueries(t *testing.T) {
	s := newTestServer("", "", "")
	defer s.Stop()

	for _, m := range services {
		s.registry.Add(m)
	}

	for _, st := range serviceTestArray {
		req, _ := http.NewRequest("GET", "/skydns/services/?query="+st.query, nil)
		resp := httptest.NewRecorder()
		s.router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatal("Failed To Retrieve Services")
		}
		var returnedServices []msg.Service
		err := json.Unmarshal(resp.Body.Bytes(), &returnedServices)
		if err != nil {
			t.Fatal("Failed to unmarshal response from server")
		}
		if len(returnedServices) != st.count {
			t.Fatal("Expected ", st.count, " got %d services", len(returnedServices))
		}

	}

}

func TestDNS(t *testing.T) {
	s := newTestServer("", "", "")
	defer s.Stop()

	for _, m := range services {
		s.registry.Add(m)
	}
	c := new(dns.Client)
	for _, tc := range dnsTestCases {
		m := new(dns.Msg)
		m.SetQuestion(tc.Question, dns.TypeSRV)
		resp, _, err := c.Exchange(m, "localhost:"+StrPort)

		if err != nil {
			t.Fatal(err)
		}

		if len(resp.Answer) != len(tc.Answer) {
			t.Fatalf("Response for %q contained %d results, %d expected", tc.Question, len(resp.Answer), len(tc.Answer))
		}

		for i, a := range resp.Answer {
			srv := a.(*dns.SRV)

			// Validate Header
			if srv.Hdr.Name != tc.Answer[i].Hdr.Name {
				t.Errorf("Answer %d should have a Header Name of %q, but has %q", i, tc.Answer[i].Hdr.Name, srv.Hdr.Name)
			}

			if srv.Hdr.Ttl != tc.Answer[i].Hdr.Ttl {
				t.Errorf("Answer %d should have a Header TTL of %d, but has %d", i, tc.Answer[i].Hdr.Ttl, srv.Hdr.Ttl)
			}

			if srv.Hdr.Rrtype != tc.Answer[i].Hdr.Rrtype {
				t.Errorf("Answer %d should have a Header Response Type of %d, but has %d", i, tc.Answer[i].Hdr.Rrtype, srv.Hdr.Rrtype)
			}

			// Validate Record
			if srv.Priority != tc.Answer[i].Priority {
				t.Errorf("Answer %d should have a Priority of %d, but has %d", i, tc.Answer[i].Priority, srv.Priority)
			}

			if srv.Weight != tc.Answer[i].Weight {
				t.Errorf("Answer %d should have a Weight of %d, but has %d", i, tc.Answer[i].Weight, srv.Weight)
			}

			if srv.Port != tc.Answer[i].Port {
				t.Errorf("Answer %d should have a Port of %d, but has %d", i, tc.Answer[i].Port, srv.Port)
			}

			if srv.Target != tc.Answer[i].Target {
				t.Errorf("Answer %d should have a Target of %q, but has %q", i, tc.Answer[i].Target, srv.Target)
			}
		}
	}
}

func TestDNSARecords(t *testing.T) {
	s := newTestServer("", "", "")
	defer s.Stop()

	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion("skydns.local.", dns.TypeA)
	resp, _, err := c.Exchange(m, "localhost:"+StrPort)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Answer) != 1 {
		t.Fatal("Answer expected to have 2 A records but has", len(resp.Answer))
	}
}

func TestDNSForward(t *testing.T) {
	s := newTestServer("", "", "8.8.8.8:53")
	defer s.Stop()

	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion("www.example.com.", dns.TypeA)
	resp, _, err := c.Exchange(m, "localhost:"+StrPort)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Answer) == 0 || resp.Rcode != dns.RcodeSuccess {
		t.Fatal("Answer expected to have A records or rcode not equal to RcodeSuccess")
	}
	// TCP
	c.Net = "tcp"
	resp, _, err = c.Exchange(m, "localhost:"+StrPort)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Answer) == 0 || resp.Rcode != dns.RcodeSuccess {
		t.Fatal("Answer expected to have A records or rcode not equal to RcodeSuccess")
	}
	// TODO(miek): DNSSEC DO query
}

func newTestServer(leader, secret, nameserver string) *Server {
	members := make([]string, 0)

	p, _ := ioutil.TempDir("", "skydns-test-")
	if err := os.MkdirAll(p, 0644); err != nil {
		panic(err.Error())
	}
	if leader != "" {
		members = append(members, leader)
	}

	Port += 10
	StrPort = strconv.Itoa(Port)
	server := NewServer(members, "skydns.local", net.JoinHostPort("127.0.0.1", StrPort), net.JoinHostPort("127.0.0.1", strconv.Itoa(Port+1)), p, 1*time.Second, 1*time.Second, secret, []string{nameserver}, false, "", "")
	server.Start()
	return server
}

// DNSSEC tests

func sectionCheck(t *testing.T, resp []dns.RR, tc []dns.RR) {
	// check the RRs in the response
	for i, r := range resp {
		if r.Header().Name != tc[i].Header().Name {
			t.Errorf("Response should have a Header Name of %q, but has %q", r.Header().Name, tc[i].Header().Name)
		}
		if r.Header().Rrtype != tc[i].Header().Rrtype {
			t.Errorf("Response should have a Header Type of %q, but has %q", r.Header().Rrtype, tc[i].Header().Rrtype)
		}
		if r.Header().Ttl != tc[i].Header().Ttl {
			t.Errorf("Response should have a Header Ttl of %q, but has %q", r.Header().Ttl, tc[i].Header().Ttl)
		}
		switch rt := r.(type) {
		case *dns.DNSKEY:
			tt := tc[i].(*dns.DNSKEY)
			if rt.Flags != tt.Flags {
				t.Errorf("DNSKEY flags should be %q, but is %q", rt.Flags, tt.Flags)
			}
			if rt.Protocol != tt.Protocol {
				t.Errorf("DNSKEY protocol should be %q, but is %q", rt.Protocol, tt.Protocol)
			}
			if rt.Algorithm != tt.Algorithm {
				t.Errorf("DNSKEY algorithm should be %q, but is %q", rt.Algorithm, tt.Algorithm)
			}
		case *dns.RRSIG:
			tt := tc[i].(*dns.RRSIG)
			if rt.TypeCovered != tt.TypeCovered {
				t.Errorf("RRSIG type-covered should be %q, but is %q", rt.TypeCovered, tt.TypeCovered)
			}
			if rt.Algorithm != tt.Algorithm {
				t.Errorf("RRSIG algorithm should be %q, but is %q", rt.Algorithm, tt.Algorithm)
			}
			if rt.Labels != tt.Labels {
				t.Errorf("RRSIG label should be %q, but is %q", rt.Labels, tt.Labels)
			}
			if rt.OrigTtl != tt.OrigTtl {
				t.Errorf("RRSIG orig-ttl should be %q, but is %q", rt.OrigTtl, tt.OrigTtl)
			}
			if rt.KeyTag != tt.KeyTag {
				t.Errorf("RRSIG key-tag should be %q, but is %q", rt.KeyTag, tt.KeyTag)
			}
			if rt.SignerName != tt.SignerName {
				t.Errorf("RRSIG signer-name should be %q, but is %q", rt.SignerName, tt.SignerName)
			}
		}
	}
}

func TestDNSSEC(t *testing.T) {
	s := newTestServer("", "", "")
	defer s.Stop()

	for _, m := range services {
		s.registry.Add(m)
	}
	c := new(dns.Client)
	for _, tc := range dnssecTestCases {
		m := newMsg(tc)
		resp, _, err := c.Exchange(m, "localhost:"+StrPort)
		if err != nil {
			t.Fatal(err)
		}
		sectionCheck(t, resp.Answer, tc.Answer)
	}
}

type dnssecTestCase struct {
	Question dns.Question
	Answer   []dns.RR
	Ns       []dns.RR
	Extra    []dns.RR
}

var dnssecTestCases = []dnssecTestCase{
	// DNSKEY Test
	{
		Question: dns.Question{"skydns.local.", dns.TypeDNSKEY, dns.ClassINET},
		Answer: []dns.RR{&dns.DNSKEY{
			Hdr: dns.RR_Header{
				Name:   "skydns.local.",
				Ttl:    origTTL,
				Rrtype: dns.TypeDNSKEY,
			},
			Flags:     256,
			Protocol:  3,
			Algorithm: 5,
			PublicKey: "not important",
		},
			&dns.RRSIG{
				Hdr: dns.RR_Header{
					Name:   "skydns.local.",
					Ttl:    origTTL,
					Rrtype: dns.TypeRRSIG,
				},
				TypeCovered: dns.TypeDNSKEY,
				Algorithm:   5,
				Labels:      2,
				OrigTtl:     origTTL,
				Expiration:  0,
				Inception:   0,
				KeyTag:      51945,
				SignerName:  "skydns.local.",
				Signature:   "not important",
			},
		},
	},
}

/*
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
	s.dnsKey = key.(*dns.DNSKEY)
	s.keyTag = s.dnsKey.KeyTag()
	s.privKey, _ = s.dnsKey.ReadPrivateKey(strings.NewReader(`
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

// newMsg return a new dns.Msg set with DNSSEC and with the question from the tc.
func newMsg(tc dnssecTestCase) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(tc.Question.Name, tc.Question.Qtype)
	m.SetEdns0(4096, true)
	return m
}
