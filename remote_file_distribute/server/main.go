package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net/http"

	"golang.org/x/sys/windows/svc"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

func main() {

	addr := flag.String("addr", "localhost:8081", "http service address")
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

	log.Fatal(http.ListenAndServe(*addr, nil))
}

func handle_rfd(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade to a WebSocket!", err)
		return
	}
	defer c.Close()

	// buffer := make([]byte, 4096)
	var data []byte

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

		switch command {
		case "SZ":
			size := binary.BigEndian.Uint64(message[2:])
			data = make([]byte, size)
			err = c.WriteMessage(mt, []byte(fmt.Sprintf("SZ: %d", len(data))))
		case "CH":
			offset := binary.BigEndian.Uint64(message[2:10])
			size := binary.BigEndian.Uint32(message[10:14])
			err = c.WriteMessage(mt, []byte(fmt.Sprintf("CH: %d, %d", offset, size)))
		case "DI":
			err = c.WriteMessage(mt, []byte(""))
		case "ST":
			err = c.WriteMessage(mt, []byte("STATUS MESSAGE RECEIVED"))
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
