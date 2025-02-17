package raft

import (
	"bytes"
	"dnsimple"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"zmq"

	"github.com/GiterLab/urllib"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/raft-boltdb"
)

const (
	retainSnapshotCount = 2
	raftTimeout         = 10 * time.Second
)

// Store is a simple key-value store, where all changes are made via Raft consensus.
type Store struct {
	mu     sync.Mutex
	m      map[string]string // The key-value store for the system.
	raft   *raft.Raft        // The consensus mechanism
	logger *log.Logger
}

type command struct {
	Op    string `json:"op,omitempty"`
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

//Room : represents the room structure to store room state
type Room struct {
	RoomID                      int
	CurrPlayers                 string
	GlobalComTopicName          string
	GlobalNotificationTopicName string
	PresidentChannel            string
	ChancelorChannel            string
}

//User : structure respresenting the user information stored
type User struct {
	UserID     string
	Name       string
	UserType   string
	NodeType   string
	SecretRole string
}

//RaftStore : Global variable exposed
var RaftStore *Store

//New : returns a new Store.
func New() *Store {
	return &Store{
		m:      make(map[string]string),
		logger: log.New(os.Stderr, "[store] ", log.LstdFlags),
	}
}

//InitRaft : initialize raft
func (s *Store) InitRaft() error {
	// Setup Raft configuration.
	config := raft.DefaultConfig()

	// Check for any existing peers.
	client := dnsimple.GetClient()
	dnsimple.PrintDomains(client)
	records := dnsimple.GetRecords(client)

	// Allow the node to entry single-mode, potentially electing itself, if
	// explicitly enabled and there is only 1 node in the cluster already.
	if len(records) <= 2 {
		config.EnableSingleNode = true
		config.DisableBootstrapAfterElect = false
	}

	//Define the port and ip that raft will bind on
	raftbind := ":5557"

	// Setup Raft communication.
	addr, err := net.ResolveTCPAddr("tcp", zmq.GetPublicIP()+raftbind)
	if err != nil {
		return err
	}
	transport, err := raft.NewTCPTransport(raftbind, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return err
	}

	// Create peer storage.
	peerStore := raft.NewJSONPeers("raftdb", transport)

	// Create the snapshot store. This allows the Raft to truncate the log.
	snapshots, err := raft.NewFileSnapshotStore("raftdb", retainSnapshotCount, os.Stderr)
	if err != nil {
		return fmt.Errorf("file snapshot store: %s", err)
	}

	// Create the log store and stable store.
	logStore, err := raftboltdb.NewBoltStore(filepath.Join("raftdb", "raft.db"))
	if err != nil {
		return fmt.Errorf("new bolt store: %s", err)
	}

	// Instantiate the Raft systems.
	ra, err := raft.NewRaft(config, (*fsm)(s), logStore, logStore, snapshots, peerStore, transport)
	if err != nil {
		return fmt.Errorf("new raft: %s", err)
	}
	s.raft = ra
	return nil
}

// Get returns the value for the given key.
func (s *Store) Get(key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[key], nil
}

// Set sets the value for the given key.
func (s *Store) Set(key string, value string) error {
	if s.raft.State() != raft.Leader {
		println("<---------- Setting the MAIN RAFT!!!!")
		println("<----------- Setting on the leader!!!!!")
		leader_ip := strings.Split(s.raft.Leader(), ":")
		println("SENDING REQUEST TO " + leader_ip[0])
		roomrequest := urllib.Post("http://" + leader_ip[0] + ":3000/raftSuperSet/")
		roomjson := make(map[string]string)
		roomjson["key"] = key
		roomjson["value"] = value
		roomrequest, err2 := roomrequest.JsonBody(roomjson)
		if err2 != nil {
			println("<----------- Not reachable!!!!")
			return fmt.Errorf("not rechable")
		}
		roomrequest.String()
		return fmt.Errorf("not leader")
	} else {
		print("setting on local ! ")
		println(zmq.GetPublicIP())
	}
	c := &command{
		Op:    "set",
		Key:   key,
		Value: value,
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}

	f := s.raft.Apply(b, raftTimeout)
	if err, ok := f.(error); ok {
		return err
	}
	return nil
}

// Delete deletes the given key.
func (s *Store) Delete(key string) error {
	if s.raft.State() != raft.Leader {
		return fmt.Errorf("not leader")
	}
	c := &command{
		Op:  "delete",
		Key: key,
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	f := s.raft.Apply(b, raftTimeout)
	if err, ok := f.(error); ok {
		return err
	}
	return nil
}

// Join :joins a node, located at addr, to this store. The node must be ready to
// respond to Raft communications at that address.
func (s *Store) Join(addr string) error {
	s.logger.Printf("received join request for remote node as %s", addr)

	f := s.raft.AddPeer(addr)
	if f.Error() != nil {
		return f.Error()
	}
	s.logger.Printf("node at %s joined successfully", addr)
	return nil
}

// Leave removes a node, located at addr, to this store. The state clean up has
// to be done by the node itself
func (s *Store) Leave(addr string) error {
	s.logger.Printf("received removal request for remote node as %s", addr)

	f := s.raft.RemovePeer(addr)
	if f.Error() != nil {
		return f.Error()
	}
	s.logger.Printf("node at %s removed succesfully", addr)
	return nil
}

type fsm Store

// Apply : applies a Raft log entry to the key-value store.
func (f *fsm) Apply(l *raft.Log) interface{} {
	var c command
	if err := json.Unmarshal(l.Data, &c); err != nil {
		panic(fmt.Sprintf("failed to unmarshal command: %s", err.Error()))
	}
	switch c.Op {
	case "set":
		return f.applySet(c.Key, c.Value)
	case "delete":
		return f.applyDelete(c.Key)
	default:
		panic(fmt.Sprintf("unrecognized command op: %s", c.Op))
	}
}

// Snapshot returns a snapshot of the key-value store.
func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Clone the map.
	o := make(map[string]string)
	for k, v := range f.m {
		o[k] = v
	}

	return &fsmSnapshot{store: o}, nil
}

// Restore stores the key-value store to a previous state.
func (f *fsm) Restore(rc io.ReadCloser) error {
	o := make(map[string]string)
	if err := json.NewDecoder(rc).Decode(&o); err != nil {
		return err
	}

	// Set the state from the snapshot, no lock required according to
	// Hashicorp docs.
	f.m = o
	return nil
}

func (f *fsm) applySet(key string, value string) interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m[key] = value
	return nil
}

