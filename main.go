package main

import (
	"flag"
	"fmt"

	"UDPRainbowBridge/client"
	"UDPRainbowBridge/server"
)

func main() {
	var model string
	var remote_ip string
	var remote_port int
	var listen_ip string
	var base_listen_port int

	flag.StringVar(&model, "model", "s", "s: 服务器模式, c: 客户端模式")

	flag.StringVar(&remote_ip, "r", "127.0.0.1", "目标地址")
	flag.IntVar(&remote_port, "rp", 60101, "目标起始端口")
	flag.StringVar(&listen_ip, "l", "0.0.0.0", "监听地址")
	flag.IntVar(&base_listen_port, "lp", 60100, "监听端口")

	flag.Parse()

	if model == "s" {
		server.Start(remote_ip, remote_port, listen_ip, base_listen_port)
	} else if model == "c" {
		client.Start(remote_ip, remote_port, listen_ip, base_listen_port)
	} else {
		fmt.Println("无效模式")
	}
}
