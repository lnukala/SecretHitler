package zmq

import (
	"bytes"
	"constants"
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

/*subscriptionData :tuple for managing ip and topic*/
type subscriptionData struct {
	ip    string
	topic string
}

// ZMessage  message structure
type ZMessage struct {
	tag     string
	content string
}

// GetTag  get tag of message
func (z ZMessage) GetTag() string {
	return z.tag
}

// GetContent  get content of message
func (z ZMessage) GetContent() string {
	return z.content
}

//SubscriptionState :used to keep track of the ip and topics subscribed to
var SubscriptionState = map[subscriptionData]bool{}

//ReceiveChannel  :all receivd message goes into this channel
var ReceiveChannel = make(chan ZMessage)

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
		socket.Send("ACK", 0)
		//Handle the request received
		input := strings.SplitN(msg[1], constants.Delimiter, 2)
		Handle(input[0], input[1])
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
		if strings.Contains(message, constants.Delimiter) == false {
			println("delimiter not present!")
		} else {
			entries := strings.SplitN(message, constants.Delimiter, 2)
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

/*ClientSetupSUB : Setting up the zmq client to subscribe message
  from concerned node*/
func ClientSetupSUB(ip string, topic string) {
	context, _ := zeromq.NewContext()
	socket, _ := context.NewSocket(zeromq.SUB)
	defer socket.Close()
	socket.SetRcvtimeo(500 * time.Millisecond) //wait on the receive call for .5 seconds
	if len(topic) > 0 {
		socket.SetSubscribe(topic)
	}
	socket.Connect("tcp://" + ip + ":5556")
	SubscriptionState[subscriptionData{ip: ip, topic: topic}] = true
	println("[ZMQ]Subscribed to " + ip)
	for (SubscriptionState[subscriptionData{ip: ip, topic: topic}] == true) {
		// fmt.Printf("[sub-state] IP:%s Topic:%s -> %t\n", ip, topic, SubscriptionState[subscriptionData{ip: ip, topic: topic}])
		message, err := socket.RecvMessage(0)
		if err == nil {
			topicrecv := message[0]
			messagedata := message[1]
			println("topic = " + topicrecv)
			println("message received = " + messagedata)
			input := strings.SplitN(messagedata, constants.Delimiter, 2)
			Handle(input[0], input[1])
		}
	}
}

/*Unsubscribe : ubsubscribe from a topic on an ip*/
func Unsubscribe(ip string, topic string) {
	SubscriptionState[subscriptionData{ip: ip, topic: topic}] = false
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
	return string(buf.String()[0 : len(buf.String())-1])
}

//SendREQ :Accepts mesage to be sent to the concerned servers
func SendREQ(message string, channels RequestChannels) Request {
	channels.inputchan <- message
	response := <-channels.outputchan
	return response
}

//Getmessage : return the message in the request struct
func (f *Request) Getmessage() string {
	return f.message
}

//Geterror : return the error in the request struct
func (f *Request) Geterror() error {
	return f.err
}

//Handle :Handle messages received
func Handle(method string, params string) {
	ReceiveChannel <- ZMessage{tag: method, content: params}
}
