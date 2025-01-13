@echo off

:: 创建输出目录
mkdir out

:: Linux amd64
set GOOS=linux
set GOARCH=amd64
go build -o .\out\udp_rainbow_bridge_linux_amd64
echo Build completed for Linux (amd64)

:: Linux arm
set GOOS=linux
set GOARCH=arm
go build -o .\out\udp_rainbow_bridge_linux_arm
echo Build completed for Linux (arm)

:: Linux arm64
set GOOS=linux
set GOARCH=arm64
go build -o .\out\udp_rainbow_bridge_linux_arm64
echo Build completed for Linux (arm64)

:: Windows amd64
set GOOS=windows
set GOARCH=amd64
go build -o .\out\udp_rainbow_bridge_windows_amd64.exe
echo Build completed for Windows (amd64)

:: 清理环境变量
set GOOS=
set GOARCH=