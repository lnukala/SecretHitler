package db

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

type Room struct {
	Room_Id int
	Curr_Players string
	Global_Com_Topic_Name string
	Global_Notification_Topic_Name string
	No_Policies_Passed int
	Fascist_Polcies_Passed int
	Liberal_Policies_Passed int
	Current_Fascist_In_Deck int
	Current_Liberal_In_Deck int
	Current_Total_In_Deck int
	Chancellor_Id int
	President_Id int
	President_Channel string
	Chancelor_Channel string
	Hitler_Id int
}

type User struct {
	Id int
	Name string
	User_Type string
	Node_Type string
	Secret_Role string
	Is_President bool
	Is_Chancelor bool
}

var db sql.db 

func openDB() {
	db, err = sql.Open("mysql",
		"user:password@tcp(127.0.0.1:3306)/hello")
	if err != nil {
		log.Fatal(err)
	}
}

func getUser(uid int) {
	
	user := new(User)
	user.Id = id;

	rows, err := db.Query("select Name, User_Type, Node_Type, Secret_Role, Is_President, Is_Chancelor from users where id = ?", uid)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&user.Name, &user.User_Type, &user.Node_Type, &user.Secret_Role, &user.Is_President, &user.Is_Chancelor)
		if err != nil {
			log.Fatal(err)
		}
	}

	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}
	
	json, _ := json.Marshal(user)
	
	return json;
}

func getBaseRoom(roomId int) {

	room =: new(Room)
	var sink int

        rows, err := db.Query("select * from room where room_id = ?", roomId)
        if err != nil {
                log.Fatal(err)
        }
        defer rows.Close()
        for rows.Next() {
                err := rows.Scan(&sink, &room.Curr_Players, &room.Global_Comm_Topic_Name, &room.Global_Notification_Topic_Name, &room.No_Policies_Passed, &room.Fascist_Policies_Passed, &room.Liberal_Policies_Passed, &room.Current_Fascist_In_Deck, &room.Current_Liberal_In_Deck, &room.Current_Total_In_Deck, &room.Chancelor_Id, &room.President_Id, &room.President_Channel, &room.Chancelor_Channel, &room.Hitler_Id )
                if err != nil {
                        log.Fatal(err)
                }
        }

        err = rows.Err()
        if err != nil {
                log.Fatal(err)
        }

        json, _ := json.Marshal(room)

        return json;

}
