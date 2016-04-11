package apiserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	raft "raft"

	"github.com/GiterLab/urllib"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
)

// APIServer : server struct
type APIServer struct {
	m          *martini.ClassicMartini
	state      int
	super      bool
	superList  []string
	userList   []string
	attachedTo string
	uid        string
}

var singleServer *APIServer

//RaftStore : Global variable for store
var RaftStore *raft.Store

//NewPlayerChannel :new player info is passed here
var NewPlayerChannel = make(chan raft.Room)

// RunServer : start the server
func (s *APIServer) RunServer() {
	s.m.Run()
}

// SetSuper : set the node to be supernode or subnode
func (s *APIServer) SetSuper(isSuper bool) {
	s.super = isSuper
}

// SetUID  : set user id
func (s *APIServer) SetUID(userid string) {
	s.uid = userid
}

// AddSuperNode   add one super node to the list
func (s *APIServer) AddSuperNode(superip string) {
	s.superList = append(s.superList, superip)
}

// AttachTo   add one super node to the list
func (s *APIServer) AttachTo(superip string) {
	s.attachedTo = superip
}

// AddUser   add a user
func (s *APIServer) AddUser(uid string) {
	s.userList = append(s.userList, uid)
}

// NotifyGet  use this method to send notification to front end. Paramemter in param map
// @hook the url
// @param parameters
func NotifyGet(hook string, param map[string]string) {
	front := "http://localhost:8000/"
	url := front + hook + "?"
	for k, v := range param {
		url += k + "=" + v + "&"
	}
	http.Get(url)
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
	singleServer.m.Post("/login", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		var username string
		for key, value := range v {
			if key == "username" {
				username = value[0]
			}
		}

		var nodeType string
		if singleServer.super == true {
			nodeType = "supernode"
		} else {
			nodeType = "subnode"
		}

		//Creating the user json
		var userjson = map[string]interface{}{"user_id": singleServer.uid, "name": username, "user_type": "liberal", "node_type": nodeType, "secret_role": "hitler"}
		var registerationjson = singleServer.uid + "," + username + "," + "liberal" + "," + nodeType + "," + "hitler"
		//Call the DNS to send the requet to a super node
		registerationrequest := urllib.Post("http://secrethitler.lnukala.me:3000/registeruser/")
		registerationrequest, err := registerationrequest.JsonBody(registerationjson)
		if err != nil {
		}
		registerationrequest.String()

		//Getting the room json and calling the update
		print("Calling the get room method!!")
		roomrequest := urllib.Post("http://secrethitler.lnukala.me:3000/getroom/")
		roomrequest.String()
		//Getting the room json and calling the update
		r.JSON(http.StatusOK, userjson)
	})

	// rooms  list all the rooms
	singleServer.m.Post("/getuser", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		var userid string
		for key, value := range v {
			if key == "userid" {
				userid = value[0]
			}
		}
		print("User id queries - " + userid)
		//Get the user details from gavins method and return to front end
		var userjson = map[string]interface{}{"user_id": userid, "name": "test_user", "user_type": "liberal", "node_type": "test_role", "secret_role": "hitler"}
		r.JSON(http.StatusOK, userjson)
	})

	// Register User
	singleServer.m.Post("/getroom", func(req *http.Request, r render.Render) {
		print("Getting the room details!!!!!!!!!!")
		RoomState := raft.RaftStore.GetRoom(0)
		print(RoomState.CurrPlayers)
		// var roomjson = map[string]interface{}{
		// 	"room_id":                        RoomState.RoomId,
		// 	"curr_players":                   RoomState.CurrPlayers,
		// 	"global_comm_topic_name":         "coms",
		// 	"global_notification_topic_name": "notifications",
		// 	"no_of_policies_passed":          0,
		// 	"fascist_policies_passed":        0,
		// 	"liberal_policies_passed":        0,
		// 	"current_fascist_in_deck":        11,
		// 	"current_liberal_in_deck":        6,
		// 	"current_total_in_deck":          17,
		// 	"chancellor_id":                  -1,
		// 	"president_id":                   -1,
		// 	"president_channel":              "pres",
		// 	"chancellor_channel":             "chan",
		// 	"hitler_id":                      -1}

		var roomjson = map[string]interface{}{
			"room_id":                        RoomState.RoomId,
			"curr_players":                   RoomState.CurrPlayers,
			"global_comm_topic_name":         RoomState.GlobalComTopicName,
			"global_notification_topic_name": RoomState.GlobalNotificationTopicName,
			"no_of_policies_passed":          RoomState.NoPoliciesPassed,
			"fascist_policies_passed":        RoomState.FascistPolciesPassed,
			"liberal_policies_passed":        RoomState.LiberalPoliciesPassed,
			"current_fascist_in_deck":        RoomState.CurrentFascistInDeck,
			"current_liberal_in_deck":        RoomState.CurrentLiberalInDeck,
			"current_total_in_deck":          RoomState.CurrentTotalInDeck,
			"chancellor_id":                  RoomState.ChancellorId,
			"president_id":                   RoomState.PresidentId,
			"president_channel":              RoomState.PresidentChannel,
			"chancellor_channel":             RoomState.ChancelorChannel,
			"hitler_id":                      RoomState.HitlerId}

		//Getting the room json and calling the update
		print("Calling the room info update method!!")
		roomrequest := urllib.Post("http://127.0.0.1:8000/add_base_room/")
		roomrequest, err2 := roomrequest.JsonBody(roomjson)
		if err2 != nil {
		}
		roomrequest.String()

		//calling the method to tell others you have joined
		NewPlayerChannel <- RoomState

		//calling the method to tell others you have joined
		//backend.NewPlayer(RoomState)

		r.JSON(http.StatusOK, "")
	})

	// Get the room details
	singleServer.m.Post("/registeruser", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		var userjson string
		for key := range v {
			print(key)
			userjson = key
		}
		raft.RaftStore.StoreUser(userjson)
		r.JSON(http.StatusOK, "")
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

	// check if it's super node and see who it attach to
	singleServer.m.Get("/issuper", func(args martini.Params, r render.Render) {
		r.JSON(http.StatusOK, map[string]interface{}{"super": singleServer.super, "attach_to": singleServer.attachedTo})
	})

	// superlist  see the super node list
	singleServer.m.Get("/superlist", func(args martini.Params, r render.Render) {
		r.JSON(http.StatusOK, map[string]interface{}{"superlist": singleServer.superList})
	})

	// userlist  see the userlist
	singleServer.m.Get("/userlist", func(args martini.Params, r render.Render) {
		r.JSON(http.StatusOK, map[string]interface{}{"userlist": singleServer.userList})
	})

	return singleServer
}
