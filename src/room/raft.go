package room

import (
	"bytes"
	"constants"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

//Room : represents the room structure to store room state
type Room struct {
	RoomID                      string
	CurrPlayers                 string
	GlobalComTopicName          string
	GlobalNotificationTopicName string
	NoPoliciesPassed            int
	FascistPoliciesPassed       int
	LiberalPoliciesPassed       int
	CurrentFascistInDeck        int
	CurrentLiberalInDeck        int
	CurrentTotalInDeck          int
	ChancellorID                string
	PresidentID                 string
	PresidentChannel            string
	ChancelorChannel            string
	HitlerID                    string
	HungCount                   int
	PresidentChoice             string
}

//User : structure respresenting the user information stored
type User struct {
	UserID     string
	Name       string
	UserType   string
	NodeType   string
	SecretRole string
	Vote       int
	IsDead     bool
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

//InitRoomRaft : initialize raft
func (s *Store) InitRoomRaft() error {
	// Setup Raft configuration.
	config := raft.DefaultConfig()

	// Allow the node to entry single-mode, potentially electing itself, if
	// explicitly enabled and there is only 1 node in the cluster already.
	config.EnableSingleNode = true
	config.DisableBootstrapAfterElect = false

	//Define the port and ip that raft will bind on
	raftbind := ":5558"

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
	peerStore := raft.NewJSONPeers("roomdb", transport)

	// Create the snapshot store. This allows the Raft to truncate the log.
	snapshots, err := raft.NewFileSnapshotStore("roomdb", retainSnapshotCount, os.Stderr)
	if err != nil {
		return fmt.Errorf("file snapshot store: %s", err)
	}

	// Create the log store and stable store.
	logStore, err := raftboltdb.NewBoltStore(filepath.Join("roomdb", "raft.db"))
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
	if val, ok := s.m[key]; ok {
		return val, nil
	}
	return "", nil
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

//Apply : applies a Raft log entry to the key-value store.
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

//Persist : persist the data
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

//Close : shoutdown the raft session running for the game
func (s *Store) Close() {
	os.RemoveAll("roomdb") //delete the directory being used to store the data
	s.raft.Shutdown()      //shut down the current raft session for the room
}

//ReadPeersJSON :read the peers in the game
func ReadPeersJSON() ([]string, error) {
	b, err := ioutil.ReadFile("roomdb/peers.json")
	if err != nil {
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

//GetUser :Get user from raft store
func (s *Store) GetUser(userID string) User {
	var user User

	response, _ := s.Get(userID)
	byteResponse := []byte(response)
	json.Unmarshal(byteResponse, &user)

	return user
}

//SetUser : set user details in room raft
func (s *Store) SetUser(userID string, user User) {

	byteUser, _ := json.Marshal(user)
	stringUser := string(byteUser)
	s.Set(userID, stringUser)
}

//SetRoom : Convenience method: update room info at back
func (s *Store) SetRoom(RoomID string, room Room) {
	byteRoom, _ := json.Marshal(room)
	stringRoom := string(byteRoom)
	s.Set(RoomID, stringRoom)
}

//GetRoom : get room details
func (s *Store) GetRoom(RoomID string) Room {
	var room Room
	stringRoom, _ := s.Get(RoomID)
	byteRoom := []byte(stringRoom)
	json.Unmarshal(byteRoom, &room)
	return room
}

//SetRole : Give a user the specified role
func (s *Store) SetRole(peer string, role string) {
	user := s.GetUser(peer)
	user.SecretRole = role
	s.SetUser(peer, user)
}

//GetRole : Return your role in the game
func (s *Store) GetRole(name string) string {
	user := s.GetUser(name)
	return user.SecretRole
}

//GetFascist : Return the identity of your fascist ally
func (s *Store) GetFascist() string {

	peerList, _ := ReadPeersJSON()
	for _, peer := range peerList {
		user := s.GetUser(peer)
		if strings.Compare(user.SecretRole, "Fascist") == 0 &&
			strings.Compare(user.UserID, zmq.GetPublicIP()) != 0 {
			return user.UserID
		}
	}
	//----SOMETHING IS WRONG
	return ""
}

//GetHitler : Return the identity of hitler
func (s *Store) GetHitler() string {

	peerList, _ := ReadPeersJSON()
	for _, peer := range peerList {
		user := s.GetUser(peer)
		if strings.Compare(user.SecretRole, "Hitler") == 0 {
			return user.UserID
		}
	}
	//----SOMETHING IS WRONG
	return ""
}

//SwitchPres ----President switches at begining of every round
func (s *Store) SwitchPres(RoomID string) {

	room := s.GetRoom(RoomID)
	stringArray, _ := ReadPeersJSON()

	if strings.Compare(room.PresidentID, "") == 0 {
		room.PresidentID = stringArray[0]
	} else {
		i := 0
		noMatch := true

		for noMatch {
			if strings.Compare(room.PresidentID, stringArray[i]) == 0 {
				noMatch = false
			}
			i++
		}
		if i == 8 {
			i = 0
		}

		//----Need to make sure that we don't elect a dead player
		user := s.GetUser(stringArray[i])
		for user.IsDead {
			i++
			if i == 8 {
				i = 0
			}
			user = s.GetUser(stringArray[i])
		}
		room.PresidentID = stringArray[i]
	}
	s.SetRoom(RoomID, room)
}

//GetPresident ----Get the President's UID
func (s *Store) GetPresident(RoomID string) string {
	room := s.GetRoom(RoomID)

	return room.PresidentID
}

//SetChancellor ----Set a chancelor post-election
func (s *Store) SetChancellor(RoomID string, chanID string) {
	room := s.GetRoom(RoomID)

	room.ChancellorID = chanID

	s.SetRoom(RoomID, room)
}

//GetChancellor ----Get the chancellor's UID
func (s *Store) GetChancellor(RoomID string) string {
	room := s.GetRoom(RoomID)

	return room.ChancellorID
}

//DrawThree : Draw 3 cards from the deck
func (s *Store) DrawThree(RoomID string) string {

	var out string

	out = ""

	room := s.GetRoom(RoomID)

	if room.CurrentTotalInDeck < 3 {
		room.CurrentTotalInDeck = 17 - room.FascistPoliciesPassed - room.LiberalPoliciesPassed
		room.CurrentLiberalInDeck = 6 - room.LiberalPoliciesPassed
		room.CurrentFascistInDeck = 11 - room.FascistPoliciesPassed
	}

	roll := rand.Intn(room.CurrentTotalInDeck)

	for i := 0; i < 3; i++ {
		if roll < room.CurrentLiberalInDeck {
			room.CurrentLiberalInDeck--
			room.CurrentTotalInDeck--
			out += "Liberal"
		} else {
			room.CurrentFascistInDeck--
			room.CurrentTotalInDeck--
			out += "Fascist"
		}
		if i != 2 {
			out += ","
		}
	}
	s.SetRoom(RoomID, room)
	return out
}

//PassTwo :
func (s *Store) PassTwo(RoomID string, choice string) {

	room := s.GetRoom(RoomID)

	room.PresidentChoice = choice

	s.SetRoom(RoomID, room)
}

//HangParliament ----Update the hung parliament counter. Return the count
func (s *Store) HangParliament(RoomID string) string {
	room := s.GetRoom(RoomID)
	room.HungCount++

	if room.HungCount == 3 {
		room.HungCount = 0
	}

	s.SetRoom(RoomID, room)

	if room.HungCount == 0 {
		return "3"
	}
	return strconv.Itoa(room.HungCount)
}

//PlayRandom -----After hung parliament x3: play a random card off the deck
func (s *Store) PlayRandom(RoomID string) {

	room := s.GetRoom(RoomID)

	if room.CurrentTotalInDeck < 1 {
		room.CurrentTotalInDeck = 17 - room.FascistPoliciesPassed - room.LiberalPoliciesPassed
		room.CurrentLiberalInDeck = 6 - room.LiberalPoliciesPassed
		room.CurrentFascistInDeck = 11 - room.FascistPoliciesPassed
	}

	roll := rand.Intn(room.CurrentTotalInDeck)

	//----Rolled a liberal
	if roll < room.CurrentLiberalInDeck {
		room.CurrentLiberalInDeck--
		room.CurrentTotalInDeck--
		room.LiberalPoliciesPassed++
	} else {
		//----Otherwise rolled a fascist
		room.CurrentFascistInDeck--
		room.CurrentTotalInDeck--
		room.FascistPoliciesPassed++
	}

	s.SetRoom(RoomID, room)
}

//PlaySelected ----Chancelor: Pass down the selected card
func (s *Store) PlaySelected(RoomID string, card string) {
	room := s.GetRoom(RoomID)

	if strings.Compare(card, "Liberal") == 0 {
		room.LiberalPoliciesPassed++
	} else {
		room.FascistPoliciesPassed++
	}

	s.SetRoom(RoomID, room)
}

//Vote ----Set your vote for the president chancelor pair(0 is YA, 1 is NEIN)
func (s *Store) Vote(userID string, vote string) {
	user := s.GetUser(userID)

	intVote, _ := strconv.Atoi(vote)

	user.Vote = intVote

	s.SetUser(userID, user)
}

//VoteResults ----Returns the results: 1 is NEIN, 0 is YA
func (s *Store) VoteResults() string {
	var count int

	userList, _ := ReadPeersJSON()
	count = 0
	deadCount := s.DeadCount()

	for _, userString := range userList {
		user := s.GetUser(userString)
		count += user.Vote
	}
	if deadCount == 2 {
		if count >= 3 {
			return constants.Nein
		}
		return constants.Ya
	}
	if count >= 4 {
		return constants.Nein
	}
	return constants.Ya
}

//DeadCount ---Count the number of dead users, to calculate voting
//concensus for no, and how many votes to wait for.
func (s *Store) DeadCount() int {

	count := 0
	users, _ := ReadPeersJSON()
	for _, userString := range users {
		user := s.GetUser(userString)
		if user.IsDead {
			count++
		}
	}
	return count
}

//InvestigateRole ----Fascist Power: Get a player's party
//affiliation(Liberal or Fascist)
func (s *Store) InvestigateRole(userID string) string {
	user := s.GetUser(userID)

	if strings.Compare(user.SecretRole, "Liberal") == 0 {
		return "Liberal"
	}
	return "Fascist"
}

//RigElection ----Fascist Power: Set the next presidental choice
func (s *Store) RigElection(RoomID string, userID string) {
	room := s.GetRoom(userID)

	room.PresidentID = userID

	s.SetRoom(RoomID, room)
}

//KillUser ----Fascist Power: Kill a user, they no longer act in game.
func (s *Store) KillUser(userID string) {
	user := s.GetUser(userID)

	user.IsDead = true

	s.SetUser(userID, user)
}

//IsGameOver ----Check for if the game is over in normal
//circumstances: 0 is not over, 1 is Liberal, 2 is fascist
func (s *Store) IsGameOver(RoomID string) string {
	room := s.GetRoom(RoomID)

	//----Liberal win #1: 5 liberal policies
	if room.LiberalPoliciesPassed == 5 {
		return constants.LiberalsWin
	}
	//----Fascist win #1: 6 fascist policies
	if room.FascistPoliciesPassed == 6 {
		return constants.FascistsWin
	}
	return constants.InProgress
}

//----Hitler Special Case wins!

//IsHitlerDead ----To be called after a kill resolves: check to see if the liberals won
func (s *Store) IsHitlerDead(userID string) string {
	user := s.GetUser(userID)
	if user.IsDead && strings.Compare(user.SecretRole, "Hitler") == 0 {
		return constants.LiberalsWin
	}
	return constants.InProgress
}

//IsHitlerChancellor ----To be called after an election: check if hitler
//is chancellor after 3+ fascist policies passed
func (s *Store) IsHitlerChancellor(RoomID string) string {
	room := s.GetRoom(RoomID)
	user := s.GetUser(room.ChancellorID)

	if room.FascistPoliciesPassed >= 3 && strings.Compare(user.SecretRole, "Hitler") == 0 {
		return constants.FascistsWin
	}
	return constants.InProgress
}
