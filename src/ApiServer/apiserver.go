package apiserver

import (
	"encoding/json"
	"net/http"
	"room"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
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

	singleServer = &APIServer{state: 0, m: martini.Classic()} // initialize a server
	singleServer.m.Use(render.Renderer())

	// heartbeet  returns StatusOK with a string
	singleServer.m.Get("/heartbeet", func() string {
		return "I'm alive"
	})

	// login  return json data including the succes information
	singleServer.m.Get("/login", func(args martini.Params, r render.Render) {

		r.JSON(http.StatusOK, map[string]interface{}{"success": true})
	})

	// rooms  list all the rooms
	singleServer.m.Get("/rooms", func(args martini.Params) string {
		mem1 := room.Member{Name: "gavin", Addr: [4]byte{100, 2, 3, 4}}
		mem2 := room.Member{Name: "vikram", Addr: [4]byte{101, 200, 3, 4}}
		room1 := room.Room{}
		room1.SetAttr(mem1, []room.Member{mem1, mem2})

		mem3 := room.Member{Name: "leela", Addr: [4]byte{103, 2, 3, 4}}
		mem4 := room.Member{Name: "nick", Addr: [4]byte{104, 200, 3, 4}}
		room2 := room.Room{}
		room2.SetAttr(mem3, []room.Member{mem3, mem4})

		list := [2]room.Room{room1, room2}

		res, _ := json.Marshal(list)
		return string(res)
	})

	// login  return json data including the succes information
	singleServer.m.Get("/myrole", func(args martini.Params, r render.Render) {
		r.JSON(http.StatusOK, map[string]interface{}{"role": "chancellor"})
	})

	return singleServer
}
