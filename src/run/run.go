package main

import (
	api "apiserver"
	"backend"
	"constants"
	"dnsimple"
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
	client := dnsimple.GetClient()
	for _, rd := range dnsimple.GetRecords(client) {
		// TODO: Ping each ip:3000/heartbeet
		// remove dead node from dns
		println("[run] subscribe to super node " + rd.Content)
		backend.Subscribe(rd.Content, "subnode")
	}

	records := dnsimple.GetRecords(client)
	if len(records) < constants.MaxSuperNumber {
		backend.Promote()
		s.SetSuper(true)
	}
	backend.Handle()
}
