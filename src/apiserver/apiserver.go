package apiserver

import (
	"constants"
	"encoding/json"
	"io/ioutil"
	"json"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"raft"
	"room"
	"strconv"
	"strings"
	"time"
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
			"name": username, "user_type": -1, "node_type": nodeType,
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

		//Initialize raft for the game that you are about to join
		if room.RaftStore != nil {
			room.RaftStore.Close() //If there is a session currently, close it
		}

		room.RaftStore = room.New()
		err = room.RaftStore.InitRoomRaft()
		if err != nil {
			println(err.Error())
		}
		time.Sleep(3000 * time.Millisecond)

		println("<----------- Calling the get room")

		//Getting the room json
		player := make(map[string]string)
		player["IP"] = zmq.GetPublicIP()
		roomrequest := urllib.Post("http://secrethitler.lnukala.me:3000/getroom/")
		roomrequest, err = roomrequest.JsonBody(player)
		if err != nil {
			println(err.Error())
			r.Error(500)
		}
		bytes, reqerr := roomrequest.Bytes()
		if reqerr != nil {
			println(reqerr.Error())
		}
		roomstate := raft.Room{}
		json.Unmarshal(bytes, &roomstate)
		time.Sleep(3000 * time.Millisecond)
		println("<----------- Passing it to the others")
		//calling the method to tell others you have joined
		NewPlayerChannel <- roomstate
		time.Sleep(3000 * time.Millisecond)

		//check if there are peers in the room raft, if not, store the room state
		peers, err := room.ReadPeersJSON()
		if err != nil {
			println(err.Error())
			r.Error(500)
		}
		if len(peers) == 1 {
			println("First player in the room. Initializing room in the room raft")
			gameroom := room.Room{
				RoomID:                      strconv.Itoa(roomstate.RoomID),
				CurrPlayers:                 roomstate.CurrPlayers,
				GlobalComTopicName:          roomstate.GlobalComTopicName,
				GlobalNotificationTopicName: roomstate.GlobalNotificationTopicName,
				NoPoliciesPassed:            0,
				FascistPoliciesPassed:       0,
				LiberalPoliciesPassed:       0,
				CurrentFascistInDeck:        11,
				CurrentLiberalInDeck:        6,
				CurrentTotalInDeck:          17,
				HungCount:                   0,
			}
			room.RaftStore.SetRoom(gameroom.RoomID, gameroom)
		}
		//Store user data into room raft
		userdata := room.User{UserID: zmq.GetPublicIP(), NodeType: nodeType,
			Name: username}

		println("<-------------- Setting the user")
		room.RaftStore.SetUser(zmq.GetPublicIP(), userdata)
		println("<---------- Getting the user")
		new_userdata := room.RaftStore.GetUser(zmq.GetPublicIP())
		//Getting the room json
		println(new_userdata.Name)
		println("<------------------------")
		r.JSON(http.StatusOK, userjson)
	})

	// rooms list all the rooms
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
		userdata := room.RaftStore.GetUser(userid)
		println("<---------- Getting the user")
		println(userdata.Name)
		println("<------------------------")

		var userjson = map[string]interface{}{"user_id": userdata.UserID,
			"name": userdata.Name, "user_type": userdata.UserType, "node_type": userdata.NodeType,
			"secret_role": userdata.SecretRole}

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
		//read the data from the player and add to the list of players stored
		player := make(map[string]string)
		body, _ := ioutil.ReadAll(req.Body)
		err := json.Unmarshal(body, &player)
		if err != nil {
			r.Error(500)
		}
		if RoomState.CurrPlayers != "" {
			RoomState.CurrPlayers = RoomState.CurrPlayers + "," + player["IP"]
		} else {
			RoomState.CurrPlayers = player["IP"]
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
			"president_channel":              RoomState.PresidentChannel,
			"chancellor_channel":             RoomState.ChancelorChannel,
		}
		//Getting the room json and calling the update
		roomrequest := urllib.Post("http://127.0.0.1:8000/add_base_room/")
		roomrequest, err2 := roomrequest.JsonBody(roomjson)
		if err2 != nil {
			println(err2.Error())
			r.Error(500)
		}
		roomrequest.String()
		r.JSON(http.StatusOK, RoomState)
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

	// Get the room details
	singleServer.m.Post("/raftset", func(req *http.Request, r render.Render) {
		println("<-------------- Setting the user on the leader")
		body, _ := ioutil.ReadAll(req.Body)
		data := make(map[string]string)
		err := json.Unmarshal(body, &data)
		if err != nil {
			println("<----------- Error")
			r.Error(500)
		}
		room.RaftStore.Set(data["key"], data["value"])
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

	// check if it's super node and see who its attach to
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
	singleServer.m.Get("/playerlist", func(args martini.Params, r render.Render) {
		peers, err := room.ReadPeersJSON()
		if err != nil {
			r.Error(500)
		}
		r.JSON(http.StatusOK, map[string]interface{}{"players": peers})
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

	//----Get the president's identity
	singleServer.m.Get("/getpresident", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		roomId := v["0"]

		president := room.RaftStore.GetPresident(roomId[0])
		r.JSON(http.StatusOK, map[string]interface{}{"president": president})
	})

	//----Set the president role to the next logical president
	singleServer.m.Get("switchpresident", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		roomId := v["0"]

		room.RaftStore.SwitchPres(roomId[0])
		r.JSON(http.StatusOK, "")
	})

	//----President can call this to set their choice for chancelor
	singleServer.m.Get("/setchancellor", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		roomId := v["0"]
		userId := v["1"]

		room.RaftStore.SetChancellor(roomId[0], userId[0])
		r.JSON(http.StatusOK, "")
	})

	//----Draw 3 cards
	singleServer.m.Get("/drawthree", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		roomId := v["0"]

		cards := room.RaftStore.DrawThree(roomId[0])
		r.JSON(http.StatusOK, map[string]interface{}{"cards": cards})
	})

	//----The president can pass 2 cards to the chancellor
	singleServer.m.Get("/passtwo", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		roomId := v["0"]
		cards := v["1"]

		room.RaftStore.PassTwo(roomId[0], cards[0])
		r.JSON(http.StatusOK, "")
	})

	//----The chancellor can send in their card to play
	singleServer.m.Get("/playselected", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		roomId := v["0"]
		card := v["1"]

		room.RaftStore.PlaySelected(roomId[0], card[0])
		r.JSON(http.StatusOK, "")
	})

	//----If the vote for president/chancellor fails, increment the counter
	singleServer.m.Get("/hangparlament", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		roomId := v["0"]

		count := room.RaftStore.GetPresident(roomId[0])
		r.JSON(http.StatusOK, map[string]interface{}{"count": count})
	})

	//----Play a random card from the top of the deck
	singleServer.m.Get("/playrandom", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		roomId := v["0"]

		room.RaftStore.PlayRandom(roomId[0])
		r.JSON(http.StatusOK, "")
	})

	//----Vote for the current president chancellor pair. 1 for no, 0 for yes
	singleServer.m.Get("/vote", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		userId := v["0"]
		vote := v["1"]

		room.RaftStore.Vote(userId[0], vote[0])
		r.JSON(http.StatusOK, "")
	})

	//----Get the results of the last vote!
	singleServer.m.Get("/voteresults", func(args martini.Params, r render.Render) {

		result := room.RaftStore.VoteResults()
		r.JSON(http.StatusOK, map[string]interface{}{"results": result})
	})

	//----Investigate a user
	singleServer.m.Get("/investigaterole", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		userId := v["0"]

		result := room.RaftStore.InvestigateRole(userId[0])
		r.JSON(http.StatusOK, map[string]interface{}{"role": result})
	})

	//----Get the results of the last vote!
	singleServer.m.Get("/rigelection", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		roomId := v["0"]
		userId := v["1"]

		room.RaftStore.RigElection(roomId[0], userId[0])
		r.JSON(http.StatusOK, "")
	})

	//----Kill a user
	singleServer.m.Get("/killuser", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		userId := v["0"]

		room.RaftStore.KillUser(userId[0])
		r.JSON(http.StatusOK, "")
	})

	//----Check for a game over condition(0 if no winner, 1 if Liberals won, 2 if fascists won)
	singleServer.m.Get("/isGameOver", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		roomId := v["0"]

		result := room.RaftStore.IsGameOver(roomId[0])
		r.JSON(http.StatusOK, map[string]interface{}{"results": result})
	})

	//----Special Case: Check after a kill if hitler is dead
	singleServer.m.Get("/ishitlerdead", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		roomId := v["0"]

		result := room.RaftStore.IsHitlerDead(roomId[0])
		r.JSON(http.StatusOK, map[string]interface{}{"results": result})
	})

	//----Special case: Check after a successful vote if Hitler is chancelor with 3+ Policies enacted
	singleServer.m.Get("/ishitlerchancelor", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		roomId := v["0"]

		result := room.RaftStore.IsHitlerChancellor(roomId[0])
		r.JSON(http.StatusOK, map[string]interface{}{"results": result})
	})

	//----Special case: Check after a successful vote if Hitler is chancelor with 3+ Policies enacted
	singleServer.m.Get("/ispresident", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		roomId := v["0"]

		result := room.RaftStore.IsPresident(roomId[0])
		r.JSON(http.StatusOK, map[string]interface{}{"isPresident": result})
	})
	//----wait for a particlular string to be sent over the channel, then return
	singleServer.m.Get("/wait", func(req *http.Request, r render.Render) {
		r.JSON(http.StatusOK, "")
	})

	//----set the webrtc id for the handshake
	singleServer.m.Post("/setwebrtcid", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		var f interface{}
		err := json.Unmarshal(body, &f)
		if err != nil {
			r.Error(500)
		}
		m := f.(map[string]interface{})
		name1 := m["peer_a"].(string)
		name2 := m["peer_b"].(string)
		compare := strings.Compare(name1, name2)
		//Generate the key for the pair
		key := ""
		if compare < 0 {
			key = name1 + name2
		} else {
			key = name2 + name1
		}
		//check if the data for the key already exists
		data := room.RaftStore.GetWebrtc(key)
		if data != nil {
			data[m["set_peer"].(string)] = m["video_id"].(string)
			room.RaftStore.SetWebrtc(key, data)
		} else {
			data := make(map[string]string)
			data[m["set_peer"].(string)] = m["video_id"].(string)
			room.RaftStore.SetWebrtc(key, data)
		}
		r.JSON(http.StatusOK, map[string]interface{}{"success": true})
	})

	//get the id for the handshake
	singleServer.m.Get("/getwebrtcid", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, err := url.ParseQuery(string(body))
		if err != nil {
			r.Error(500)
		}
		//get the key
		var keyvalue string
		for key, value := range v {
			if key == "key" {
				keyvalue = value[0]
			}
		}
		data := room.RaftStore.GetWebrtc(keyvalue)
		if data == nil {
			r.JSON(http.StatusOK, map[string]interface{}{"success": false})
		}
		r.JSON(http.StatusOK, map[string]interface{}{"success": true,
			"data": data})
	})

	return singleServer
}
