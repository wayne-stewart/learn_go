package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/http"

	"golang.org/x/sys/windows/svc"

	"github.com/gorilla/websocket"

	"remote_deploy/common"
)

var upgrader = websocket.Upgrader{}

func main() {

	listen_addr := flag.String("listen_addr", "localhost:8081", "http service address")
	flag.Parse()

	is_service, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("failed to determine if we are running in user interactive: %v", err)
	}

	if is_service {
		log.Println("running as a service.")
	} else {
		log.Println("running as user interactive.")
	}

	http.HandleFunc("/rfd", handle_rfd)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, friend. Who are you?")
	})

	log.Fatal(http.ListenAndServe(*listen_addr, nil))
}

func handle_rfd(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade to a WebSocket!", err)
		return
	}
	defer c.Close()

	buffer := bytes.NewBuffer(make([]byte, 0, 10*1024*1024))

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Println("Failed to read from WebSocket!", err)
			}
			return
		}

		/* the command will be the first 2 characters of the message */
		command := string(message[0:2])

		switch {
		case mt == websocket.TextMessage && string(message[0:5]) == common.META_BAR:
			log.Println("meta data received")
			// size := binary.BigEndian.Uint64(message[2:])
			// data = make([]byte, size)
			// err = c.WriteMessage(mt, []byte(fmt.Sprintf("SZ: %d", len(data))))
		case mt == websocket.TextMessage && string(message) == common.DATA_DONE:
			log.Println("data received")
		case mt == websocket.BinaryMessage:
			buffer.Write(message)
			//log.Println("data received")
			// offset := binary.BigEndian.Uint64(message[2:10])
			// size := binary.BigEndian.Uint32(message[10:14])
			// err = c.WriteMessage(mt, []byte(fmt.Sprintf("CH: %d, %d", offset, size)))
		default:
			_ = c.WriteMessage(mt, []byte("UNKNOWN COMMAND: "+command))
			return
		}

		if err != nil {
			log.Println("Failed to write to WebSocket!", err)
			return
		}
	}
}

func decompress_deploy(compress_buffer []byte, destinations []string) error {
	for i := 0; i < len(destinations); i++ {
		reader := bytes.NewReader(compress_buffer)
		if err := common.Uncompress(reader, destinations[i]); err != nil {
			return err
		}
	}
	return nil
}
