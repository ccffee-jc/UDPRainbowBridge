package server

import (
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"UDPRainbowBridge/utils"
)

var (
	// 监听端口组
	listen_record_sockets []*utils.RecordSocket

	// 监听端口组对应的地址锁
	listen_record_add_mutex []*sync.Mutex

	// 监听端口组对应的index锁
	listen_record_index_mutex = sync.Mutex{}

	// 本地连接端口
	remote_con_socket *net.UDPConn

	// 命中包统计
	hit_counts []int

	// 命中统计锁
	hit_mutex = sync.Mutex{}
)

// 创建监听端口列表
func create_cluster_listen_socket(listen_ip_list []string) {
	// 初始化数组
	listen_record_sockets = make([]*utils.RecordSocket, len(listen_ip_list))
	listen_record_add_mutex = make([]*sync.Mutex, len(listen_ip_list))

	// 循环最大监听数量次数，监听对应端口
	for i := 0; i < len(listen_ip_list); i++ {

		addr, err_resolve := net.ResolveUDPAddr("udp", listen_ip_list[i])

		if err_resolve != nil {
			fmt.Printf("解析地址 %s 失败: %v\n", listen_ip_list[i], err_resolve)
			continue
		}

		// 监听
		conn, err_listen := net.ListenUDP("udp", addr)
		if err_listen != nil {
			fmt.Printf("监听UDP套接字 %s 失败: %v\n", addr.String(), err_listen)
			continue
		}

		listen_record_sockets[i] = &utils.RecordSocket{
			Socket: conn,
			Addr:   "",
		}

		listen_record_add_mutex[i] = &sync.Mutex{}

		fmt.Printf("创建UDP监听：%s\n", addr.String())
	}
}

func handle_cluster_socket_info(recordSocket *utils.RecordSocket, addMutex *sync.Mutex, index int, mtu int) {
	defer recordSocket.Socket.Close()

	// 缓存
	buf := make([]byte, mtu)

	// 循环读取数据
	for {
		n, addr, err := recordSocket.Socket.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("读取本地监听套接字数据失败:", err)
			continue
		}

		// 记录地址
		addMutex.Lock()
		recordSocket.Addr = addr.String()
		addMutex.Unlock()

		// 判断长度
		if n < 4 {
			fmt.Println("数据包长度小于4字节，丢弃。")
			continue
		}

		// 取前四个字节解析序列号
		seq := string(buf[:4])

		// 判断序列号是否有效
		listen_record_index_mutex.Lock()
		// fmt.Println("测试序列号:", seq)
		if !utils.IndexIsValid(seq) {
			// fmt.Println("序列号无效:", seq)
			listen_record_index_mutex.Unlock()
			continue
		}

		// fmt.Println("！！有效序列号:", seq)

		// 记录包序号
		utils.RecordIndex(seq)

		// 释放锁
		listen_record_index_mutex.Unlock()

		// 增加命中统计
		hit_mutex.Lock()
		hit_counts[index]++
		hit_mutex.Unlock()

		// 转发到远程端口中
		if remote_con_socket != nil {
			// 发送数据
			_, sendErr := remote_con_socket.Write([]byte(buf[4:n]))
			if sendErr != nil {
				fmt.Println("转发数据包失败", sendErr)
			} else {
				// fmt.Printf("转发数据包->%s\n", remote_con_socket.RemoteAddr().String())
			}
		} else {
			fmt.Println("连接未建立，转发失败")
		}
	}
}

// 创建本地转发端口
func create_remote_socket(addr string) {
	remoteAddr, err_resolve := net.ResolveUDPAddr("udp", addr)

	if err_resolve != nil {
		fmt.Printf("解析地址 %s 失败: %v\n", addr, err_resolve)
		os.Exit(1)
	}

	_remote_con_socket, err := net.DialUDP("udp", nil, remoteAddr)
	remote_con_socket = _remote_con_socket

	if err != nil {
		fmt.Printf("无法连接到服务器 %s: %v\n", remoteAddr, err)
		os.Exit(1)
	} else {
		fmt.Printf("创建转发接口：%s", remoteAddr.String())
	}

}

// 监听远端输入
func handle_remote_socket_info(mtu int) {
	buffer := make([]byte, mtu)

	for {
		n, _, err := remote_con_socket.ReadFromUDP(buffer)

		if err != nil {
			fmt.Printf("接收消息出错: %v\n", err)
			continue
		}

		// fmt.Printf("开始处理消息，消息长度：%d\n", n)

		// 生成序号
		listen_record_index_mutex.Lock()
		seq := utils.GetIndex()
		listen_record_index_mutex.Unlock()

		// 添加序列号
		packet := append([]byte(seq), buffer[:n]...)

		// 发送数据包到所有已记录的客户端
		for index, clientRecord := range listen_record_sockets {
			// 判断远端地址是否存在
			listen_record_add_mutex[index].Lock()
			if len(clientRecord.Addr) == 0 {
				// 地址不存在，跳过
				// fmt.Printf("回传地址不存在，index：%d\n", index)
				listen_record_add_mutex[index].Unlock()
				continue
			}

			addr := clientRecord.Addr

			clientAddr, _ := net.ResolveUDPAddr("udp", clientRecord.Addr)
			listen_record_add_mutex[index].Unlock()

			// 发送数据
			_, sendErr := clientRecord.Socket.WriteToUDP(packet, clientAddr)

			if sendErr != nil {
				fmt.Printf("数据表转发失败，客户端地址：%s\n", addr)
			} else {
				fmt.Printf("数据转发成功，客户端地址：%s\n", addr)
			}

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

func Start(remote_ip_list []string, listen_ip_list []string, mtu int, mode string) {
	// 创建本地监听端口套接字群
	create_cluster_listen_socket(listen_ip_list)

	hit_counts = make([]int, len(listen_ip_list))

	for index := range hit_counts {
		hit_counts[index] = 0
	}

	// 本地监听端口监听信息
	for i := 0; i < len(listen_ip_list); i++ {
		recordSocket := listen_record_sockets[i]
		addMutex := listen_record_add_mutex[i]

		// 判断值是否有效
		if recordSocket == nil {
			continue
		}

		go handle_cluster_socket_info(recordSocket, addMutex, i, mtu)
	}

	// 创建本地转发端口
	create_remote_socket(remote_ip_list[0])

	// 监听本地端口输入
	go handle_remote_socket_info(mtu)

	fmt.Println("程序启动，等待客户端接入")

	go print_hit_counts()

	// 阻止主函数退出
	select {}
}
