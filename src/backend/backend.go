package backend

import (
	"apiserver"
	"constants"
	"dnsimple"
	"fmt"
	"math/rand"
	"net/http"
	"raft"
	"room"
	"strconv"
	"strings"
	"time"
	"userinfo"
	"zmq"

	"github.com/GiterLab/urllib"
	"github.com/deckarep/golang-set"
)

/*CommunicationState :the state of the backend comm infra*/
type CommunicationState struct {
	PublishChannel  chan zmq.PubMessage
	SubscriptionMap map[string]map[string]bool //map of ip to map of topics
	RequestChanMap  map[string]zmq.RequestChannels
}

const success = "true"

/*State :the state of the backend communication infra*/
var State CommunicationState

/*RoomState :room of the player*/
var RoomState = raft.Room{}

//Supernodes :et of the supernodes currently in the system
var Supernodes = mapset.NewSet()

// MySuper  backend use supernode
var MySuper = "localhost"

//default state is subnode
var isSuper = false

//Subscribe :method to subscribe to a particular topic on a particular ip
func Subscribe(ip string, topic string) {
	//check if already subscribed on this ip and topic
	if State.SubscriptionMap[ip] != nil && State.SubscriptionMap[ip][topic] == true {
		println("already subscribed to " + ip + " on topic '" + topic + "'")
		return
	}
	//if not subscribes, subscribe and set the state
	go zmq.ClientSetupSUB(ip, topic)
	if State.SubscriptionMap[ip] == nil {
		topicmap := make(map[string]bool)
		topicmap[topic] = true
		State.SubscriptionMap[ip] = topicmap
	} else {
		State.SubscriptionMap[ip][topic] = true
	}
}

//UnsubscribeTopic :method to unsubscribe from a specific topic on a specific ip
func UnsubscribeTopic(ip string, topic string) {
	zmq.Unsubscribe(ip, topic)
	State.SubscriptionMap[ip][topic] = false
}

/*Request : Request a response for a message from an IP*/
func Request(ip string, message string) (string, error) {
	if State.RequestChanMap[ip] == (zmq.RequestChannels{}) {
		channel := zmq.ClientSetupREQ(ip)
		State.RequestChanMap[ip] = channel
	}
	time.Sleep(500 * time.Millisecond)
	response := zmq.SendREQ(message, State.RequestChanMap[ip])
	if response.Geterror() != nil {
		println("[Backend request]" + response.Geterror().Error())
	} else if response.Getmessage() == success {
		println("received success response!")
	} else if strings.Contains(response.Getmessage(), constants.Delimiter) == false {
		println("delimiter not present in message received in promote")
	} else {
		input := strings.SplitN(response.Getmessage(), constants.Delimiter, 2)
		zmq.Handle(input[0], input[1])
	}
	return response.Getmessage(), response.Geterror()
}

/*Promote :used to promote a node to supernode*/
func Promote() {
	client := dnsimple.GetClient()
	records := dnsimple.GetRecords(client)

	for _, record := range records {
		Supernodes.Add(record.Content)
		Request(record.Content, "promoteREQ"+constants.Delimiter+zmq.GetPublicIP()) // tell him I'm a supernode
		Subscribe(record.Content, "supernode")                                      // subscribe to "supernode" topic
		Subscribe(record.Content, "subnode")
	}
	if len(records) > 0 {
		syncFrom(records[0].Content)
	}
	dnsimple.AddRecord(client)
	Supernodes.Add(zmq.GetPublicIP())
}

// SyncFrom  pull userinfo from other supernodes
func syncFrom(ip string) {
	// TODO: sync userinfor from this server
}

/*Demote :demote node (ip) from supernode status*/
func Demote(ip string) {
	client := dnsimple.GetClient()
	dnsimple.DeleteRecord(client, ip)
	//if you are not the node being demoted
	if strings.Compare(zmq.GetPublicIP(), ip) != 0 {
		Publish("supernode", "demote", ip) //tell everyone to demote the user
		Supernodes.Remove(ip)              //remove from personal set of supernodes
		UnsubscribeTopic(ip, "supernode")  //unsubscribe from the node for control messages
	} else {
		for supernodeIP := range Supernodes.Iter() {
			UnsubscribeTopic(supernodeIP.(string), "supernode") //unsubscribe from all supernodes
		}
		Supernodes = mapset.NewSet()
	}
}

/*Publish : publish to a topic*/
func Publish(topic string, method string, params string) {
	zmsg := zmq.ZMessage{}
	zmsg.SetTag(method)
	zmsg.SetContent(params)
	pubmsg := zmq.PubMessage{}
	pubmsg.SetTopic(topic)
	pubmsg.SetMessage(zmsg)
	State.PublishChannel <- pubmsg
}

