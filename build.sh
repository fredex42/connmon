#!/bin/bash

echo Cleaning...
rm -f src/connmonn/connmonn
rm -f src/connary/connary

echo Compiling...
cd src/connmonn
GOOS=linux GOARCH=amd64 go build
cd ../connary
GOOS=linux GOARCH=amd64 go build
cd ../..
echo Building image...
docker build .
