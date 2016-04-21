#!/bin/sh

echo "Installing the packages required to run project"
echo "If some github packages are not present in the repo, please do a go get"
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
