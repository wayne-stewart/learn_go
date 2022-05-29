package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	data_size := 0
	destinations := make([]string, 0)

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Println("Failed to read from WebSocket!", err)
			}
			return
		}

		switch {
		case mt == websocket.TextMessage && string(message[0:5]) == common.META_BAR:
			log.Println("meta data received")
			meta_strings := strings.Split(string(message[5:]), "|")
			data_size, _ = strconv.Atoi(meta_strings[0])
			destinations = strings.Split(meta_strings[2], ",")
		case mt == websocket.TextMessage && string(message) == common.DATA_DONE:
			log.Println("data received")
			if buffer.Len() != data_size {
				_ = c.WriteMessage(websocket.TextMessage, []byte("ERROR: inavlid data size"))
			}
			if err := decompress_deploy(c, buffer.Bytes(), destinations); err != nil {
				_ = c.WriteMessage(websocket.TextMessage, []byte("ERROR: "+err.Error()))
			}
		case mt == websocket.BinaryMessage:
			buffer.Write(message)
		default:
			_ = c.WriteMessage(mt, []byte("ERROR: unknown command"))
			return
		}

		if err != nil {
			log.Println("Failed to write to WebSocket!", err)
			return
		}
	}
}

func decompress_deploy(conn *websocket.Conn, compress_buffer []byte, destinations []string) error {
	ts := time.Now()
	send_progress := func(count int, total_items int, message string, previously_written int) int {
		// only send progress updates every 100ms
		since := time.Since(ts)
		if since > 200*time.Millisecond {
			ts = time.Now()
			_ = conn.WriteMessage(websocket.TextMessage,
				[]byte("PROGRESS: "+common.ProgressEachValue(count, total_items, message)))
		}
		return 0
	}
	for i := 0; i < len(destinations); i++ {
		reader := bytes.NewReader(compress_buffer)
		if err := common.Uncompress(reader, destinations[i], send_progress); err != nil {
			return err
		}
		_ = conn.WriteMessage(websocket.TextMessage, []byte("PROG DONE: "+destinations[i]))
	}
	_ = conn.WriteMessage(websocket.TextMessage, []byte("DONE"))
	return nil
}
