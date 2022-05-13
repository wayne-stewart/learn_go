package main

import (
	"encoding/binary"
	"flag"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	addr := flag.String("addr", "localhost:8081", "url is required")
	flag.Parse()
	/*
		resp, err := http.Get(*url)
		if err != nil {
			log.Fatalln(err)
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalln(err)
		}

		sb := string(body)

		log.Println(sb)
	*/

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	uri := url.URL{Scheme: "ws", Host: *addr, Path: "/rfd"}
	c, _, err := websocket.DefaultDialer.Dial(uri.String(), nil)
	if err != nil {
		log.Fatalln("Failed to connect with WebSocket!", err)
	}

	defer c.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Println("Failed to read from WebSocket!", err)
				}
				return
			}
			log.Printf("recv: %s", message)
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	// first send the size of the data to be transferred
	// so that the server can allocate the space needed to store it
	buffer := make([]byte, 10)
	buffer[0] = 'S'
	buffer[1] = 'Z'
	binary.BigEndian.PutUint64(buffer[2:], 12345)
	err = c.WriteMessage(websocket.BinaryMessage, buffer)
	if err != nil {
		log.Fatalln("Failed to write buffer size to web socket!", err)
	}

	// send the chunks
	buffer = make([]byte, 4096+14)
	for i := 0; i < 10; i++ {
		buffer[0] = 'C'
		buffer[1] = 'H'
		binary.BigEndian.PutUint64(buffer[2:], uint64(i))    // offset
		binary.BigEndian.PutUint32(buffer[10:], uint32(i*2)) // size
		err = c.WriteMessage(websocket.BinaryMessage, buffer)
		if err != nil {
			log.Fatalln("Failed to write buffer size to web socket!", err)
		}
	}

	err = c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		log.Fatalln("Failed to gracefully close WebSocket!", err)
	}

	<-done

	// for {
	// 	select {
	// 	case <-done:
	// 		return
	// 	// case <-ticker.C:
	// 	// 	b = !b
	// 	// 	if b {
	// 	// 		err = c.WriteMessage(websocket.TextMessage, []byte("PT"))
	// 	// 	} else {
	// 	// 		err = c.WriteMessage(websocket.TextMessage, []byte("ST"))
	// 	// 	}
	// 	// 	if err != nil {
	// 	// 		log.Fatalln("Failed to write to WebSocket!", err)
	// 	// 	}
	// 	case <-interrupt:
	// 		closeConnection(c, &done)
	// 		return
	// 	}
	// }
}

func closeConnection(conn *websocket.Conn, done *chan struct{}) {
	err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		log.Fatalln("Failed to gracefully close WebSocket!", err)
	}

	// hold off on returning out of the loop until the websocket is closed
	// gracefully or we receive a terminate interrupt from the OS
	select {
	case <-*done:
	case <-time.After(time.Second):
	}
}
