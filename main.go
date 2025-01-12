package main

import (
	"flag"
	"fmt"
	"strings"

	"UDPRainbowBridge/client"
	"UDPRainbowBridge/server"
)

func main() {
	var sc string
	var remote_ip string
	var remote_port int
	var listen_ip string
	var base_listen_port int
	var mtu int

	// 客户端指定ip列表
	var client_local_ip_str string

	// 聚合指定的端口（和base_listen_port冲突）
	var force_listen_port_str string

	var mode string

	flag.StringVar(&sc, "sc", "s", "s: 服务器模式, c: 客户端模式")

	flag.StringVar(&remote_ip, "r", "127.0.0.1", "目标地址")
	flag.IntVar(&remote_port, "rp", 60101, "目标起始端口")
	flag.StringVar(&listen_ip, "l", "0.0.0.0", "监听地址")
	flag.IntVar(&base_listen_port, "lp", 60100, "监听端口")

	flag.StringVar(&mode, "mode", "mode1", "mode1: 多倍发包模式，mode2: 自动选择线路模式")
	flag.StringVar(&client_local_ip_str, "local_ip_list", "", "可选，客户端指定ip列表，例子：192.168.2.110:192.168.3.110")
	flag.IntVar(&mtu, "mtu", 1492, "可选，mtu，最大包体支持，默认：1492")

	flag.StringVar(&force_listen_port_str, "force_listen_port", "", "可选，强制聚合指定的端口，和base_listen_port冲突，例子：60100:60101")

	flag.Parse()

	force_listen_port := strings.Split(force_listen_port_str, ":")

	if sc == "s" {
		server.Start(remote_ip, remote_port, listen_ip, base_listen_port, mtu, force_listen_port)
	} else if sc == "c" {
		// 使用:分割client_local_ip_str获取客户端指定ip列表
		client_local_ip_list := strings.Split(client_local_ip_str, ":")
		client.Start(remote_ip, remote_port, listen_ip, base_listen_port, mtu, client_local_ip_list, force_listen_port)
	} else {
		fmt.Println("无效模式")
	}
}
