package userinfo

// User : structure of members
type User struct {
	UID     string
	Addr    string
	IsSuper bool
}

// Room : structure of rooms
type Room struct {
	Rid     string
	Host    string
	Members []string
}

// UserMap  the users
var UserMap = make(map[string]User)

// RoomMap  the rooms
var RoomMap = make(map[string]Room)

// AddUser  add one user to UserMap
func AddUser(mem User) {
	UserMap[mem.UID] = mem
}

// AddUserToRoom  add one user to a room
func AddUserToRoom(mem string, rid string) {
	room, ok := RoomMap[rid]
	if ok != true {
		var memList []string
		RoomMap[rid] = Room{Rid: rid, Host: mem, Members: memList}
	}
	room = RoomMap[rid]
	room.Members = append(room.Members, mem)
}

// SetAttr Member::SetAttr
func (memp *User) SetAttr(id string, addr string) {
	memp.UID = id
	memp.Addr = addr
}

// SetAttr Room::SetAttr
func (roomp *Room) SetAttr(host string, memList []string) {
	roomp.Host = host
	roomp.Members = memList
}
