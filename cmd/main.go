package main

import (
	"context"

	"github.com/code-sigs/go-box/pkg/box"
)

type HelloRequest struct {
	Name string `json:"name"`
}

type HelloResponse struct {
	Message string `json:"message"`
}

func helloGRPC(ctx context.Context, req *HelloRequest) (*HelloResponse, error) {
	return &HelloResponse{Message: "Hello, " + req.Name}, nil
}

func main() {
	b := box.New()
	b.Router.Register("/hello", helloGRPC)
	b.Router.Run(":8080")
}
