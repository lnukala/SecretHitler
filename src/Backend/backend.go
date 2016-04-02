package main

import (
	api "apiserver"
	"time"
	"zmq"
)

/*CommunicationState :the state of the backend comm infra*/
type CommunicationState struct {
	PublishChannel  chan string
	SubscriptionMap map[string]map[string]bool //map of ip to map of topics
	RequestChanMap  map[string]zmq.RequestChannels
}

/*State :the state of the backend communication infra*/
var State CommunicationState

func main() {
	s := api.GetServer()
	go s.RunServer()                       // run the server in new thread
	go zmq.ServerSetupREP()                //set up server to respond to direct requests
	publishchannel := zmq.ServerSetupPUB() //Set up channel for publishing state
	channelMap := make(map[string]zmq.RequestChannels)
	submap := make(map[string]map[string]bool)

	State = CommunicationState{PublishChannel: publishchannel,
		SubscriptionMap: submap, RequestChanMap: channelMap}

	subscribe("127.0.0.1", "topic")
	time.Sleep(1000 * time.Millisecond)
	publishchannel <- "topic" + zmq.Delimiter + "hello1"
	publishchannel <- "topic" + zmq.Delimiter + "hello2"
	time.Sleep(1000 * time.Millisecond)
	unsubscribeTopic("127.0.0.1", "topic")
	time.Sleep(1000 * time.Millisecond)
	publishchannel <- "topic" + zmq.Delimiter + "hello3"

	/*chans := zmq.ClientSetupREQ("127.0.0.1")
	time.Sleep(1000 * time.Millisecond)
	response := zmq.SendREQ("test", chans)
	print(response.Getmessage())*/
	time.Sleep(100000 * time.Millisecond)
}

//subscribe :method to subscribe to a particular topic on a particular ip
func subscribe(ip string, topic string) {
	//check if already subscribed on this ip and topic
	if State.SubscriptionMap[ip] != nil && State.SubscriptionMap[ip][topic] == true {
		println("already subscribed to " + ip + " on topic '" + topic + "'")
		return
	}
	//if not subscribes, subscribe and set the state
	go zmq.ClientSetupSUB("127.0.0.1", "topic")
	if State.SubscriptionMap[ip] == nil {
		topicmap := make(map[string]bool)
		topicmap[topic] = true
		State.SubscriptionMap[ip] = topicmap
	} else {
		State.SubscriptionMap[ip][topic] = true
	}
}

//method to unsubscribe from a specific topic on a specific ip
func unsubscribeTopic(ip string, topic string) {
	zmq.Unsubscribe(ip, topic)
	State.SubscriptionMap[ip][topic] = false
}
