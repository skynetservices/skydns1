#!/usr/bin/env bash

go get github.com/goraft/raft
go get github.com/gorilla/mux
go get github.com/miekg/dns
go get github.com/skynetservices/skydns/msg

go build
