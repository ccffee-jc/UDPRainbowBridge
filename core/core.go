package core

import (
	"encoding/hex"
	"net"
	"time"
)

// 定义套接字结构体，包含一个udp套接字与套接字的地址
type RecordSocket struct {
	Socket *net.UDPConn
	Addr   string
}

// 全局变量存储标识符及其过期时间
var (
	indices           = make(map[string]time.Time)
	expiration        = 5 * time.Second
	counter    uint16 = 0
)

// RecordIndex 记录接收到的包的唯一标识符
func RecordIndex(index string) {
	// 记录该Index，并设置过期时间
	indices[index] = time.Now().Add(expiration)
}

// IndexIsValid 判断接收到的包的唯一标识符是否有效
// 返回 true 表示有效（可以处理），false 表示无效（重复包）
func IndexIsValid(index string) bool {
	expireTime, exists := indices[index]
	if exists {
		if time.Now().Before(expireTime) {
			// fmt.Printf("标识符无效: %s, %v\n", index, exists)
			// 存在且未过期，标识符无效（重复包）
			return false
		}
		// 已过期，删除该标识符
		delete(indices, index)
	}
	// fmt.Printf("有效: %s, %v\n", index, exists)
	// 不存在或已过期，标识符有效
	return true
}

// GetIndex 生成一个新的唯一标识符
func GetIndex() string {
	// 将计数器转换为2字节
	indexBytes := uint16ToBytes(counter)

	// 递增计数器
	counter++

	index := hex.EncodeToString(indexBytes)

	return index
}

// uint16ToBytes 将uint16转换为2字节切片（大端）
func uint16ToBytes(n uint16) []byte {
	bytes := make([]byte, 2)
	bytes[0] = byte((n >> 8) & 0xFF)
	bytes[1] = byte(n & 0xFF)
	return bytes
}
