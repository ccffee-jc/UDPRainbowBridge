package client

import (
	"UDPRainbowBridge/core"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// InterfaceAddress 用于存储网络接口的名称和IPv4地址
type InterfaceAddress struct {
	NAME string
	IPV4 string
}

const send_queue_max_len = 1024

// 全局的序列号映射表和互斥锁
var (
	// 序列号锁
	index_mutex = sync.Mutex{}

	// 地址锁
	addr_mutex = sync.Mutex{}

	// 本地监听套接字与对端端口结构体
	local_addr_record *core.RecordSocket

	// 聚合连接
	sockets []*net.UDPConn

	// 命中包统计
	hit_counts []int

	// 命中统计锁
	hit_mutex = sync.Mutex{}

	// 聚合端口发送队列组()
	send_queue [][send_queue_max_len][]byte

	// 监听顿口发送队列指针列表，第一位代表当前数据位置，第二位代表当前已经发送数据位置
	send_queue_point_list [][2]int64

	// 模式
	mode string
)

// 使用传入的字符串地址创建udp套接字
func create_cluster_socket(remote_addr_list []string, send_addr_list []string) []*net.UDPConn {
	var sockets []*net.UDPConn
	for index, local_ip := range send_addr_list {
		// 本地地址
		localAddr, err_resolveLocal := net.ResolveUDPAddr("udp", local_ip)

		if err_resolveLocal != nil {
			fmt.Println("解析本地地址失败:", err_resolveLocal)
			continue
		}

		// 远程地址
		remoteAddr, err_resolveRemote := net.ResolveUDPAddr("udp", remote_addr_list[index])

		if err_resolveRemote != nil {
			fmt.Println("解析远程地址失败:", err_resolveRemote)
			continue
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
func handle_cluster_socket_info(socket *net.UDPConn, index int, mtu int) {
	defer socket.Close()

	// 缓存
	buf := make([]byte, mtu)

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
		if !core.IndexIsValid(seq) {
			// fmt.Println("序列号无效:", seq)
			index_mutex.Unlock()
			continue
		}

		// 记录包序号
		core.RecordIndex(seq)

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
func create_local_socket(listen_addr string) bool {
	// 创建本地监听套接字
	listenUdpAddr, err_resolve := net.ResolveUDPAddr("udp", listen_addr)

	if err_resolve != nil {
		fmt.Println("解析本地监听地址失败:", err_resolve)
		return false
	}

	conn, err := net.ListenUDP("udp", listenUdpAddr)

	// 创建RecordSocket结构体
	local_addr_record = &core.RecordSocket{
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
func handle_local_socket_info(mtu int) {
	defer local_addr_record.Socket.Close()

	// 缓存
	buf := make([]byte, mtu)

	sendIndex := 0

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
		index := core.GetIndex()

		// 添加序列号
		packet := append([]byte(index), buf[:n]...)

		if mode == "mode1" {
			// 多倍发包模式
			// 通过所有的UDP连接发送数据包
			for index := range sockets {
				// 将数据包放入发送队列
				// 先获取位置
				next_index := atomic.LoadInt64(&send_queue_point_list[index][0])

				// 判断是否满了（下一个位置是发送位置）(理论上不应该发生)
				waitCount := 0
				for {
					if next_index+1 == atomic.LoadInt64(&send_queue_point_list[index][1]) {
						// 等待写入
						time.Sleep(1 * time.Millisecond)

						fmt.Println("发送队列满了，等待写入完成", waitCount)
						waitCount++
					} else {
						break
					}
				}

				// 写入数据
				send_queue[index][next_index] = packet
				// 坐标后移
				atomic.StoreInt64(&send_queue_point_list[index][0], (next_index+1)%send_queue_max_len)

			}
		} else if mode == "mode2" {
			// 链路聚合模式
			// 按顺序进行发包
			// 先获取位置
			next_index := atomic.LoadInt64(&send_queue_point_list[sendIndex][0])

			// 判断是否满了（下一个位置是发送位置）(理论上不应该发生)
			waitCount := 0
			for {
				if next_index+1 == atomic.LoadInt64(&send_queue_point_list[sendIndex][1]) {
					// 等待写入
					time.Sleep(1 * time.Millisecond)

					fmt.Println("发送队列满了，等待写入完成", waitCount)
					waitCount++
				} else {
					break
				}
			}

			// 写入数据
			send_queue[sendIndex][next_index] = packet
			// 坐标后移
			atomic.StoreInt64(&send_queue_point_list[sendIndex][0], (next_index+1)%send_queue_max_len)

			// sendIndex++
			sendIndex = (sendIndex + 1) % len(sockets)
		}

	}
}

func print_hit_counts() {
	for {
		// 间隔五秒输出一次
		time.Sleep(5 * time.Second)

		// 获取总数
		total := 0
		for _, count := range hit_counts {
			total += count
		}

		// 输出统计信息
		hit_mutex.Lock()
		for index, count := range hit_counts {
			fmt.Printf("套接字 %d 命中包数量: %d%\n", index, count / total * 100)
			hit_counts[index] = 0
		}
		hit_mutex.Unlock()
	}
}

// 发送数据包的线程
func send_packet_thread(index int) {
	for {
		// 判断是否有数据
		if atomic.LoadInt64(&send_queue_point_list[index][0]) == atomic.LoadInt64(&send_queue_point_list[index][1]) {
			// 没有数据，等待
			time.Sleep(1 * time.Millisecond)
			continue
		}

		// 获取需要发送的数据
		packet := send_queue[index][atomic.LoadInt64(&send_queue_point_list[index][1])]

		// 指向下一个数据
		atomic.StoreInt64(&send_queue_point_list[index][1], (send_queue_point_list[index][1]+1)%send_queue_max_len)

		// 发送数据
		_, sendErr := sockets[index].Write(packet)

		if sendErr != nil {
			fmt.Printf("数据表转发失败，服务端地址：%s\n", sockets[index].RemoteAddr().String())
		} else {
			// fmt.Printf("数据转发成功，服务端地址：%s\n", sockets[index].RemoteAddr().String())
		}
	}
}

func Start(remote_ip_list []string, listen_ip_list []string, send_ip_list []string, mtu int, m string) {
	mode = m

	// 用选择的接口建立udp套接字
	for index, addr := range remote_ip_list {
		fmt.Printf("远程地址%d: %s\n", index, addr)
	}
	sockets = create_cluster_socket(remote_ip_list, send_ip_list)
	hit_counts = make([]int, len(sockets))

	// 初始化发送队列数组
	send_queue = make([][send_queue_max_len][]byte, len(remote_ip_list))
	send_queue_point_list = make([][2]int64, len(remote_ip_list))

	for index := range hit_counts {
		hit_counts[index] = 0
		send_queue[index] = [send_queue_max_len][]byte{}
		send_queue_point_list[index] = [2]int64{0, 0}
	}

	// 监听sockets中的套接字接受信息
	for index, socket := range sockets {
		go handle_cluster_socket_info(socket, index, mtu)

		// 启动写回线程
		go send_packet_thread(index)
	}

	// 创建本地监听套接字
	if !create_local_socket(listen_ip_list[0]) {
		// 创建本地监听套接字失败
		return
	}

	// 监听本地套接字
	go handle_local_socket_info(mtu)

	// 统计日志
	go print_hit_counts()

	fmt.Printf("程序运行，等待输入\n")

	// 阻止主函数退出
	select {}
}
