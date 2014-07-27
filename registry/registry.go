// Copyright (c) 2013 The SkyDNS Authors. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package registry

import (
	"errors"
	"fmt"
	"github.com/miekg/dns"
	"github.com/skynetservices/skydns1/msg"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrExists    = errors.New("Service already exists in registry")
	ErrNotExists = errors.New("Service does not exist in registry")
)

type Registry interface {
	Add(s msg.Service) error
	Get(domain string) ([]msg.Service, error)
	GetUUID(uuid string) (msg.Service, error)
	GetExpired() []string
	Remove(s msg.Service) error
	RemoveUUID(uuid string) error
	UpdateTTL(uuid string, ttl uint32, expires time.Time) error
	AddCallback(s msg.Service, c msg.Callback) error
	Len() int
	// GetNSEC return the previous and next name according to the key given.
	GetNSEC(key string) (string, string)
	// DNSSEC sets or resets if we support DNSSEC.
	DNSSEC(bool) bool
}

// New returns a new DefaultRegistry.
func New() Registry {
	return &DefaultRegistry{
		tree:  newNode(),
		nodes: make(map[string]*node),
		nsec:  make([]denialReference, 0, 10),
	}
}

// DefaultRegistry is a datastore for registered services.
type DefaultRegistry struct {
	tree  *node
	nodes map[string]*node
	mutex sync.Mutex

	// holds a list of sorted domain names
	nsec   []denialReference // D N S S E C
	dnssec bool              // if dnssec is disabled, some expensive data structures aren't used
}

type denialReference struct {
	name      string // domain name
	reference int    // reference count
}

// Add adds a service to registry.
func (r *DefaultRegistry) Add(s msg.Service) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// TODO: Validate service has correct values, and getRegistryKey returns a valid value
	if _, ok := r.nodes[s.UUID]; ok {
		return ErrExists
	}

	k := getRegistryKey(s)
	n, err := r.tree.add(strings.Split(k, "."), s)
	if err == nil {
		r.nodes[n.value.UUID] = n
		if r.dnssec {
			r.addNSEC(s.Region + "." + s.Version + "." + s.Name + "." + s.Environment)
			r.addNSEC(s.Version + "." + s.Name + "." + s.Environment)
			r.addNSEC(s.Name + "." + s.Environment)
			r.addNSEC(s.Environment)
		}
	}
	return err
}

// the registry look is already being held.
func (r *DefaultRegistry) addNSEC(key string) {
	i := sort.Search(len(r.nsec), func(i int) bool { return r.nsec[i].name >= key })
	if i < len(r.nsec) && r.nsec[i].name == key {
		r.nsec[i].reference++
		return
	}
	r.nsec = append(r.nsec, denialReference{"", 0})
	copy(r.nsec[i+1:], r.nsec[i:])

	r.nsec[i].name = key
	r.nsec[i].reference = 1
}

// RemoveUUID removes a Service specified by an UUID.
func (r *DefaultRegistry) RemoveUUID(uuid string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if n, ok := r.nodes[uuid]; ok {
		return r.removeService(n.value)
	}
	return ErrNotExists
}

// UpdateTTL updates the TTL of a service, as well as pushes the expiration time out TTL seconds from now.
// This serves as a ping, for the service to keep SkyDNS aware of it's existence so that it is not expired, and purged.
func (r *DefaultRegistry) UpdateTTL(uuid string, ttl uint32, expires time.Time) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if n, ok := r.nodes[uuid]; ok {
		n.value.TTL = ttl
		n.value.Expires = expires
		return nil
	}
	return ErrNotExists
}

