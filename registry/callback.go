package registry

import (
	"errors"
	"fmt"
	"github.com/skynetservices/skydns/msg"
	"strings"
	"sync"
	"time"
)

// Creates a new CallbackRegistry
func NewCallback() Registry {
	return &CallRegistry{
		tree:  newNode(),
		nodes: make(map[string]*node),
	}
}

// Datastore for registered callbacks
type CallbackRegistry struct {
	tree  *node
	nodes map[string]*node
	mutex sync.Mutex
}

// Remove callback specified by UUID
func (r *CallbackRegistry) RemoveUUID(uuid string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if n, ok := r.nodes[uuid]; ok {
		return r.removeService(n.value)
	}

	return ErrNotExists
}

// Retrieve a callback based on it's UUID
func (r *CallbackRegistry) GetUUID(uuid string) (s msg.Service, err error) {
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

func (r *CallbackRegistry) Len() int { return r.tree.size() }

// no-ops to implement the interface
func (r *CallbackRegistry) Add(s msg.Service) error                                    { return nil }
func (r *CallbackRegistry) UpdateTTL(uuid string, ttl uint32, expires time.Time) error { return nil }
func (r *CallbackRegistry) Remove(s msg.Service) (err error)                           { return nil }
func (r *CallbackRegistry) Get(domain string) ([]msg.Service, error)                   { return nil, nil }
func (r *CallbackRegistry) GetExpired() (uuids []string)                               {}
func (r *CallbackRegistry) AddCallback(s msg.Service, uuid string) error               { return nil }
func (r *CallbackRegistry) RemoveCallback(s msg.Service, uuid string) error            { return nil }

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
		case "all", "any":
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
	case "all", "any":
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
