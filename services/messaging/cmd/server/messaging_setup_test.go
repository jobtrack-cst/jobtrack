package main

import (
	"google.golang.org/grpc"
	"testing"
)

func TestRegisterMessages_RegistersBothPublicMethods(t *testing.T) {
	server := grpc.NewServer()
	defer server.Stop()
	registerMessages(nil, nil)(server)
	info, ok := server.GetServiceInfo()["jobtrack.messaging.v1.MessagingService"]
	if !ok {
		t.Fatal("messaging service not registered")
	}
	methods := map[string]bool{}
	for _, method := range info.Methods {
		methods[method.Name] = true
	}
	if len(methods) != 2 || !methods["SendMessage"] || !methods["ListMessages"] {
		t.Fatalf("unexpected methods: %v", methods)
	}
}