// removeService remove service from registry while r.mutex is held.
func (r *DefaultRegistry) removeService(s msg.Service) error {
	// we can always delete, even if r.tree reports it doesn't exist,
	// because this means, we just removed a bad service entry.
	// Map deletion is also a no-op, if entry not found in map
	delete(r.nodes, s.UUID)
	// No matter what, call the callbacks
	log.Println("Calling", len(s.Callback), "callback(s) for service", s.UUID)
	for _, c := range s.Callback {
		c.Call(s)
	}

	// TODO: Validate service has correct values, and getRegistryKey returns a valid value
	k := getRegistryKey(s)
	if r.dnssec {
		r.removeNSEC(s.Region + "." + s.Version + "." + s.Name + "." + s.Environment)
		r.removeNSEC(s.Version + "." + s.Name + "." + s.Environment)
		r.removeNSEC(s.Name + "." + s.Environment)
		r.removeNSEC(s.Environment)
	}

	return r.tree.remove(strings.Split(k, "."))
}

// registry lock is already being held.
func (r *DefaultRegistry) removeNSEC(key string) {
	i := sort.Search(len(r.nsec), func(i int) bool { return r.nsec[i].name >= key })
	if i < len(r.nsec) && r.nsec[i].name == key {
		r.nsec[i].reference--
		if r.nsec[i].reference == 0 {
			copy(r.nsec[i:], r.nsec[i+1:])
			r.nsec[len(r.nsec)-1] = denialReference{"", 0}
			r.nsec = r.nsec[:len(r.nsec)-1]
		}
	}
}

// Remove removes a service from registry.
func (r *DefaultRegistry) Remove(s msg.Service) (err error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if n, ok := r.nodes[s.UUID]; ok {
		return r.removeService(n.value)
	}
	return ErrNotExists
}

// GetUUID retrieves a service based on its UUID.
func (r *DefaultRegistry) GetUUID(uuid string) (s msg.Service, err error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if s, ok := r.nodes[uuid]; ok {
		s.value.TTL = s.value.RemainingTTL()

		if s.value.TTL >= 1 {
			return s.value, nil
		}
	}
	return s, ErrNotExists
}

func (r *DefaultRegistry) GetNSEC(key string) (string, string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if len(r.nsec) == 0 {
		return "", "" // @ -> @, empty zone
	}
	i := sort.Search(len(r.nsec), func(i int) bool { return r.nsec[i].name >= key })
	if i < len(r.nsec) && r.nsec[i].name == key {
		if i+1 == len(r.nsec) {
			return r.nsec[i].name + ".", ""
		}
		return r.nsec[i].name, r.nsec[i+1].name + "."
	}
	if i == 0 {
		return "", r.nsec[i].name + "."
	}
	// TODO(miek): do I need i + 1 == len(r.nsec) here?
	return r.nsec[i-1].name + ".", r.nsec[i].name + "."
}

func (r *DefaultRegistry) DNSSEC(b bool) bool {
	b1 := r.dnssec
	r.dnssec = b
	return b1
}

// Get retrieves a list of services from the registry that matches the given domain pattern:
//
// uuid.host.region.version.service.environment
// any of these positions may supply the wildcard "*", to have all values match in this position.
// additionally, you only need to specify as much of the domain as needed the domain version.service.environment is perfectly acceptable,
// and will assume "*" for all the ommited subdomain positions
func (r *DefaultRegistry) Get(domain string) ([]msg.Service, error) {
	// TODO: account for version wildcards
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Ensure we are using lowercase keys, as this is the way they are stored
	domain = strings.ToLower(domain)

	// DNS queries have a trailing .
	if strings.HasSuffix(domain, ".") {
		domain = domain[:len(domain)-1]
	}

	tree := dns.SplitDomainName(domain)

	// Domains can be partial, and we should assume wildcards for the unsupplied portions
	if len(tree) < 6 {
		pad := 6 - len(tree)
		t := make([]string, pad)

		for i := 0; i < pad; i++ {
			t[i] = "*"
		}

		tree = append(t, tree...)
	}
	return r.tree.get(tree)
}

// GetExpired returns a slice of expired UUIDs.
func (r *DefaultRegistry) GetExpired() (uuids []string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	now := time.Now()

	for _, n := range r.nodes {
		if !n.value.NoExpire {
			if now.After(n.value.Expires) {
				uuids = append(uuids, n.value.UUID)
			}
		}
	}

	return
}

