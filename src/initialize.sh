#!/bin/sh

echo "Installing the packages required to run project"
echo "If some github packages are not present in the repo, please go a go get"

go install zmq
go install dnsimple
go install constants
go install backend
go install userinfo
go install apiserver
go install run
go install test
