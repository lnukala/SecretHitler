package main

import (
	api "apiserver"
	"dnsimple"
	"sync"
	"time"
	"zmq"
)

func main() {

	s := api.GetServer()
	go s.RunServer() // run the server in new thread

	client := dnsimple.GetClient()
	dnsimple.PrintDomains(client)
	dnsimple.GetRecords(client)
	dnsimple.AddRecord(client, "test")
	for {
		if rec := dnsimple.GetRecords(client); len(rec) == 0 {
			time.Sleep(30 * 1000 * time.Millisecond)
		} else {
			break
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go zmq.ClientSetupSUB("127.0.0.1", "topic")
	channel := zmq.ServerSetupPUB()
	time.Sleep(1000 * time.Millisecond)
	channel <- "topic" + zmq.Delimiter + "hello"

	go zmq.ServerSetupREP()
	chans := zmq.ClientSetupREQ("127.0.0.1")
	time.Sleep(1000 * time.Millisecond)
	response := zmq.SendREQ("test", chans)
	print(response.Getmessage())
	wg.Wait()
}
