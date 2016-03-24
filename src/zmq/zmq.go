package zmq

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strings"

	zeromq "github.com/pebbe/zmq4"
)

/*ServerSetupREP :Setting up the zmq server to receive requests
  and handle them*/
func ServerSetupREP() {
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

/*ServerSetupPUB :Setting up the zmq publish server that returns a channel to
  write to */
func ServerSetupPUB() chan string {
	context, _ := zeromq.NewContext()
	socket, _ := context.NewSocket(zeromq.PUB)
	socket.Bind("tcp://*:5556")
	pubChannel := make(chan string)
	go publishMessage(pubChannel, socket)
	return pubChannel
}

/*publishMessage : Listen on a channel and send the message that is received*/
func publishMessage(channel chan string, socket *zeromq.Socket) {
	defer socket.Close()
	messageDelimiter := "!@#$%%$#@!"
	for {
		message := <-channel
		entries := strings.Split(message, messageDelimiter)
		topic := entries[0]
		messagedata := entries[1]
		println("topic = " + topic + " and message = " + messagedata)
		_, err := socket.SendMessage(topic, messagedata)
		if err != nil {
			print("unable to send message " + messagedata)
		}
	}
}

/*ClientSetupREQ : Setting up the zmq client to send a message to concerned node*/
func ClientSetupREQ(ip string) {
	context, _ := zeromq.NewContext()
	socket, _ := context.NewSocket(zeromq.REQ)
	socket.SetSndtimeo(50) //wait 50 milliseconds to send the message
	socket.SetIdentity("username")
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

/*ClientSetupSUB : Setting up the zmq client to send a message to concerned node*/
func ClientSetupSUB(ip string, topic string) {
	context, _ := zeromq.NewContext()
	socket, _ := context.NewSocket(zeromq.SUB)
	defer socket.Close()
	if len(topic) > 0 {
		socket.SetSubscribe(topic)
	}
	//defer socket.Close()
	socket.Connect("tcp://" + ip + ":5556")
	println("Subscribed to " + ip)

	for {
		message, _ := socket.RecvMessage(0)
		topicrecv := message[0]
		messagedata := message[1]
		println("topic = " + topicrecv)
		println("message received = " + messagedata)
	}
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

//GetPublicIP : get public ip of your machine
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
