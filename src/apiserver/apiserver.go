package apiserver

import (
	"constants"
	"encoding/json"
	"io/ioutil"
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

//VoteChannel :basically an "I voted" sticker
var IVotedChannel = make(chan string)

//SendRoomChannel :pass "run" here to send update
var SendRoomUpdateChannel = make(chan string)

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
		println("[LOGIN] @@@@@@@ Calling the register user")
		registerationrequest := urllib.Post("http://" + singleServer.attachedTo + ":3000/registeruser/")
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

		println("[LOGIN] @@@@@@@ Initialise the room raft")
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
		println("[LOGIN] @@@@@@@ Calling the get room")
		roomrequest := urllib.Post("http://" + singleServer.attachedTo + ":3000/getroom/")
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
				ChancellorID:                "",
				HungCount:                   0,
				VoteResult:                  -1,
				CardPlayed:                  "",
			}
			room.RaftStore.SetRoom(gameroom.RoomID, gameroom)
			time.Sleep(3000 * time.Millisecond)
			print("The room ID was updated to ")
			println(room.RaftStore.GetRoom(gameroom.RoomID).RoomID)
			room.RaftStore.Set("RoomID", gameroom.RoomID)
		}
		//Store user data into room raft
		userdata := room.User{UserID: zmq.GetPublicIP(), NodeType: nodeType,
			Name: username, UserType: -1, Vote: -1}

		println("<-------------- Setting the user")
		room.RaftStore.SetUser(zmq.GetPublicIP(), userdata)
		println("<---------- Getting the user")
		newuserdata := room.RaftStore.GetUser(zmq.GetPublicIP())
		//Getting the room json
		println(newuserdata.Name)
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
		println("@@@@@@@@@@ <------------- Calling the get room")
		peers, _ := room.ReadPeersJSON()
		print("Number of people in current raft room are : ")
		print(len(peers))
		RoomState := raft.RaftStore.GetRoom(roomID)
		println("@@@@@@@@@@ Getting the player")
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

		println("[LOGIN] @@@@@@@ Reading the data from the players")
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

		roomrequest := urllib.Post("http://" + player["IP"] + ":8000/add_base_room/")
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

	// Get the room details
	singleServer.m.Post("/raftSuperSet", func(req *http.Request, r render.Render) {
		println("<-------------- Setting the user on the leader")
		body, _ := ioutil.ReadAll(req.Body)
		data := make(map[string]string)
		err := json.Unmarshal(body, &data)
		if err != nil {
			println("<----------- Error")
			r.Error(500)
		}
		raft.RaftStore.Set(data["key"], data["value"])
		r.JSON(http.StatusOK, "")
	})

	//allocrole : called by leader when 8 players join. Only works in leader
	singleServer.m.Post("/allocrole", func(req *http.Request, r render.Render) {
		println("Coming here!!!!!!")
		if room.RaftStore.IsLeader() == true {
			println("I AM THE LEADER ALLOCATING ROLE!!!!!")
			peers, err := room.ReadPeersJSON()
			if err != nil {
				println(err.Error())
				r.Error(500)
			}
			roles := mapset.NewSet()
			print("length of peers : ")
			println(len(peers))
			for i := 0; i < len(peers); i++ {
				println("looping in allocrole")
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
				println("reaching here to set the role for " + peers[i] + "as " + role)
				room.RaftStore.SetRole(peers[i], role)
			}
			room.RaftStore.RigElection("0", zmq.GetPublicIP())
		} else {
			println("^^^^ Waiting for my role allocation!!!!!")
			time.Sleep(3000 * time.Millisecond)
		}

		r.JSON(http.StatusOK, "")
	})

	// check if it's super node and see who its attach to
	singleServer.m.Post("/getRole", func(req *http.Request, r render.Render) {
		role := ""
		for role == "" {
			role = room.RaftStore.GetRole(zmq.GetPublicIP())
			if role == "" {
				println("-------------WAITING TO GET ROLE")
				time.Sleep(1000 * time.Millisecond)
			}
		}
		var rolejson = map[string]interface{}{"role": role}
		r.JSON(http.StatusOK, rolejson)
	})

	//isLeader : check if the node is leader in the game raft
	singleServer.m.Post("/isLeader", func(req *http.Request, r render.Render) {
		reply := "false"
		if room.RaftStore.IsLeader() == true {
			reply = "true"
		}
		var isleader = map[string]interface{}{"leader": reply}
		r.JSON(http.StatusOK, isleader)
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
	singleServer.m.Post("/getfascist", func(req *http.Request, r render.Render) {
		fascist := room.RaftStore.GetFascist()
		var fascistjson = map[string]interface{}{"fascist": fascist}
		r.JSON(http.StatusOK, fascistjson)
	})

	//----Get the identity of Hitler
	singleServer.m.Post("/gethitler", func(req *http.Request, r render.Render) {
		hitler := room.RaftStore.GetHitler()
		var hitlerjson = map[string]interface{}{"hitler": hitler}
		r.JSON(http.StatusOK, hitlerjson)
	})

	//----Get the president's identity
	singleServer.m.Post("/getpresident", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))

		var roomId string
		for key, value := range v {
			if key == "roomId" {
				roomId = value[0]
			}
		}

		president := room.RaftStore.GetPresident(roomId)
		r.JSON(http.StatusOK, map[string]interface{}{"president": president})
	})

	//----President can call this to set their choice for chancelor
	singleServer.m.Post("/setchancellor", func(req *http.Request, r render.Render) {
		println("Reaching set chancellor!")
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		userId := v["chancellor"]

		println("Setting chancellor to " + userId[0])
		room.RaftStore.SetChancellor("0", userId[0])

		println("Waiting before telling others")
		SendRoomUpdateChannel <- "run"
		r.JSON(http.StatusOK, "")
	})

	//----Draw 3 cards
	singleServer.m.Post("/drawthree", func(req *http.Request, r render.Render) {

		cards := room.RaftStore.DrawThree("0")
		println("Cards to be sent " + cards)
		r.JSON(http.StatusOK, map[string]interface{}{"card_id": cards})
	})

	//----The president can pass 2 cards to the chancellor
	singleServer.m.Post("/passtwo", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		cards := v["selected_cards"]

		room.RaftStore.PassTwo("0", cards[0])
		SendRoomUpdateChannel <- "run"
		r.JSON(http.StatusOK, "")
	})

	//----The chancellor can send in their card to play
	singleServer.m.Post("/playselected", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		card := v["selected_card"]

		room.RaftStore.PlaySelected("0", card[0])
		SendRoomUpdateChannel <- "run"
		r.JSON(http.StatusOK, "")
	})

	//----If the vote for president/chancellor fails, increment the counter
	singleServer.m.Post("/hangparlament", func(req *http.Request, r render.Render) {

		room.RaftStore.HangParlament("0")
		SendRoomUpdateChannel <- "run"
		r.JSON(http.StatusOK, "")
	})

	//----Play a random card from the top of the deck
	singleServer.m.Post("/playrandom", func(req *http.Request, r render.Render) {

		room.RaftStore.PlayRandom("0")
		SendRoomUpdateChannel <- "run"
		r.JSON(http.StatusOK, "")
	})

	//----Vote for the current president chancellor pair. 1 for no, 0 for yes
	singleServer.m.Post("/vote", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		vote := v["vote"]

		room.RaftStore.Vote(zmq.GetPublicIP(), vote[0])
		IVotedChannel <- "voted"
		r.JSON(http.StatusOK, "")
	})

	//----Investigate a user
	singleServer.m.Post("/investigaterole", func(req *http.Request, r render.Render) {

		result := room.RaftStore.InvestigateRole("0")
		SendRoomUpdateChannel <- "run"
		r.JSON(http.StatusOK, map[string]interface{}{"role": result})
	})

	//----Get the results of the last vote!
	singleServer.m.Post("/rigelection", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		userId := v["0"]

		room.RaftStore.RigElection("0", userId[0])
		SendRoomUpdateChannel <- "run"
		r.JSON(http.StatusOK, "")
	})

	//----Kill a user
	singleServer.m.Post("/killuser", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))
		userId := v["0"]

		room.RaftStore.KillUser("0", userId[0])
		SendRoomUpdateChannel <- "run"
		r.JSON(http.StatusOK, "")
	})

	//----Check for a game over condition(0 if no winner, 1 if Liberals won, 2 if fascists won)
	singleServer.m.Post("/isGameOver", func(req *http.Request, r render.Render) {

		result := room.RaftStore.IsGameOver("0")
		r.JSON(http.StatusOK, map[string]interface{}{"results": result})
	})

	//----Special Case: Check after a kill if hitler is dead
	singleServer.m.Post("/ishitlerdead", func(req *http.Request, r render.Render) {
		result := room.RaftStore.IsHitlerDead("0")
		r.JSON(http.StatusOK, map[string]interface{}{"results": result})
	})

	//----Special case: Check after a successful vote if Hitler is chancelor with 3+ Policies enacted
	singleServer.m.Post("/ishitlerchancelor", func(req *http.Request, r render.Render) {
		result := room.RaftStore.IsHitlerChancellor("0")
		r.JSON(http.StatusOK, map[string]interface{}{"results": result})
	})

	singleServer.m.Post("/ispresident", func(req *http.Request, r render.Render) {
		body, _ := ioutil.ReadAll(req.Body)
		v, _ := url.ParseQuery(string(body))

		var roomId string
		for key, value := range v {
			if key == "roomId" {
				roomId = value[0]
			}
		}

		result := room.RaftStore.IsPresident(roomId)
		r.JSON(http.StatusOK, map[string]interface{}{"isPresident": result})
	})

	//----The usual alternative to rig_election. Switches the president
        singleServer.m.Post("/switchpresident", func(req *http.Request, r render.Render) {
                room.RaftStore.SwitchPres("0")
                r.JSON(http.StatusOK, "")
        })


	//----Reset the round, setting all state flags back to starting values
	singleServer.m.Post("/resetround", func(req *http.Request, r render.Render) {
		room.RaftStore.ResetRound("0")
		SendRoomUpdateChannel <- "run"
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
		println("Setting webrtc id: " + key)
		data := room.RaftStore.GetWebrtc(key)
		if data != nil {
			data[m["set_peer"].(string)] = m["video_id"].(string)
			room.RaftStore.SetWebrtc(key, data)
		} else {
			data := make(map[string]string)
			println("Setting video ID: " + m["video_id"].(string))
			data[m["set_peer"].(string)] = m["video_id"].(string)
			room.RaftStore.SetWebrtc(key, data)
		}
		r.JSON(http.StatusOK, map[string]interface{}{"success": true})
	})

	//get the id for the handshake
	singleServer.m.Get("/getwebrtcid", func(req *http.Request, r render.Render) {
		query := req.URL.Query()
		keyvalue := query.Get("key")

		println("Getting webrtc id: " + keyvalue)
		data := room.RaftStore.GetWebrtc(keyvalue)
		if data == nil {
			r.JSON(http.StatusOK, map[string]interface{}{"success": false})
			return
		}
		r.JSON(http.StatusOK, map[string]interface{}{"success": true, "data": data})
	})

	return singleServer
}
