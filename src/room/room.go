package room

// Member : structure of members
type Member struct {
	Name string
	Addr [4]byte
}

// Room : structure of rooms
type Room struct {
	Host    Member
	Members []Member
}

// SetAttr Member::SetAttr
func (memp *Member) SetAttr(name string, addr [4]byte) {
	memp.Name = name
	memp.Addr = addr
}

// SetAttr Room::SetAttr
func (roomp *Room) SetAttr(host Member, memList []Member) {
	roomp.Host = host
	roomp.Members = memList
}
