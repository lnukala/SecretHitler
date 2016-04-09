package raft

import (
	"dnsimple"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
	"zmq"

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

type Room struct {
	RoomId                      int
	CurrPlayers                 string
	GlobalComTopicName          string
	GlobalNotificationTopicName string
	NoPoliciesPassed            int
	FascistPolciesPassed        int
	LiberalPoliciesPassed       int
	CurrentFascistInDeck        int
	CurrentLiberalInDeck        int
	CurrentTotalInDeck          int
	ChancellorId                int
	PresidentId                 int
	PresidentChannel            string
	ChancelorChannel            string
	HitlerId                    int
}

type User struct {
	UserId   int
	Name     string
	UserType string
	NodeType string
	SecretRole string
}

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
	if len(records) <= 1 {
		config.EnableSingleNode = true
		config.DisableBootstrapAfterElect = false
	}

	//Define the port and ip that raft will bind on
	raftbind := "127.0.0.1:5557"

	// Setup Raft communication.
	addr, err := net.ResolveTCPAddr("tcp", raftbind)
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
		return fmt.Errorf("not leader")
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

// Join joins a node, located at addr, to this store. The node must be ready to
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

/**
* Get our room object if able, or create it if it doesn't exist
 */
func (s *Store) GetRoom(roomId int) string {

	roomString := strconv.Itoa(roomId)
	response, err := s.Get(roomString)

	if err != nil {
		room := Room{roomId, zmq.GetPublicIP(), "coms", "notifications", 0, 0, 0, 11, 6, 17, -1, -1, "pres", "chan", -1}
		jsonObj, _ := json.Marshal(room)
		stringObj := string(jsonObj)
		s.Set(roomString, stringObj)
		return stringObj
	}

	return response

}

/**
* Pass in a JSON object, change to struct, then store it!
* Returns true if successful
*/
func (s *Store) StoreUser(passedObj map[string]interface{}) {

	var uid int
	var name string
	var userType string
	var nodeType string
	var secretRole string

        uid = passedObj["user_id"].(int)
	name = passedObj["name"].(string)
	userType = passedObj["user_type"].(string)
	nodeType = passedObj["node_type"].(string)
	secretRole = passedObj["secret_role"].(string)

	user := User{uid, name, userType, nodeType, secretRole}

        jsonObj, _ := json.Marshal(user)
	stringId := strconv.Itoa(uid)
	stringObj := string(jsonObj)
        s.Set(stringId, stringObj)

}

// GetUser Get user from raft store
func (s *Store) GetUser(userId int) []byte {

	var byteResponse []byte
	intId := strconv.Itoa(userId)
	response, _ := s.Get(intId)
	byteResponse = []byte(response)
	return byteResponse
}
