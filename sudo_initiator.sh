#!/bin/sh

echo "Setting the environment in the sudo su"
ulimit -n 120000
export PATH=$PATH:/usr/local/go/bin
export GOROOT=/usr/local/go
export PATH=$PATH:$GOROOT/bin
export GOPATH=/home/ec2-user/hitler

