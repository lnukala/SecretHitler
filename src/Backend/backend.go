package main

import (
	"sync"
	"zmq"

	"github.com/go-martini/martini"
)

func main() {
	m := martini.Classic()
	m.Get("/", func() string {
		return "Hello world!"
	})
	var wg sync.WaitGroup
	wg.Add(2)
	/*m.Run()*/
	go zmq.ServerSetup()
	go zmq.ClientSetup("127.0.0.1")
	wg.Wait()
}
