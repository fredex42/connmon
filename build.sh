#!/bin/bash

echo Cleaning...
rm -f src/connmonn
echo Compiling...
cd src
GOOS=linux GOARCH=amd64 go build
cd ..
echo Building image...
docker build .
