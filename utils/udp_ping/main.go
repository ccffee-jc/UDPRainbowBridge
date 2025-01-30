package main

import (
	"flag"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Message represents a ping message with a sequence number
type Message struct {
	Seq uint64
}

func main() {
	// Define command-line flags
	mode := flag.String("mode", "server", "Mode to run: server or client")
	address := flag.String("address", "localhost:9999", "UDP address to listen on or send to (e.g., localhost:9999)")
	serverAddr := flag.String("server", "localhost:9999", "UDP server address (client mode)")
	interval := flag.Duration("interval", time.Second, "Interval between messages (client mode)")
	flag.Parse()

	if *mode == "server" {
		runServer(*address)
	} else if *mode == "client" {
		runClient(*serverAddr, *interval)
	} else {
		fmt.Println("Invalid mode. Use 'server' or 'client'.")
	}
}

// runServer starts the UDP server
func runServer(address string) {
	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		fmt.Printf("Error resolving address %s: %v\n", address, err)
		return
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Printf("Error listening on %s: %v\n", address, err)
		return
	}
	defer conn.Close()

	fmt.Printf("Server listening on %s\n", address)

	buffer := make([]byte, 1024)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Printf("Error reading from UDP: %v\n", err)
			continue
		}

		// Log received message
		msg := string(buffer[:n])
		fmt.Printf("Received from %s: %s\n", clientAddr, msg)

		// Echo the message back to the sender
		_, err = conn.WriteToUDP(buffer[:n], clientAddr)
		if err != nil {
			fmt.Printf("Error writing to UDP: %v\n", err)
			continue
		}
	}
}

// runClient starts the UDP client
func runClient(server string, interval time.Duration) {
	serverUDPAddr, err := net.ResolveUDPAddr("udp", server)
	if err != nil {
		fmt.Printf("Error resolving server address %s: %v\n", server, err)
		return
	}

	// Local address can be any available port
	localAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 0}
	conn, err := net.DialUDP("udp", localAddr, serverUDPAddr)
	if err != nil {
		fmt.Printf("Error dialing UDP server %s: %v\n", server, err)
		return
	}
	defer conn.Close()

	fmt.Printf("Client sending to %s every %v\n", server, interval)

	var seq uint64 = 0
	var mu sync.Mutex
	sentMessages := make(map[uint64]time.Time)

	// Channel to signal when to stop
	stop := make(chan struct{})

	// Goroutine to listen for incoming messages
	go func() {
		buffer := make([]byte, 1024)
		for {
			select {
			case <-stop:
				return
			default:
				conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				n, _, err := conn.ReadFromUDP(buffer)
				if err != nil {
					// Ignore timeout errors
					if ne, ok := err.(net.Error); ok && ne.Timeout() {
						continue
					}
					fmt.Printf("Error reading from UDP: %v\n", err)
					continue
				}

				// Parse the received message to get the sequence number
				recvMsg := string(buffer[:n])
				parts := strings.Split(recvMsg, ":")
				if len(parts) != 2 {
					fmt.Printf("Invalid message format: %s\n", recvMsg)
					continue
				}
				recvSeq, err := strconv.ParseUint(parts[0], 10, 64)
				if err != nil {
					fmt.Printf("Invalid sequence number: %s\n", parts[0])
					continue
				}

				recvTime := time.Now()

				// Get the send time
				mu.Lock()
				sendTime, exists := sentMessages[recvSeq]
				if exists {
					// Calculate one-way time
					roundTrip := recvTime.Sub(sendTime)
					oneWay := roundTrip / 2
					fmt.Printf("Seq %d: One-way time = %v\n", recvSeq, oneWay)
					// Remove the record
					delete(sentMessages, recvSeq)
				}
				mu.Unlock()
			}
		}
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Send a new message
			seq++
			msg := Message{Seq: seq}
			msgStr := fmt.Sprintf("%d:%d", msg.Seq, time.Now().UnixNano())
			// msg中加入1000字节的随机字符串
			for i := 0; i < 1000; i++ {
				msgStr += "a"
			}
			_, err := conn.Write([]byte(msgStr))
			if err != nil {
				fmt.Printf("Error sending message: %v\n", err)
				continue
			}
			sendTime := time.Now()

			// Record the send time
			mu.Lock()
			sentMessages[msg.Seq] = sendTime
			mu.Unlock()

			// Start a timer to check for timeout
			go func(s uint64, t time.Time) {
				time.Sleep(interval)
				mu.Lock()
				_, exists := sentMessages[s]
				if exists {
					// Timeout occurred, discard the message and prepare to resend
					fmt.Printf("Seq %d: No response, will resend.\n", s)
					delete(sentMessages, s)
					// Optionally, you can implement retransmission logic here
				}
				mu.Unlock()
			}(msg.Seq, sendTime)
		}
	}
}
