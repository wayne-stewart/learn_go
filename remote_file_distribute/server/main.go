package main

import (
	"flag"
	"log"
	"net/http"

	"golang.org/x/sys/windows/svc"

	"github.com/gorilla/websocket"
)

func main() {

	addr := flag.String("addr", "localhost:8080", "http service address")
	flag.Parse()
	upgrader := websocket.Upgrader{}

	is_interactive, err := svc.IsAnInteractiveSession()
	if err != nil {
		log.Fatalf("failed to determine if we are running in user interactive: %v", err)
	}

	if is_interactive {
		log.Println("running as user interactive.")
	} else {
		log.Println("running as a service.")
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Fatal("upgrade: ", err)
		}
		defer c.Close()

		for {
			mt, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}
			log.Printf("recv: %s", message)
			msg := []byte("ECHO: " + string(message))
			err = c.WriteMessage(mt, msg)
			if err != nil {
				log.Println("write:", err)
				break
			}
		}
	})

	/*
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "Setting Up WebSockets")
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				fmt.Fprint(w, "WebSocket upgrade failed.")
				return
			}
			defer conn.Close()

			for {
				mt, message, err := conn.ReadMessage()
				if err != nil {
					fmt.Fprint(w, "Failed Reading Message.")
					return
				}
				message = []byte("Echo: " + string(message))
				err = conn.WriteMessage(mt, message)
				if err != nil {
					fmt.Fprint(w, "Write Message Failed.")
				}
			}
		})
	*/

	log.Fatal(http.ListenAndServe(*addr, nil))
}
