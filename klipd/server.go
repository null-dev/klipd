package klipd

import (
	"github.com/thoas/go-funk"
	"github.com/xtaci/kcp-go"
	"log"
	"net"
	"strconv"
	"sync"
)

type trackedConns struct {
	mutex       sync.Mutex
	connections []*kcp.UDPSession
}

func StartServer(password string, ip string, port int) {
	var bc, cryptError = newBlockCryptFromPassword(password)
	if cryptError != nil {
		log.Fatal("Could not initialize encryption: ", cryptError)
		return
	}
	var builtAddress = ip + ":" + strconv.Itoa(port)
	var lis, listenError = kcp.ListenWithOptions(builtAddress, bc, DEFAULT_DATA_SHARDS, DEFAULT_PARITY_SHARDS)
	if listenError != nil {
		log.Fatal("Could not bind to the supplied address: ", listenError)
		return
	}

	var channel = make(chan packet)
	var connections trackedConns

	go func() {
		for {
			var packet = <-channel

			connections.mutex.Lock()
			// Write clipboard to ourself first before broadcasting
			writeToClipboard(packet.data)

			broadcastClipboardPacket(packet.sourceIp, packet.data, connections.connections)
			connections.mutex.Unlock()
		}
	}()

	go watchClipboard(func(newData string) {
		connections.mutex.Lock()
		broadcastClipboardPacket(nil, packetData{Clipboard: newData}, connections.connections)
		connections.mutex.Unlock()
	})

	log.Println("klipd daemon running on: ", builtAddress)
	for {
		if conn, err := lis.AcceptKCP(); err == nil {
			log.Println("==> Client ", conn.RemoteAddr(), " connected!")
			conn.SetWriteDelay(false)
			conn.SetStreamMode(true)

			connections.mutex.Lock()
			connections.connections = append(connections.connections, conn)
			connections.mutex.Unlock()
			go serverHandleConnection(conn, channel, &connections)
		}
	}
}

func broadcastClipboardPacket(sourceIp net.Addr, data packetData, connections []*kcp.UDPSession) {
	for _, conn := range connections {
		if conn.RemoteAddr() != sourceIp {
			// Broadcast new clipboard data
			var err = conn.SetWriteDeadline(defaultWriteDeadline())
			if err != nil {
				log.Println("Failed to send clipboard data to ", conn.RemoteAddr(), "! ", err)
				// Close connection on error
				_ = conn.Close()
				continue
			}
			err = writePacket(data, conn)
			if err != nil {
				log.Println("Failed to send clipboard data to ", conn.RemoteAddr(), "! ", err)
				// Close connection on error
				_ = conn.Close()
				continue
			}
		}
	}
}

func serverHandleConnection(conn *kcp.UDPSession, channel chan packet, openConnections *trackedConns) {
	var err error = nil
	for {
		var data packetData

		err = conn.SetReadDeadline(defaultReadDeadline())
		if err != nil {
			break
		}
		err = readMessage(conn, &data)
		if err != nil {
			break
		}

		channel <- packet{sourceIp: conn.RemoteAddr(), data: data}
	}

	// Close connection and remove connection from tracking list
	if err != nil {
		log.Println("An error occurred with a connection, dropping connection: ", err)
	}
	log.Println("<== Client ", conn.RemoteAddr(), " disconnected!")
	_ = conn.Close()
	openConnections.mutex.Lock()
	openConnections.connections = funk.Filter(openConnections.connections, func(thisConn *kcp.UDPSession) bool {
		return thisConn.RemoteAddr() != conn.RemoteAddr()
	}).([]*kcp.UDPSession)
	openConnections.mutex.Unlock()
}
