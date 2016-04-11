package backend

import (
	"apiserver"
	"constants"
	"dnsimple"
	"fmt"
	"math/rand"
	"net/http"
	"raft"
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
			Subscribe(params, RoomState.GlobalComTopicName)
			RoomState.CurrPlayers = RoomState.CurrPlayers + ", " + params
			request := urllib.Post("http://127.0.0.1:8000/add_base_room/")
			var roomjson = map[string]interface{}{"room_id": RoomState.RoomId,
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
			request, err := request.JsonBody(roomjson)
			if err == nil {
				println(err.Error())
			} else {
				request.String()
			}
			zmq.ResponseChannel <- success
		case "raftPromote":
			raft.RaftStore.Join(params + ":5557")
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
		Promote() // tell other supernodesI'm a supernode and subscribe
		for _, superrec := range records {
			server.AddSuperNode(superrec.Content) // pass the super nodes info to APIServer
		}
		server.SetSuper(true) // I'm a supernode
		server.AddSuperNode(zmq.GetPublicIP())
		Publish("subnode", "adduser", zmq.GetPublicIP())
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

//HandleNewPlayer  : HANDLE NEW players
func HandleNewPlayer() {
	for {
		RoomState = <-apiserver.NewPlayerChannel
		players := strings.Split(RoomState.CurrPlayers, ",")
		for i := 0; i < len(players); i++ {
			Subscribe(players[i], RoomState.GlobalComTopicName)
			Request(players[i], "newPlayer"+constants.Delimiter+zmq.GetPublicIP())
		}
	}
}
