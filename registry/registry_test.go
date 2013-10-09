package registry

import (
	"github.com/skynetservices/skydns/msg"
	"testing"
)

/* TODO:
   Benchmarks
*/

var services = []msg.Service{
	msg.Service{
		UUID:        "123",
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Test",
		Host:        "localhost",
		Environment: "Production",
		Port:        9000,
		TTL:         4,
	},
	msg.Service{
		UUID:        "321",
		Name:        "TestService",
		Version:     "1.0.1",
		Region:      "Test",
		Environment: "Production",
		Host:        "localhost",
		Port:        9001,
		TTL:         4,
	},
}

func TestAdd(t *testing.T) {
	reg := New()

	for _, s := range services {
		if err := reg.Add(s); err != nil {
			t.Fatal(err)
		}
	}

	if reg.Len() != 2 {
		t.Fatal("Registry length incorrect")
	}
}

func TestAddDuplicate(t *testing.T) {
	reg := New()

	s := msg.Service{
		UUID:        "123",
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Test",
		Host:        "localhost",
		Environment: "Production",
		Port:        9000,
		TTL:         4,
	}

	reg.Add(s)
	err := reg.Add(s)

	if err != ErrExists {
		t.Fatal("Registry should error on duplicate")
	}
}

func TestGetRegistryKey(t *testing.T) {
	s := msg.Service{
		UUID:        "123",
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Test",
		Host:        "localhost",
		Environment: "Production",
		Port:        9000,
		TTL:         4,
	}

	key := getRegistryKey(s)

	if key != "123.localhost.test.1-0-0.testservice.production" {
		t.Fatal("Key incorrect. Received: ", key)
	}
}

func TestRemove(t *testing.T) {
	reg := New()

	for _, s := range services {
		if err := reg.Add(s); err != nil {
			t.Fatal(err)
		}
	}

	err := reg.Remove(services[0])

	if err != nil {
		t.Fatal(err)
	}

	if reg.Len() != 1 {
		t.Fatal("Service not removed")
	}
}

func TestRemoveUUID(t *testing.T) {
	reg := New()

	for _, s := range services {
		if err := reg.Add(s); err != nil {
			t.Fatal(err)
		}
	}

	err := reg.RemoveUUID(services[0].UUID)

	if err != nil {
		t.Fatal(err)
	}

	if reg.Len() != 1 {
		t.Fatal("Service not removed")
	}
}

func TestGet(t *testing.T) {
	reg := New()

	for _, s := range services {
		if err := reg.Add(s); err != nil {
			t.Fatal(err)
		}
	}

	// Test explicit path
	results, err := reg.Get("123.localhost.test.1-0-0.testservice.production")

	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatal("Failed to return correct services")
	}

	// Test Wildcard
	results, err = reg.Get("any.localhost.test.all.testservice.production")

	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 2 {
		t.Fatal("Failed to return correct services")
	}

	// Test implicit wildcards
	results, err = reg.Get("testservice.production")

	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 2 {
		t.Fatal("Failed to return correct services")
	}

	// Test only supplying any for environment
	results, err = reg.Get("any")

	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 2 {
		t.Fatal("Failed to return correct services")
	}

	// Test trailing .
	results, err = reg.Get("testservice.production.")

	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 2 {
		t.Fatal("Failed to return correct services")
	}
}

func TestGetUUID(t *testing.T) {
	reg := New()

	for _, s := range services {
		if err := reg.Add(s); err != nil {
			t.Fatal(err)
		}
	}

	s, err := reg.GetUUID(services[0].UUID)

	if err != nil {
		t.Fatal(err)
	}

	if s != services[0] {
		t.Fatal("Failed to retrieve proper service")
	}
}

func TestUpdateTTL(t *testing.T) {
	reg := New()
	r := reg.(*DefaultRegistry)

	for _, s := range services {
		if err := reg.Add(s); err != nil {
			t.Fatal(err)
		}
	}

	origExpire := r.nodes[services[0].UUID].expires

	if err := reg.UpdateTTL(services[0].UUID, 10); err != nil {
		t.Fatal("Failed to update TTL", err)
	}

	results, err := reg.Get("123.localhost.test.1-0-0.testservice.production")

	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatal("Failed to return correct services")
	}

	// Validate TTL and Expiration
	if results[0].TTL != 10 {
		t.Fatal("TTL was not updated", results[0].TTL)
	}

	if r.nodes[services[0].UUID].expires.Unix() <= origExpire.Unix() {
		t.Fatal("Service expiration not updated")
	}
}

func TestGetExpired(t *testing.T) {
	reg := New()

	s := msg.Service{
		UUID:        "123",
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Test",
		Host:        "localhost",
		Environment: "Production",
		Port:        9000,
		TTL:         500,
	}

	sExpired := msg.Service{
		UUID:        "124",
		Name:        "TestService",
		Version:     "1.0.0",
		Region:      "Test",
		Host:        "localhost",
		Environment: "Production",
		Port:        9000,
		TTL:         0,
	}

	reg.Add(s)
	reg.Add(sExpired)

	expired := reg.GetExpired()

	if len(expired) != 1 {
		t.Fatalf("Expected %d expired services, received %d", 1, len(expired))
	}

	if expired[0] != "124" {
		t.Fatal("Incorrect UUID returned for expired entry")
	}
}
