#!/bin/sh

echo "Installing the packages required to run project"
echo "If some github packages are not present in the repo, please do a go get"
lsof -n -i:5558 | grep LISTEN | awk '{ print $2 }' | uniq | xargs kill -9
lsof -n -i:5557 | grep LISTEN | awk '{ print $2 }' | uniq | xargs kill -9
lsof -n -i:3000 | grep LISTEN | awk '{ print $2 }' | uniq | xargs kill -9
rm -rf raftdb
rm -rf roomdb
go install zmq
go install dnsimple
go install constants
go install room
go install backend
go install userinfo
go install apiserver
go install raft
go install run
