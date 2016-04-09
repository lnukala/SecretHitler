package main

import (
	api "apiserver"
	"backend"
	raft "raft"
	"time"
	"zmq"
)

// RaftStore : Global variable

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
	isSuper := backend.Bootstrap(s)

	//----TODO Integrate GetRoom as necessry
	if isSuper { //----We only set up sn stuff if we're a sn
		raft.RaftStore = raft.New()
		raft.RaftStore.InitRaft()
		//api.RaftStore = RaftStore
		//room := raft.GetRoom(0) //TODO This is the magical room id, should probably get changed at some point
	}

	time.Sleep(1000 * time.Millisecond)

	backend.Subscribe("127.0.0.1", "topic")
	time.Sleep(1000 * time.Millisecond)
	backend.Publish("topic", "method", "params")
	time.Sleep(1000 * time.Millisecond)
	backend.UnsubscribeTopic("127.0.0.1", "topic")
	time.Sleep(1000 * time.Millisecond)
	backend.Publish("topic", "method", "params")

	select {}
}
