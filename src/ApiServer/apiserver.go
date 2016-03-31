package apiserver

import (
	"github.com/go-martini/martini"
)

// APIServer : server struct
type APIServer struct {
	m     *martini.ClassicMartini
	state int
}

var singleServer *APIServer

// RunServer : start the server
func (s *APIServer) RunServer() {
	s.m.Run()
}

// GetServer : return a http server
func GetServer() *APIServer {
	if singleServer != nil {
		return singleServer
	}

	singleServer = &APIServer{state: 0}
	singleServer.m = martini.Classic()
	singleServer.m.Get("/", func() string {
		return "Hello world!"
	})

	return singleServer
}