func (f *fsm) applyDelete(key string) interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.m, key)
	return nil
}

type fsmSnapshot struct {
	store map[string]string
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		// Encode data.
		b, err := json.Marshal(f.store)
		if err != nil {
			return err
		}
		// Write data to sink.
		if _, err := sink.Write(b); err != nil {
			return err
		}
		// Close the sink.
		if err := sink.Close(); err != nil {
			return err
		}
		return nil
	}()
	if err != nil {
		sink.Cancel()
		return err
	}
	return nil
}

func (f *fsmSnapshot) Release() {}

/*GetRoom : Get our room object if able, or create it if it doesn't exist*/
func (s *Store) GetRoom(roomID int) Room {
	var roomResponse Room
	roomString := strconv.Itoa(roomID)
	response, _ := s.Get(roomString)

	if len(response) == 0 {
		room := Room{roomID, "", "coms", "notifications", "pres", "chan"}
		jsonObj, _ := json.Marshal(room)
		response = string(jsonObj)
		s.Set(roomString, response)
		return room
	}
	byteResponse := []byte(response)
	json.Unmarshal(byteResponse, &roomResponse)
	return roomResponse
}

/*StoreUser :Pass in a CSV object, change to struct, then store it!
* Returns true if successful*/
func (s *Store) StoreUser(passedObj string) {
	shortObj := passedObj[1 : len(passedObj)-1]
	tokenArray := strings.Split(shortObj, ",")
	user := User{tokenArray[0], tokenArray[1], tokenArray[2], tokenArray[3], tokenArray[4]}
	jsonObj, _ := json.Marshal(user)
	stringObj := string(jsonObj)
	s.Set(tokenArray[0], stringObj)
}

// GetUser Get user from raft store
func (s *Store) GetUser(userID string) []byte {
	var byteResponse []byte
	response, _ := s.Get(userID)
	byteResponse = []byte(response)
	return byteResponse
}

//ReadPeersJSON :read the peers in the game
func ReadPeersJSON() ([]string, error) {
	b, err := ioutil.ReadFile("roomdb/peers.json")
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if len(b) == 0 {
		return nil, nil
	}

	var peers []string
	dec := json.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(&peers); err != nil {
		return nil, err
	}

	return peers, nil
}

//IsLeader :read the peers in the game
func (s *Store) IsLeader() bool {
	if s.raft.Leader() == zmq.GetPublicIP() {
		return true
	}
	return false
}
