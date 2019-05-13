/*
 * Based on https://github.com/grpc/grpc-go/blob/master/examples/helloworld/greeter_server/main.go
 * Copyright 2015 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"google.golang.org/grpc"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"

	handler "github/com/bakins/grpc-transode-handler"
)

const (
	port = "127.0.0.1:9090"
)

type server struct{}

// SayHello implements helloworld.GreeterServer
func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("Received: %v", in.Name)
	return &pb.HelloReply{Message: "Hello " + in.Name}, nil
}

func main() {
	h, err := handler.New(handler.WithErrorLogger(simpleLogger{}))
	if err != nil {
		log.Fatalf("failed to create transcode handler: %v", err)
	}

	svr := &server{}
	g := grpc.NewServer()

	// Register service handler on both grpc servers
	pb.RegisterGreeterServer(g, svr)
	pb.RegisterGreeterServer(h.Server(), svr)

	go func() {
		if err := h.Start(); err != nil {
			log.Fatalf("failed to start internal proxy: %v", err)
		}
	}()

	http.Handle("/", handle(g, h))
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func handle(g *grpc.Server, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			g.ServeHTTP(w, r)
		} else {
			h.ServeHTTP(w, r)
		}
	})
}

type simpleLogger struct{}

func (s simpleLogger) Log(message string, err error) {
	fmt.Printf("%s: %v", message, err)
}
