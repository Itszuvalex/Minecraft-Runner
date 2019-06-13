@echo off
set GOPATH=%~dp0
echo %GOPATH%
set GOOS=linux
set GOARCH=amd64
echo Building linux runner
go build -o mcrunner.exe main
echo Building Dockerfile
docker build .
echo Removing built linux exe
del mcrunner.exe
echo Done!
