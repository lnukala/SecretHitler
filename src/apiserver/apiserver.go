package apiserver

import (
	"constants"
	"encoding/json"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	raft "raft"
	"room"
	"strconv"
	"strings"
	"zmq"

	"github.com/GiterLab/urllib"
	"github.com/deckarep/golang-set"
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

//NewPlayerChannel :new player info is passed here
var NewPlayerChannel = make(chan raft.Room)

//RoomID : The ID of the room being returned
var roomID int

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
		var userjson = map[string]interface{}{"user_id": singleServer.uid,
			"name": username, "user_type": "", "node_type": nodeType,
			"secret_role": ""}
		var registerationjson = singleServer.uid + "," +
			username + "," + "liberal" + "," + nodeType + "," + "hitler"
		//Call the DNS to send the requet to a super node
		registerationrequest := urllib.Post("http://secrethitler.lnukala.me:3000/registeruser/")
		registerationrequest, err := registerationrequest.JsonBody(registerationjson)
		if err != nil {
			println(err.Error())
			r.Error(500)
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
		var userjson = map[string]interface{}{"user_id": userid, "name": "test_user",
			"user_type": "liberal", "node_type": "test_role", "secret_role": "hitler"}
		r.JSON(http.StatusOK, userjson)
	})

	// Register User
	singleServer.m.Post("/getroom", func(req *http.Request, r render.Render) {
		RoomState := raft.RaftStore.GetRoom(roomID)
		players := strings.Split(RoomState.CurrPlayers, ",")
		if len(players) >= constants.MaxPlayers {
			if roomID < math.MaxInt32 {
				roomID = roomID + 1
			} else {
				roomID = 0 //avoid overflow
			}
			raft.RaftStore.Delete(strconv.Itoa(roomID))
			RoomState = raft.RaftStore.GetRoom(roomID)
		}
		if RoomState.CurrPlayers != "" {
			RoomState.CurrPlayers = RoomState.CurrPlayers + "," + zmq.GetPublicIP()
		} else {
			RoomState.CurrPlayers = zmq.GetPublicIP()
		}
		println("[APISERVER] Current players in the room are " + RoomState.CurrPlayers)
		jsonObj, _ := json.Marshal(RoomState)
		roomstring := string(jsonObj)
		raft.RaftStore.Set(strconv.Itoa(roomID), roomstring)
		var roomjson = map[string]interface{}{
			"room_id":                        RoomState.RoomID,
			"curr_players":                   RoomState.CurrPlayers,
			"global_comm_topic_name":         RoomState.GlobalComTopicName,
			"global_notification_topic_name": RoomState.GlobalNotificationTopicName,
			"no_of_policies_passed":          RoomState.NoPoliciesPassed,
			"fascist_policies_passed":        RoomState.FascistPolciesPassed,
			"liberal_policies_passed":        RoomState.LiberalPoliciesPassed,
			"current_fascist_in_deck":        RoomState.CurrentFascistInDeck,
			"current_liberal_in_deck":        RoomState.CurrentLiberalInDeck,
			"current_total_in_deck":          RoomState.CurrentTotalInDeck,
			"chancellor_id":                  RoomState.ChancellorID,
			"president_id":                   RoomState.PresidentID,
			"president_channel":              RoomState.PresidentChannel,
			"chancellor_channel":             RoomState.ChancelorChannel,
			"hitler_id":                      RoomState.HitlerID}

		//Getting the room json and calling the update
		print("Calling the room info update method!!")
		roomrequest := urllib.Post("http://127.0.0.1:8000/add_base_room/")
		roomrequest, err2 := roomrequest.JsonBody(roomjson)
		if err2 != nil {
			println(err2.Error())
			r.Error(500)
		}
		roomrequest.String()

		//Initialize raft for the game that you are about to join
		if room.RaftStore != nil {
			room.RaftStore.Close() //If there is a session currently, close it
		}
		room.RaftStore = room.New()
		err := room.RaftStore.InitRoomRaft()
		if err != nil {
			println(err.Error())
			r.Error(500)
		}

		//calling the method to tell others you have joined
		NewPlayerChannel <- RoomState

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

	//allocrole : called by leader when 8 players join. Only works in leader
	singleServer.m.Get("/allocrole", func(args martini.Params, r render.Render) {
		if room.RaftStore.IsLeader() == true {
			peers, err := room.ReadPeersJSON()
			if err != nil {
				println(err.Error())
				r.Error(500)
			}
			roles := mapset.NewSet()
			for i := 0; i < len(peers); i++ {
				number := rand.Intn(constants.MaxPlayers)
				for roles.Contains(number) == true {
					number = rand.Intn(constants.MaxPlayers) //pick a unique number
				}
				roles.Add(number)
				role := ""
				switch {
				case number >= 0 && number <= 4:
					role = "Liberal"
				case number >= 5 && number <= 6:
					role = "Fascist"
			        case number == 7:
					role = "Hitler"
				default:
					println("Control shouldn't reach here. Error")
					r.Error(500)
				}
				room.RaftStore.SetRole(peers[i], role)
			}
		}
		r.JSON(http.StatusOK, "")
	})

	// check if it's super node and see who it attach to
	singleServer.m.Get("/getRole", func(args martini.Params, r render.Render) {
		role := room.RaftStore.GetRole(zmq.GetPublicIP())
		r.JSON(http.StatusOK, map[string]interface{}{"role": role})
	})

	//isLeader : check if the node is leader in the game raft
	singleServer.m.Get("/isLeader", func(args martini.Params, r render.Render) {
		reply := "false"
		if room.RaftStore.IsLeader() == true {
			reply = "true"
		}
		r.JSON(http.StatusOK, map[string]interface{}{"leader": reply})
	})

	// check if it's super node and see who it attach to
	singleServer.m.Get("/issuper", func(args martini.Params, r render.Render) {
		r.JSON(http.StatusOK, map[string]interface{}{"super": singleServer.super,
			"attach_to": singleServer.attachedTo})
	})

	// superlist  see the super node list
	singleServer.m.Get("/superlist", func(args martini.Params, r render.Render) {
		r.JSON(http.StatusOK, map[string]interface{}{"superlist": singleServer.superList})
	})

	// userlist  see the userlist
	singleServer.m.Get("/userlist", func(args martini.Params, r render.Render) {
		r.JSON(http.StatusOK, map[string]interface{}{"userlist": singleServer.userList})
	})

	//----Begin gameplay logic

	//----Get the identity of the other fascist in the game
	singleServer.m.Get("/getfascist", func(args martini.Params, r render.Render) {
		fascist := room.RaftStore.GetFascist()
		r.JSON(http.StatusOK, map[string]interface{}{"fascist": fascist})
	})

	//----Get the identity of Hitler
        singleServer.m.Get("/gethitler", func(args martini.Params, r render.Render) {
                hitler := room.RaftStore.GetHitler()
                r.JSON(http.StatusOK, map[string]interface{}{"hitler": hitler})
        })

	return singleServer
}
