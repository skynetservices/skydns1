// Copyright (c) 2013 Erik St. Martin, Brian Ketelsen. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package msg

import (
	"time"
)

type Service struct {
	UUID        string
	Name        string
	Version     string
	Environment string
	Region      string
	Host        string
	Port        uint16
	TTL         uint32 // Seconds
	Expires     time.Time
	Callback    map[string]bool `json:"-"` // Callbacks are found by UUID
}

// Returns the amount of time remaining before expiration
func (s *Service) RemainingTTL() uint32 {
	d := s.Expires.Sub(time.Now())

	ttl := uint32(d.Seconds())

	if ttl < 1 {
		return 0
	}

	return ttl
}

// Updates TTL property to the RemainingTTL
func (s *Service) UpdateTTL() {
	s.TTL = s.RemainingTTL()
}

type Callback struct {
	UUID        string

	// Name of the service
	Name        string
	Version     string
	Environment string
	Region      string
	Host        string

	Reply	    string
	Port        uint16
}