//Handle  handle all received messages
func Handle() {
	println("Someone is sending me a message!!!!!!")
	for {
		z := <-zmq.ReceiveChannel
		method := z.GetTag()
		params := z.GetContent()
		switch method {
		case "promoteREQ": // params: the IP address
			Supernodes.Add(params)
			println("promote request received for " + params)
			Subscribe(params, "supernode") // subscribe back
			Subscribe(params, "subnode")
			userinfo.AddUser(userinfo.User{UID: params, Addr: params, IsSuper: true})
			zmq.ResponseChannel <- success
		case "promoteREP":
			///TODO: Add logic for handling responses to promotion requests
		case "demote":
			Supernodes.Remove(params)
			//TODO: add logic for handling demotions requests received
		case "syncREQ":
			///TODO: send userinfo back
		case "adduser_req":
			if isSuper {
				Publish("subnode", "adduser", params)
			}
			api := apiserver.GetServer()
			api.AddUser(params)
			zmq.ResponseChannel <- success
		case "adduser":
			api := apiserver.GetServer()
			api.AddUser(params)
			zmq.ResponseChannel <- success
		case "newPlayer":
			println("<----------------- New Player being added")
			Subscribe(params, RoomState.GlobalComTopicName) //subscribe to the new node
			if room.RaftStore.IsLeader() == true {
				room.RaftStore.Join(params + ":5558") //add new node to the room raft
			}
			println("<----------- Adding to list of player:: Recieving Local variable")
			RoomState.CurrPlayers = RoomState.CurrPlayers + "," + params

			roomjson := room.RaftStore.GetRoom(strconv.Itoa(RoomState.RoomID))
			roomjson.CurrPlayers = RoomState.CurrPlayers

			println("<------------- Changing the room object")
			room.RaftStore.SetRoom(roomjson.RoomID, roomjson)

			var new_roomjson = map[string]interface{}{
				"room_id":                        RoomState.RoomID,
				"curr_players":                   RoomState.CurrPlayers,
				"global_comm_topic_name":         RoomState.GlobalComTopicName,
				"global_notification_topic_name": RoomState.GlobalNotificationTopicName,
				"president_channel":              RoomState.PresidentChannel,
				"chancellor_channel":             RoomState.ChancelorChannel,
			}

			println("<----------- Calling the update form the recieving local")
			request := urllib.Post("http://127.0.0.1:8000/update_room/")
			request, err := request.JsonBody(new_roomjson)
			if err != nil {
				println(err.Error())
			} else {
				request.String()
			}
			zmq.ResponseChannel <- success
		case "raftPromote":
			raft.RaftStore.Join(params + ":5557")
			zmq.ResponseChannel <- success
		case "updateRoom":
			updateRoom()
			zmq.ResponseChannel <- success
		case "playerVoted":
			if strings.Compare(room.RaftStore.IsPresident(strconv.Itoa(RoomState.RoomID)), "true") == 0 {
				if strings.Compare(room.RaftStore.VoteResults(strconv.Itoa(RoomState.RoomID)), constants.NoVote) != 0 {
					updateRoom()
				}
			}
			zmq.ResponseChannel <- success
		case "resetRoom":
			println("Attempting to reset the room as a player has left the room!")
			//tell the front end to stop updating
			urllib.Post("http://127.0.0.1:8000/stop_refresh/")
			//Unsubscribe from everyone on zeromq
			peers, _ := room.ReadPeersJSON()
			for _, peer := range peers {
				UnsubscribeTopic(peer, RoomState.GlobalComTopicName)
			}
			room.RaftStore.Close()
			time.Sleep(3000 * time.Millisecond)
			RoomState = raft.Room{}
			apiserver.Firstround = true
			urllib.Post("http://127.0.0.1:8000/node_relogin/")
			zmq.ResponseChannel <- success
		default:
			println("No logic added to handle this method. Please check!")
			zmq.ResponseChannel <- success
		}
	}
}

func cleanRecords() {
	client := dnsimple.GetClient()
	records := dnsimple.GetRecords(client)
	for _, r := range records {
		url := "http://" + r.Content + ":3000/heartbeat"
		fmt.Println(url)
		_, err := http.Get(url)
		if err != nil {
			fmt.Println("Deleting " + r.Content)
			r.Delete(client)
		}
	}
}

// Bootstrap  bootstrap routine
func Bootstrap(server *apiserver.APIServer) bool {
	println("$$$$$$$$ Coming to the bootstrap")
	cleanRecords()
	client := dnsimple.GetClient()
	records := dnsimple.GetRecords(client)
	fmt.Println("Existing Supernodes: " + strconv.Itoa(len(records)))
	//delete your own entry from dns in case it exists
	for _, superrec := range records {
		if superrec.Content == zmq.GetPublicIP() {
			dnsimple.DeleteRecord(client, zmq.GetPublicIP())
			records = dnsimple.GetRecords(client)
		}
	}
	if len(records) < constants.MaxSuperNumber {
		println("$$$$$$$$$$$$ I AM A SUPER NODE")
		Promote() // tell other supernodesI'm a supernode and subscribe
		for _, superrec := range records {
			server.AddSuperNode(superrec.Content) // pass the super nodes info to APIServer
		}
		server.SetSuper(true) // I'm a supernode
		println("$$$$$$$$$$$$ Sending him as a supernode")
		server.AddSuperNode(zmq.GetPublicIP())
		//Publish("subnode", "adduser", zmq.GetPublicIP())
		isSuper = true
	} else {
		rnn := rand.Int() % len(records)
		MySuper = records[rnn].Content
		Request(MySuper, "adduser_req"+constants.Delimiter+zmq.GetPublicIP()) // add user to supernode
		Subscribe(MySuper, "subnode")                                         // subscribe to "subnode" topic
		server.AttachTo(MySuper)                                              // tell apiserver I attache to a Supernode
		server.SetSuper(false)                                                // I'm a subnode
	}
	return isSuper
}

