package main

import (
	"sync"
	"time"
	"zmq"

	"github.com/go-martini/martini"
)

func main() {
	m := martini.Classic()
	m.Get("/", func() string {
		return "Hello world!"
	})
	//m.Run()
	var wg sync.WaitGroup
	wg.Add(2)
	/*m.Run()*/
	go zmq.ClientSetupSUB("127.0.0.1", "topic")
	channel := zmq.ServerSetupPUB()
	time.Sleep(1000 * time.Millisecond)
	channel <- "topic!@#$%%$#@!hello"
	wg.Wait()
}