// AddCallback adds callback c to the service s.
func (r *DefaultRegistry) AddCallback(s msg.Service, c msg.Callback) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if n, ok := r.nodes[s.UUID]; ok {
		if n.value.Callback == nil {
			n.value.Callback = make(map[string]msg.Callback)
		}
		n.value.Callback[c.UUID] = c
		return nil
	}
	return ErrNotExists
}

// Len returns the size of the registry r.
func (r *DefaultRegistry) Len() int {
	return r.tree.size()
}

type node struct {
	leaves map[string]*node
	depth  int
	length int

	value msg.Service
}

func newNode() *node {
	return &node{
		leaves: make(map[string]*node),
	}
}

func (n *node) remove(tree []string) error {
	// We are the last element, remove
	if len(tree) == 1 {
		if _, ok := n.leaves[tree[0]]; !ok {
			return ErrNotExists
		} else {
			delete(n.leaves, tree[0])
			n.length--

			return nil
		}
	}

	// Forward removal
	k := tree[len(tree)-1]
	if _, ok := n.leaves[k]; !ok {
		return ErrNotExists
	}

	var err error
	if err = n.leaves[k].remove(tree[:len(tree)-1]); err == nil {
		n.length--

		// Cleanup empty paths
		if n.leaves[k].size() == 0 {
			delete(n.leaves, k)
		}
	}

	return err
}

func (n *node) add(tree []string, s msg.Service) (*node, error) {
	// We are the last element, insert
	if len(tree) == 1 {
		if _, ok := n.leaves[tree[0]]; ok {
			return nil, ErrExists
		}

		n.leaves[tree[0]] = &node{
			value:  s,
			leaves: make(map[string]*node),
			depth:  n.depth + 1,
		}

		n.length++

		return n.leaves[tree[0]], nil
	}

	// Forward entry
	k := tree[len(tree)-1]

	if _, ok := n.leaves[k]; !ok {
		n.leaves[k] = newNode()
		n.leaves[k].depth = n.depth + 1
	}

	newNode, err := n.leaves[k].add(tree[:len(tree)-1], s)
	if err != nil {
		return nil, err
	}

	// This node length should account for all nodes below it
	n.length++
	return newNode, nil
}

func (n *node) size() int {
	return n.length
}

func (n *node) get(tree []string) (services []msg.Service, err error) {
	// We've hit the bottom
	if len(tree) == 1 {
		switch tree[0] {
		case "*":
			if len(n.leaves) == 0 {
				return services, ErrNotExists
			}

			for _, s := range n.leaves {
				s.value.UpdateTTL()

				if s.value.TTL > 1 {
					services = append(services, s.value)
				}
			}
		default:
			if _, ok := n.leaves[tree[0]]; !ok {
				return services, ErrNotExists
			}

			n.leaves[tree[0]].value.UpdateTTL()

			if n.leaves[tree[0]].value.TTL > 1 {
				services = append(services, n.leaves[tree[0]].value)
			}
		}

		return
	}

	k := tree[len(tree)-1]

	switch k {
	case "*":
		if len(n.leaves) == 0 {
			return services, ErrNotExists
		}

		var success bool
		for _, l := range n.leaves {
			if s, e := l.get(tree[:len(tree)-1]); e == nil {
				services = append(services, s...)
				success = true
			}
		}

		if !success {
			return services, ErrNotExists
		}
	default:
		if _, ok := n.leaves[k]; !ok {
			return services, ErrNotExists
		}

		return n.leaves[k].get(tree[:len(tree)-1])
	}
	return
}

func getRegistryKey(s msg.Service) string {
	return strings.ToLower(fmt.Sprintf("%s.%s.%s.%s.%s.%s", s.UUID, strings.Replace(s.Host, ".", "-", -1), s.Region, strings.Replace(s.Version, ".", "-", -1), s.Name, s.Environment))
}
