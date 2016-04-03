package main

import (
	api "apiserver"
	"backend"
	"time"
	"zmq"
)

func main() {
	s := api.GetServer()
	go s.RunServer()                       // run the server in new thread
	go zmq.ServerSetupREP()                //set up server to respond to direct requests
	publishchannel := zmq.ServerSetupPUB() //Set up channel for publishing state
	channelMap := make(map[string]zmq.RequestChannels)
	submap := make(map[string]map[string]bool)
	backend.State = backend.CommunicationState{PublishChannel: publishchannel,
		SubscriptionMap: submap, RequestChanMap: channelMap}

	backend.Subscribe("127.0.0.1", "topic")
	time.Sleep(1000 * time.Millisecond)
	backend.Publish("topic", "method", "params")
	time.Sleep(1000 * time.Millisecond)
	backend.UnsubscribeTopic("127.0.0.1", "topic")
	time.Sleep(1000 * time.Millisecond)
	backend.Publish("topic", "method", "params")

	time.Sleep(100000 * time.Millisecond)
}
