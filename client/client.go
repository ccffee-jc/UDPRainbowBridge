package client

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"UDPRainbowBridge/utils"
)

// InterfaceAddress 用于存储网络接口的名称和IPv4地址
type InterfaceAddress struct {
	NAME string
	IPV4 string
}

// 全局的序列号映射表和互斥锁
var (
	// 序列号锁
	index_mutex = sync.Mutex{}

	// 地址锁
	addr_mutex = sync.Mutex{}

	// 目标ip
	target_ip string

	// 目标起始端口
	target_port_base int

	// 监听ip
	listen_ip string

	// 本地监听端口
	listen_port int

	// 本地监听套接字与对端端口结构体
	local_addr_record *utils.RecordSocket

	// 聚合连接
	sockets []*net.UDPConn

	// 命中包统计
	hit_counts []int

	// 命中统计锁
	hit_mutex = sync.Mutex{}
)

// 获取所有活动的网络接口
func get_all_inter_face() []InterfaceAddress {
	interfaces, err := net.Interfaces()
	if err != nil {
		fmt.Println("获取网络接口失败:", err)
		return nil
	}

	var arr []InterfaceAddress
	for _, inter := range interfaces {
		// 排除MTU为-1的接口
		if inter.MTU == -1 {
			continue
		}
		// 排除未启用的接口
		if inter.Flags&net.FlagUp != net.FlagUp {
			continue
		}
		addrs, err := inter.Addrs()
		if err != nil || len(addrs) < 1 {
			continue
		}
		// 只处理IPv4地址
		var v4 string
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}
			if ip.To4() != nil {
				v4 = ip.String()
				break
			}
		}
		if v4 == "" {
			continue
		}
		arr = append(arr, InterfaceAddress{
			NAME: inter.Name,
			IPV4: v4,
		})
	}
	return arr
}

// 让用户选择要使用的网络接口
func select_inter_face(inter_faces_list []InterfaceAddress) []string {
	var input string
	fmt.Println("请输入要使用的接口索引，例如 0,1,2:")
	_, err := fmt.Scanf("%s", &input)
	if err != nil {
		fmt.Println("读取输入失败:", err)
		return nil
	}

	indices := strings.Split(input, ",")
	var selected []string
	for _, str := range indices {
		str = strings.TrimSpace(str)
		idx, err := strconv.Atoi(str)
		if err != nil || idx < 0 || idx >= len(inter_faces_list) {
			fmt.Printf("无效的索引: %s\n", str)
			continue
		}
		selected = append(selected, inter_faces_list[idx].IPV4)
	}
	return selected
}

// 使用传入的字符串地址创建udp套接字
func create_cluster_socket(addr []string) []*net.UDPConn {
	var sockets []*net.UDPConn
	for index, local_ip := range addr {
		// 本地地址
		localAddr := &net.UDPAddr{
			IP:   net.ParseIP(local_ip),
			Port: 0,
		}

		// 远程地址
		remoteAddr := &net.UDPAddr{
			IP:   net.ParseIP(target_ip),
			Port: target_port_base + index,
		}

		// 创建链接
		conn, err := net.DialUDP("udp", localAddr, remoteAddr)
		if err != nil {
			fmt.Println("创建udp套接字失败:", err)
			continue
		}

		// 添加到数组后面
		sockets = append(sockets, conn)

		fmt.Printf("创建udp套接字，对端ip：%s 本地ip：%s\n", remoteAddr.String(), localAddr.String())
	}
	return sockets
}

