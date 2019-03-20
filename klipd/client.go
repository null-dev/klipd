package klipd

import (
	"github.com/xtaci/kcp-go"
	"log"
	"strconv"
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
		var err = conn.SetWriteDeadline(defaultWriteDeadline())
		if err != nil {
			log.Println("Connection error: ", err)
			retryConnection()
			return
		}
		err = writePacket(packetData{Clipboard: newData}, conn)
		if err != nil {
			log.Println("Connection error: ", err)
			retryConnection()
		}
	})

	for {
		var data packetData
		if (conn.SetReadDeadline(defaultReadDeadline()) == nil) &&
			readMessage(conn, &data) == nil {
			writeToClipboard(data)
		} else {
			// Reconnect on error
			retryConnection()
		}
	}
}
