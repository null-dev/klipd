package klipd

import (
	"github.com/xtaci/kcp-go"
	"log"
	"strconv"
	"sync"
	"time"
)

func StartClient(password string, ip string, port int) {
	var bc, cryptError = newBlockCryptFromPassword(password)
	if cryptError != nil {
		log.Fatal("Could not initialize encryption: ", cryptError)
		return
	}
	var builtAddress = ip + ":" + strconv.Itoa(port)
	var conn *kcp.UDPSession = nil
	var connWriteMutex sync.Mutex

	var connect func()
	var retryConnection = func() {
		log.Fatal("A connection error occurred, reconnecting in 5 seconds...")
		time.Sleep(time.Duration(5) * time.Second)
		connect()
	}
	connect = func() {
		if conn != nil {
			_ = conn.Close()
		} // Close old connection

		var tempConn, listenError = kcp.DialWithOptions(builtAddress, bc, DEFAULT_DATA_SHARDS, DEFAULT_PARITY_SHARDS)
		if listenError != nil {
			log.Fatal("Could not connect to the supplied address: ", listenError)
			retryConnection()
		} else {
			conn = tempConn
			conn.SetWriteDelay(false)
			conn.SetStreamMode(true)

			log.Println("klipd is connected to: ", builtAddress)
		}
	}
	connect()

	go watchClipboard(func(newData string) {
		connWriteMutex.Lock()
		if err := conn.SetWriteDeadline(defaultWriteDeadline()); err != nil {
			log.Println("Connection error: ", err)
			retryConnection()
			connWriteMutex.Unlock()
			return
		}
		if err := writePacket(packetData{Clipboard: newData}, conn); err != nil {
			log.Println("Connection error: ", err)
			retryConnection()
		}
		connWriteMutex.Unlock()
	})

	// Keep connection alive
	go (func() {
		for range time.Tick(DEFAULT_KEEP_ALIVE_FREQUENCY) {
			connWriteMutex.Lock()
			if err := conn.SetWriteDeadline(defaultWriteDeadline()); err != nil {
				log.Println("Connection error: ", err)
				retryConnection()
				connWriteMutex.Unlock()
				continue
			}
			if err := writeKeepAlive(conn); err != nil {
				log.Println("Connection error: ", err)
				retryConnection()
			}
			connWriteMutex.Unlock()
		}
	})()

	for {
		var data packetData
		err, keepAlive := readMessage(conn, &data)
		if err != nil {
			log.Println("Connection error: ", err)
			retryConnection()
			continue
		}

		if !keepAlive {
			writeToClipboard(data)
		}
	}
}