//HandleNewPlayer : Handle new players
func HandleNewPlayer() {
	for {
		RoomState = <-apiserver.NewPlayerChannel
		players := strings.Split(RoomState.CurrPlayers, ",")
		for i := 0; i < len(players); i++ {
			Subscribe(players[i], RoomState.GlobalComTopicName)
			if players[i] != zmq.GetPublicIP() {
				println("<------------------- [Backend] Sending new player request to " + players[i])
				Request(players[i], "newPlayer"+constants.Delimiter+zmq.GetPublicIP())
			}
		}
	}
}

//SendRoomUpdate : Send the updated room info to all players in the room (99.9 % President)
func SendRoomUpdate() {
	for {
		run := <-apiserver.SendRoomUpdateChannel
		println("^^^^^^^^ President got this message!!!!")
		println(run)
		roomObj := room.RaftStore.GetRoom(strconv.Itoa(RoomState.RoomID))
		println("Telling everyone to update their rooms on the channel " + roomObj.GlobalComTopicName)
		//time.Sleep(3000 * time.Millisecond)
		Publish(roomObj.GlobalComTopicName, "updateRoom", "")
		//updateRoom()
	}
}

//IVotedUpdate :Just publish to the voting channel that you voted.
func IVotedUpdate() {
	for {
		run := <-apiserver.IVotedChannel
		println(run)
		roomObj := room.RaftStore.GetRoom(strconv.Itoa(RoomState.RoomID))
		Publish(roomObj.GlobalComTopicName, "playerVoted", "")
		//----Need to check if we're the last vote
		if strings.Compare(room.RaftStore.IsPresident(strconv.Itoa(RoomState.RoomID)), "true") == 0 {
			if strings.Compare(room.RaftStore.VoteResults(strconv.Itoa(RoomState.RoomID)), constants.NoVote) != 0 {
				apiserver.SendRoomUpdateChannel <- "run"
			}
		}
	}
}

func updateRoom() {
	r, err := room.RaftStore.Get("RoomID")
	if err != nil {
		println(err.Error())
		println("Error in get")
	}
	roomObj := room.RaftStore.GetRoom(r)
	roomID, err := strconv.Atoi(roomObj.RoomID)
	if err != nil {
		println("Error!!!!!")
		println(err.Error())
	}
	var new_roomjson = map[string]interface{}{
		"room_id":                        roomID,
		"curr_players":                   roomObj.CurrPlayers,
		"global_comm_topic_name":         roomObj.GlobalComTopicName,
		"global_notification_topic_name": roomObj.GlobalNotificationTopicName,
		"no_policies_passed":             roomObj.NoPoliciesPassed,
		"fascist_policies_passed":        roomObj.FascistPoliciesPassed,
		"liberal_policies_passed":        roomObj.LiberalPoliciesPassed,
		"current_fascist_in_deck":        roomObj.CurrentFascistInDeck,
		"current_liberal_in_deck":        roomObj.CurrentLiberalInDeck,
		"current_total_in_deck":          roomObj.CurrentTotalInDeck,
		"chancellor_id":                  roomObj.ChancellorID,
		"president_id":                   roomObj.PresidentID,
		"president_channel":              roomObj.PresidentChannel,
		"chancellor_channel":             roomObj.ChancelorChannel,
		"hung_count":                     roomObj.HungCount,
		"vote_result":                    roomObj.VoteResult,
		"president_choice":               roomObj.PresidentChoice,
		"dead_list":                      roomObj.DeadList,
		"card_played":                    roomObj.CardPlayed,
	}
	request := urllib.Post("http://127.0.0.1:8000/update_room/")
	request, err = request.JsonBody(new_roomjson)
	if err != nil {
		println(err.Error())
	} else {
		request.String()
	}
}

//HeartbeatReq : Listen for heart beat requests
func HeartbeatReq() {
	for {
		IP := <-apiserver.HeartBeatRequestChannel
		go heartBeat(IP)
	}
}

//heartBeat : heartbeat the IP to check if it is still alive
func heartBeat(IP string) {
	count := 0
	for {
		if count >= 3 {
			break
		}
		url := "http://" + IP + ":3000/heartbeat"
		fmt.Println(url)
		_, err := http.Get(url)
		if err != nil {
			fmt.Println("Heart Beat failed to reach " + IP)
			count = count + 1
		} else {
			count = 0
		}
		time.Sleep(5000 * time.Millisecond)
	}
	println("*********************Quitting the room raft as player has left/is not reachable*********************")
	//tell everyone to unsubscribe from the room
	if room.RaftStore.IsLeader() == true {
		println("Detected player loss at Leader! Telling everyone to update!")
		Publish(RoomState.GlobalComTopicName, "resetRoom", "")
	}
}