// 监听集群套接字信息
func handle_cluster_socket_info(socket *net.UDPConn, index int) {
	defer socket.Close()

	// 缓存
	buf := make([]byte, 1350)

	// 循环读取数据
	for {
		n, _, err := socket.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("读取数据失败:", err)
			continue
		}

		if n < 4 {
			fmt.Println("数据包长度小于4字节，丢弃。")
			continue
		}

		// 取前四个字节解析序列号
		seq := string(buf[:4])

		// 判断序列号是否有效
		index_mutex.Lock()
		if !utils.IndexIsValid(seq) {
			// fmt.Println("序列号无效:", seq)
			index_mutex.Unlock()
			continue
		}

		// 记录包序号
		utils.RecordIndex(seq)

		// 释放锁
		index_mutex.Unlock()

		// 判断local_addr_record是否为空
		if local_addr_record == nil {
			fmt.Println("local_addr_record为空，丢弃数据包。")
			continue
		}

		// 增加命中统计
		hit_mutex.Lock()
		hit_counts[index]++
		hit_mutex.Unlock()

		// 将数据转发到本地监听端口
		if local_addr_record != nil {
			// 使用local_addr_record获取地址
			addr_mutex.Lock()

			// 判断Addr是否有值
			if len(local_addr_record.Addr) == 0 {
				// 地址为空值，不发送
				addr_mutex.Unlock()
				continue
			}

			remoteAddr, _ := net.ResolveUDPAddr("udp", local_addr_record.Addr)
			addr_mutex.Unlock()
			_, sendErr := local_addr_record.Socket.WriteToUDP([]byte(buf[4:n]), remoteAddr)
			if sendErr != nil {
				fmt.Println("转发数据包失败:", sendErr)
			} else {
				// 打印日志输出
				// fmt.Printf("转发数据包到本地监听端口:%s\n", local_addr_record.Addr)
			}
		}
	}
}

// 创建本地监听套接字
func create_local_socket() bool {
	// 创建本地监听套接字
	listenUdpAddr := &net.UDPAddr{
		IP:   net.ParseIP(listen_ip),
		Port: listen_port,
	}

	conn, err := net.ListenUDP("udp", listenUdpAddr)

	// 创建RecordSocket结构体
	local_addr_record = &utils.RecordSocket{
		Socket: conn,
		Addr:   "",
	}

	if err != nil {
		fmt.Println("创建本地监听套接字失败:", err)
		return false
	}

	fmt.Printf("创建本地监听套接字：%s\n", listenUdpAddr.String())

	return true
}

// 处理本地监听套接字信息
func handle_local_socket_info() {
	defer local_addr_record.Socket.Close()

	// 缓存
	buf := make([]byte, 1350)

	// 循环读取数据
	for {
		n, addr, err := local_addr_record.Socket.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("读取本地监听套接字数据失败:", err)
			continue
		}

		// 记录地址
		addr_mutex.Lock()
		local_addr_record.Addr = addr.String()
		addr_mutex.Unlock()

		// 增加包序号
		index := utils.GetIndex()

		// 添加序列号
		packet := append([]byte(index), buf[:n]...)

		// 通过所有的UDP连接发送数据包
		for _, udpConn := range sockets {
			_, err := udpConn.Write(packet)
			if err != nil {
				fmt.Println("发送UDP包失败:", err)
			}
			// fmt.Printf("数据包转发->%s\n", udpConn.RemoteAddr().String())
		}
	}
}

func print_hit_counts() {
	for {
		// 间隔五秒输出一次
		time.Sleep(5 * time.Second)

		// 输出统计信息
		hit_mutex.Lock()
		for index, count := range hit_counts {
			fmt.Printf("套接字 %d 命中包数量: %d\n", index, count)
			hit_counts[index] = 0
		}
		hit_mutex.Unlock()
	}
}

func Start(_target_ip string, _target_port_base int, _listen_ip string, _listen_port int) {
	target_ip = _target_ip
	target_port_base = _target_port_base
	listen_ip = _listen_ip
	listen_port = _listen_port

	// 获取所有活动的网络接口
	inter_faces_list := get_all_inter_face()
	if len(inter_faces_list) == 0 {
		fmt.Println("未找到活动的网络接口。")
		return
	}

	// 显示可用的网络接口
	fmt.Println("可用的网络接口:")
	for idx, inter := range inter_faces_list {
		fmt.Printf("%d: %s - %s\n", idx, inter.NAME, inter.IPV4)
	}

	// 用户选择要使用的接口
	local_addr_list := select_inter_face(inter_faces_list)
	if len(local_addr_list) == 0 {
		fmt.Println("未选择有效的网络接口。")
		return
	}

	// 用选择的接口建立udp套接字
	sockets = create_cluster_socket(local_addr_list)
	hit_counts = make([]int, len(sockets))

	for index := range hit_counts {
		hit_counts[index] = 0
	}

	// 监听sockets中的套接字接受信息
	for index, socket := range sockets {
		go handle_cluster_socket_info(socket, index)
	}

	// 创建本地监听套接字
	if !create_local_socket() {
		// 创建本地监听套接字失败
		return
	}

	// 监听本地套接字
	go handle_local_socket_info()

	fmt.Printf("程序运行，等待输入\n")

	// 统计日志
	go print_hit_counts()

	// 阻止主函数退出
	select {}
}
