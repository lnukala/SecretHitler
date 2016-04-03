package backend

import (
	"constants"
	"dnsimple"
	"strings"
	"time"
	"zmq"

	"github.com/deckarep/golang-set"
)

/*CommunicationState :the state of the backend comm infra*/
type CommunicationState struct {
	PublishChannel  chan string
	SubscriptionMap map[string]map[string]bool //map of ip to map of topics
	RequestChanMap  map[string]zmq.RequestChannels
}

/*State :the state of the backend communication infra*/
var State CommunicationState

//Subscribe :method to subscribe to a particular topic on a particular ip
func Subscribe(ip string, topic string) {
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
		println(response.Geterror().Error())
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
func Promote(userid string) {
	client := dnsimple.GetClient()
	records := dnsimple.GetRecords(client)
	//If there are other supernodes in the system, get the state from them
	for _, record := range records {
		Subscribe(record.Content, "supernode")
		Request(record.Content, "promote"+constants.Delimiter+zmq.GetPublicIP())
	}
	dnsimple.AddRecord(client, userid)
}

/*Demote :demote node (ip) from supernode status*/
func Demote(ip string) {
	client := dnsimple.GetClient()
	dnsimple.DeleteRecord(client, ip)
	if strings.Compare(zmq.GetPublicIP(), ip) != 0 {
		State.PublishChannel <- "supernode" + constants.Delimiter +
			"demote" + constants.Delimiter + ip
		zmq.Supernodes.Remove(ip)         //remove from personal set of supernodes
		UnsubscribeTopic(ip, "supernode") //unsubscribe from the node for control messages
	} else {
		for supernodeIP := range zmq.Supernodes.Iter() {
			UnsubscribeTopic(supernodeIP.(string), "supernode") //unsubscribe from all supernodes
		}
		zmq.Supernodes = mapset.NewSet()
	}
}

/*Publish : publish to a topic*/
func Publish(topic string, method string, params string) {
	message := topic + constants.Delimiter + method + constants.Delimiter + params
	State.PublishChannel <- message
}
