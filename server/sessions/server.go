// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sessions

import (
	"log"
	"net/http"

	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/mux"
	predis "github.com/markmandel/paddle-soccer/server/pkg/redis"
	"k8s.io/client-go/kubernetes"
)

// Version is the current api version number
const Version string = "sessions:0.3"

// Server is the http server instance
type Server struct {
	srv  *http.Server
	pool *redis.Pool
	cs   kubernetes.Interface
	// the game server image
	gameServerImage string
}

// handler is the extended http.HandleFunc to provide context for this application
type handler func(*Server, http.ResponseWriter, *http.Request) error

// NewServer returns the HTTP Server instance
func NewServer(hostAddr, redisAddr string, image string) *Server {
	if redisAddr == "" {
		redisAddr = ":6379"
	}

	log.Printf("[Info][Server] Starting server version %v on port %v", Version, hostAddr)
	log.Printf("[Info][Server] Connecting to Redis at %v", redisAddr)

	s := &Server{gameServerImage: image, pool: predis.NewPool(redisAddr)}

	r := s.newHandler()

	s.srv = &http.Server{
		Handler: r,
		Addr:    hostAddr,
	}

	return s
}

// Start starts the HTTP server
func (s *Server) Start() error {
	err := predis.WaitForConnection(s.pool)
	if err != nil {
		log.Printf("[Error][Server] Could not connect to redis: %v", err)
		return err
	}

	s.cs, err = clientSet()
	if err != nil {
		log.Printf("[Error][Server] Could not connect to kubernetes api: %v", err)
		return err
	}

	err = s.srv.ListenAndServe()
	if err != nil {
		log.Fatalf("[Error][Server] Error starting server: %v", err)
		return err
	}

	return nil
}

// newHandler returns the http routes for this application
func (s *Server) newHandler() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/register", s.wrapMiddleware(registerHandler)).Methods("POST")
	r.HandleFunc("/session/{id}", s.wrapMiddleware(getHandler)).Methods("GET")
	r.HandleFunc("/session", s.wrapMiddleware(createHandler)).Methods("POST")

	return r
}

// wrapMiddleware returns a http.HandleFunc
// wrapped in standard middleware
func (s *Server) wrapMiddleware(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := h(s, w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
