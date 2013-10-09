package main

import (
	"github.com/goraft/raft"
	"github.com/skynetservices/skydns/msg"
	"github.com/skynetservices/skydns/registry"
	"log"
)

// Command for adding service to registry
type AddServiceCommand struct {
	Service msg.Service
}

// Creates a new AddServiceCommand
func NewAddServiceCommand(s msg.Service) *AddServiceCommand {
	return &AddServiceCommand{s}
}

// Name of command
func (c *AddServiceCommand) CommandName() string {
	return "add-service"
}

// Adds service to registry
func (c *AddServiceCommand) Apply(server *raft.Server) (interface{}, error) {
	log.Println("Adding Service:", c.Service)

	reg := server.Context().(registry.Registry)
	err := reg.Add(c.Service)

	return c.Service, err
}

type UpdateTTLCommand struct {
	UUID string
	TTL  uint32
}

// Creates a new UpdateTTLCommand
func NewUpdateTTLCommand(uuid string, ttl uint32) *UpdateTTLCommand {
	return &UpdateTTLCommand{uuid, ttl}
}

// Name of command
func (c *UpdateTTLCommand) CommandName() string {
	return "update-ttl"
}

// Updates TTL in registry
func (c *UpdateTTLCommand) Apply(server *raft.Server) (interface{}, error) {
	log.Println("Updating Service:", c.UUID)

	reg := server.Context().(registry.Registry)
	err := reg.UpdateTTL(c.UUID, c.TTL)

	return c.UUID, err
}

type RemoveServiceCommand struct {
	UUID string
}

// Creates a new RemoveServiceCommand
func NewRemoveServiceCommand(uuid string) *RemoveServiceCommand {
	return &RemoveServiceCommand{uuid}
}

// Name of command
func (c *RemoveServiceCommand) CommandName() string {
	return "remove-service"
}

// Updates TTL in registry
func (c *RemoveServiceCommand) Apply(server *raft.Server) (interface{}, error) {
	log.Println("Removing Service:", c.UUID)

	reg := server.Context().(registry.Registry)
	err := reg.RemoveUUID(c.UUID)

	return c.UUID, err
}
