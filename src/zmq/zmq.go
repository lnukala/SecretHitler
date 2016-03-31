package zmq

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	zeromq "github.com/pebbe/zmq4"
)

/*Request :returning both the message data and the error*/
type Request struct {
	message string
	err     error
}

/*RequestChannels :struct containing the input and output channels tuple*/
type RequestChannels struct {
	inputchan  chan string
	outputchan chan Request
}

//Delimiter :used to separate the topic and the message data received in the
//channel
const Delimiter = "!@#$%%$#@!"

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
	for {
		message := <-channel
		if strings.Contains(message, Delimiter) == false {
			println("delimiter not present!")
		} else {
			entries := strings.Split(message, Delimiter)
			topic := entries[0]
			messagedata := entries[1]
			println("topic = " + topic + " and message = " + messagedata)
			_, err := socket.SendMessage(topic, messagedata)
			if err != nil {
				println("unable to send message " + messagedata + " error = " + err.Error())
			}
		}
	}
}

/*ClientSetupREQ : Setting up the zmq client to send a message
  to concerned node*/
func ClientSetupREQ(ip string) RequestChannels {
	context, _ := zeromq.NewContext()
	socket, _ := context.NewSocket(zeromq.REQ)
	socket.SetSndtimeo(50) //wait 50 milliseconds to send the message
	//defer socket.Close()
	socket.Connect("tcp://" + ip + ":5555")
	fmt.Printf("Connected to " + ip)

	requestChannel := make(chan string)
	returnChannel := make(chan Request)
	go receiveRequest(requestChannel, returnChannel, socket)
	return RequestChannels{inputchan: requestChannel, outputchan: returnChannel}
}

/* receiveRequest : handle incoming requests on the channel concerned*/
func receiveRequest(channelReceive chan string, channelReturn chan Request,
	socket *zeromq.Socket) {
	defer socket.Close()
	socket.SetRcvtimeo(5000 * time.Millisecond)
	for {
		msg := <-channelReceive
		_, err := socket.SendMessage(GetPublicIP(), msg)
		if err != nil {
			println("Message can not be sent")
			channelReturn <- Request{message: "", err: err}
		}
		println("Sending " + msg)
		reply, err := socket.Recv(0)
		if err != nil {
			println("Did not receive response from server. " + err.Error())
			channelReturn <- Request{message: reply, err: err}
		}
		//println("Received at client ", string(reply))
		channelReturn <- Request{message: reply, err: err}
	}
}

/*ClientSetupSUB : Setting up the zmq client to send a message
  to concerned node*/
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

//SendREQ :Accepts mesage to be sent to the concerned servers
func SendREQ(message string, channels RequestChannels) Request {
	channels.inputchan <- message
	response := <-channels.outputchan
	return response
}

//GetREQmessage : return the message in the request struct
func (f *Request) GetREQmessage() string {
	return f.message
}

//GetREQerror : return the error in the request struct
func (f *Request) GetREQerror() error {
	return f.err
}
