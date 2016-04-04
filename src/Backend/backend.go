package backend

import (
	"constants"
	"dnsimple"
	"strings"
	"time"
	"userinfo"
	"zmq"

	"github.com/deckarep/golang-set"
)

/*CommunicationState :the state of the backend comm infra*/
type CommunicationState struct {
	PublishChannel  chan zmq.PubMessage
	SubscriptionMap map[string]map[string]bool //map of ip to map of topics
	RequestChanMap  map[string]zmq.RequestChannels
}

/*State :the state of the backend communication infra*/
var State CommunicationState

//Supernodes :et of the supernodes currently in the system
var Supernodes = mapset.NewSet()

//Subscribe :method to subscribe to a particular topic on a particular ip
func Subscribe(ip string, topic string) {
	//check if already subscribed on this ip and topic
	if State.SubscriptionMap[ip] != nil && State.SubscriptionMap[ip][topic] == true {
		println("already subscribed to " + ip + " on topic '" + topic + "'")
		return
	}
	//if not subscribes, subscribe and set the state
	go zmq.ClientSetupSUB(ip, "topic")
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
	} else {
		if strings.Contains(message, constants.Delimiter) == false {
			println("delimiter not present in message received in promote")
		} else {
			input := strings.SplitN(message, constants.Delimiter, 2)
			zmq.Handle(input[0], input[1])
		}
	}
	return response.Getmessage(), response.Geterror()
}

/*Promote :used to promote a node to supernode*/
func Promote() {
	client := dnsimple.GetClient()
	records := dnsimple.GetRecords(client)
	//If there are other supernodes in the system, get the state from them
	for _, record := range records {
		Supernodes.Add(record.Content)
		Request(record.Content, "promoteREQ"+constants.Delimiter+zmq.GetPublicIP())
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

// Handle  handle all received messages
func Handle() {
	for {
		z := <-zmq.ReceiveChannel
		method := z.GetTag()
		params := z.GetContent()
		switch method {
		case "promoteREQ": // params: the IP address
			Supernodes.Add(params)
			Subscribe(params, "supernode")
			userinfo.AddUser(userinfo.User{UID: params, Addr: params, IsSuper: true})
		case "promoteREP":
			///TODO: Add logic for handling responses to promotion requests
		case "demote":
			Supernodes.Remove(params)
			//TODO: add logic for handling demotions requests received
		case "syncREQ":
			///TODO: send userinfo back
		default:
			println("No logic added to handle this method. Please check!")
		}
	}
}
