create database SecretHitler;
use SecretHitler;

create table room (room_id INT NOT NULL PRIMARY KEY,
curr_players VARCHAR(100),
global_comm_topic_name VARCHAR(100),
global_notification_topic_name VARCHAR(100),
no_policies_passed INT DEFAULT 0,
fascist_policies_passed INT DEFAULT 0,
liberal_policies_passed INT DEFAULT 0,
current_fascist_in_deck INT DEFAULT 11,
current_liberal_in_deck INT DEFAULT 6,
current_total_in_deck INT DEFAULT 17, 
chancellor_id INT DEFAULT -1,
president_id INT DEFAULT -1,
president_channel VARCHAR(100),
chancellor_channel VARCHAR(100), 
hitler_id INT DEFAULT -1
);

create table user (id INT NOT NULL PRIMARY KEY,
name VARCHAR(100),
user_type VARCHAR(100),
node_type VARCHAR(100),
secret_role VARCHAR(100),
is_president BOOL,
is_chancelor BOOL);

create table message (sender_id INT NOT NULL PRIMARY KEY,
topic VARCHAR(100),
message_body VARCHAR(300),
timestamp DATETIME
);
