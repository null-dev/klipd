package klipd

import (
	"crypto/sha1"
	"encoding/binary"
	"github.com/atotto/clipboard"
	"github.com/vmihailenco/msgpack"
	"github.com/xtaci/kcp-go"
	"golang.org/x/crypto/pbkdf2"
	"io"
	"log"
	"net"
	"time"
)

const DEFAULT_DATA_SHARDS = 10
const DEFAULT_PARITY_SHARDS = 10
const DEFAULT_POLL_FREQUENCY = time.Duration(250) * time.Millisecond
const DEFAULT_KEEP_ALIVE_FREQUENCY = time.Duration(30) * time.Second

var SALT = []byte("klipd-password-salt")

func defaultReadDeadline() (t time.Time) {
	return time.Now().Add(DEFAULT_KEEP_ALIVE_FREQUENCY * 2)
}

func defaultWriteDeadline() (t time.Time) {
	return time.Now().Add(time.Duration(10) * time.Second)
}

type packet struct {
	sourceIp net.Addr
	data     packetData
}

type packetData struct {
	Clipboard string
}

func newBlockCryptFromPassword(password string) (bc kcp.BlockCrypt, err error) {
	var key = pbkdf2.Key([]byte(password), SALT, 4096, 32, sha1.New)
	return kcp.NewBlowfishBlockCrypt(key)
}

func writeToClipboard(data packetData) {
	if clipboardWriteError := clipboard.WriteAll(data.Clipboard); clipboardWriteError != nil {
		log.Println("Failed to write incoming text to the clipboard: ", clipboardWriteError)
	}
}

func readMessage(conn *kcp.UDPSession, message interface{}) (err error, keepAlive bool) {
	var msgLengthEncoded = make([]byte, 4)

	if _, readError := io.ReadFull(conn, msgLengthEncoded); readError != nil {
		return readError, false
	}

	var msgLength = binary.LittleEndian.Uint32(msgLengthEncoded)

	// 0 byte length is keep-alive message
	if msgLength == 0 {
		return nil, true
	}

	var msg = make([]byte, msgLength)
	if _, readError := io.ReadFull(conn, msg); readError != nil {
		return readError, false
	}

	return msgpack.Unmarshal(msg, message), false
}

func writePacket(data packetData, conn *kcp.UDPSession) (err error) {
	return writeMessage(conn, &data)
}

func writeMessage(conn *kcp.UDPSession, message interface{}) (err error) {
	var marshaledMsg, _ = msgpack.Marshal(message)

	var msgLengthEncoded = make([]byte, 4)

	binary.LittleEndian.PutUint32(msgLengthEncoded, uint32(len(marshaledMsg)))

	if _, err := conn.Write(msgLengthEncoded); err != nil {
		return err
	}

	if _, err := conn.Write(marshaledMsg); err != nil {
		return err
	}

	return nil
}

func writeKeepAlive(conn *kcp.UDPSession) (err error) {
	var msgLengthEncoded = make([]byte, 4)
	binary.LittleEndian.PutUint32(msgLengthEncoded, 0)
	_, outErr := conn.Write(msgLengthEncoded)
	return outErr
}

func watchClipboard(callback func(newContents string)) {
	var curData = ""
	for range time.Tick(DEFAULT_POLL_FREQUENCY) {
		if newContents, readErr := clipboard.ReadAll(); readErr == nil && curData != newContents {
			curData = newContents
			callback(curData)
		}
	}
}
