package apiserver

import (
	"encoding/json"
	"net/http"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
)

// APIServer : server struct
type APIServer struct {
	m     *martini.ClassicMartini
	state int
	super bool
}

var singleServer *APIServer

// RunServer : start the server
func (s *APIServer) RunServer() {
	s.m.Run()
}

// SetSuper : set the node to be supernode or subnode
func (s *APIServer) SetSuper(isSuper bool) {
	s.super = isSuper
}

// GetServer : return a http server
func GetServer() *APIServer {
	if singleServer != nil {
		return singleServer
	}

	singleServer = &APIServer{state: 0, m: martini.Classic()} // initialize a server
	singleServer.m.Use(render.Renderer())

	// heartbeet  returns StatusOK with a string
	singleServer.m.Get("/heartbeat", func() string {
		return "I'm alive"
	})

	// login  return json data including the succes information
	singleServer.m.Get("/login", func(args martini.Params, r render.Render) {
		r.JSON(http.StatusOK, map[string]interface{}{"success": true})
	})

	// rooms  list all the rooms
	singleServer.m.Get("/rooms", func(args martini.Params) string {
		res, _ := json.Marshal([]int{1, 2, 3, 4})
		return string(res)
	})

	// login  return json data including the succes information
	singleServer.m.Get("/myrole", func(args martini.Params, r render.Render) {
		r.JSON(http.StatusOK, map[string]interface{}{"role": "chancellor"})
	})

	// login  return json data including the succes information
	singleServer.m.Get("/issuper", func(args martini.Params, r render.Render) {
		r.JSON(http.StatusOK, map[string]interface{}{"super": singleServer.super})
	})

	return singleServer
}
