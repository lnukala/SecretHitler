package main

import (
	api "apiserver"
	"backend"
	"raft"
	"time"
	"zmq"
)

func main() {
	s := api.GetServer()
	s.SetUID(zmq.GetPublicIP())
	s.AddUser(zmq.GetPublicIP())           // add itself
	go s.RunServer()                       // run the server in new thread
	go zmq.ServerSetupREP()                //set up server to respond to direct requests
	publishchannel := zmq.ServerSetupPUB() //Set up channel for publishing state
	channelMap := make(map[string]zmq.RequestChannels)
	submap := make(map[string]map[string]bool)
	backend.State = backend.CommunicationState{PublishChannel: publishchannel,
		SubscriptionMap: submap, RequestChanMap: channelMap}
	go backend.Handle() //set up the handler for the messages received
	backend.Bootstrap(s)

	raft := raft.New()
	raft.InitRaft()
	time.Sleep(5000 * time.Millisecond)
	raft.Set("foo", "bar")
	// Wait for committed log entry to be applied.
	time.Sleep(500 * time.Millisecond)
	value, err := raft.Get("foo")
	if err != nil {
		print(err.Error())
	}
	print(value)

	/*backend.Subscribe("127.0.0.1", "topic")
	time.Sleep(1000 * time.Millisecond)
	backend.Publish("topic", "method", "params")
	time.Sleep(1000 * time.Millisecond)
	backend.UnsubscribeTopic("127.0.0.1", "topic")
	time.Sleep(1000 * time.Millisecond)
	backend.Publish("topic", "method", "params")*/

	select {}
}
