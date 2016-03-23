package zmq

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strings"

	zeromq "github.com/pebbe/zmq4"
)

/*ServerSetup :Setting up the zmq server to receive requests and handle them*/
func ServerSetup() {
	context, _ := zeromq.NewContext()
	socket, _ := context.NewSocket(zeromq.REP)
	defer socket.Close()
	socket.SetLinger(0)
	socket.Bind("tcp://*:5555")
	// Wait for messages
	for {
		msg, _ := socket.RecvMessage(0)
		sourceIP := msg[0]
		sourceIP = strings.Replace(sourceIP, "\n", "", 1)
		print("Message received at server from ", sourceIP)
		println(", message: ", msg[1])

		// send reply back to client
		reply := fmt.Sprintf("Sorry, I dont have an answer for that")
		socket.Send(reply, 0)
	}
}

/*ClientSetup : Setting up the zmq client to send a message to concerned node*/
func ClientSetup(ip string) {
	context, _ := zeromq.NewContext()
	socket, _ := context.NewSocket(zeromq.REQ)
	socket.SetSndtimeo(50) //wait 50 milliseconds to send the message
	//defer socket.Close()

	socket.Connect("tcp://" + ip + ":5555")
	fmt.Printf("Connected to " + ip)

	msg := "Hello world!"
	//_, err := socket.Send(msg, 0)
	_, err := socket.SendMessage(GetPublicIP(), msg)
	if err != nil {
		println("Message can not be sent")
		return
	}
	println("Sending ", "hello, you there?")
	reply, _ := socket.Recv(0)
	println("Received at client ", string(reply))
}

/*func Request(ip string, message *string) {
	if socket.Send([]byte(msg), 0) == EAGAIN {
		println("Message can not be sent")
		return
	}
	println("Sending ", msg)
	reply, _ := socket.Recv(0)
	println("Received ", string(reply))
}*/

//GetPublicIP : get public ip
func GetPublicIP() string {
	resp, err := http.Get("http://myexternalip.com/raw")
	if err != nil {
		os.Stderr.WriteString(err.Error())
		os.Stderr.WriteString("\n")
		os.Exit(1)
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	return buf.String()
}
