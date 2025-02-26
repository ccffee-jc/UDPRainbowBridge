package main

import (
	"UDPRainbowBridge/client"
	"UDPRainbowBridge/server"
	"flag"
	"fmt"
	"strings"
)

func main() {
	var s bool
	var c bool
	var m int
	var mode string
	var r string
	var l string
	var send string

	// 规划参数：将ip与端口统一，且重复类型参数只留一个
	// 转发地址，参数名称：r 参数值示例：192.168.2.3:8080;192.168.2.110:8080
	// 监听地址，参数名称: l 参数值示例：0.0.0.0:9000;0.0.0.0:90001:192.168.2.3:9002
	// 发送地址（客户端用，local地址）， 参数名称：send 参数值192.168.100.1;192.168.99.1  不用带端口！！
	// -s 服务端模式
	// -c 客户端模式
	// -m mtu值设置
	// -mode 模式选择 mode1: 多倍发包模式，mode2:链路聚合模式
	flag.BoolVar(&s, "s", false, "服务端模式")
	flag.BoolVar(&c, "c", false, "客户端模式")
	flag.StringVar(&mode, "mode", "mode1", "mode1: 多倍发包模式，mode2:链路聚合模式")
	flag.IntVar(&m, "mtu", 1492, "可选，mtu，最大包体支持，默认：1492")

	flag.StringVar(&r, "r", "", "转发地址 服务端此参数只能有一个地址，客户端多个 参数值示例：192.168.2.3:8080;192.168.2.110:8080")
	flag.StringVar(&l, "l", "", "监听地址 服务端此参数有多个，客户端单个 参数值示例：0.0.0.0:9000;0.0.0.0:90001:192.168.2.3:9002")
	flag.StringVar(&send, "send", "", "发送地址 客户端用 参数值192.168.100.1:0;192.168.99.1:0  自动选择发送端口请指定端口为0！！")

	flag.Parse()

	// 解析地址参数 使用;分割地址
	// 转发地址
	remote_ip_list := strings.Split(r, ";")
	// 监听地址
	listen_ip_list := strings.Split(l, ";")

	if s {
		// 服务器模式
		server.Start(remote_ip_list, listen_ip_list, m, mode)
	} else if c {
		// 客户端模式

		// 发送地址
		client_local_ip_list := strings.Split(send, ";")

		client.Start(remote_ip_list, listen_ip_list, client_local_ip_list, m, mode)
	}

	// 没有输入参数
	fmt.Println("无效模式")
}
