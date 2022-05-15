package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"remote_deploy/common"
)

type Destinations []string

func (i *Destinations) String() string {
	return "decompress destinations"
}

func (i *Destinations) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var destinations Destinations

func main() {
	addr := flag.String("addr", "", "Address of remote server: -addr <domain or ip>:port")
	src := flag.String("src", "", "source: folder to deploy is required; -src c:\\dir1\\dir2")
	flag.Var(&destinations, "dst", "destinations: multiple can be specified, one is required; -dst \\\\server1\\c$\\dir1\\dir2")
	flag.Parse()

	if len(*src) == 0 {
		fatalError("src is required")
	}
	validate_dir_exists(*src)

	remote_message_chan := make(chan struct{})

	interrupt_chan := make(chan os.Signal, 1)
	signal.Notify(interrupt_chan, os.Interrupt)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	/*
			count := 0
			total := 5

			for {
				select {
				case <-interrupt_chan:
					fmt.Print("\nOperation Cancelled\n")
					return
				case <-ticker.C:
					count++
					fmt.Printf("\r[%d/%d] discovering files", count, total)
					if count >= total {
						fmt.Println()
						goto PROGRESS_END
					}
				}
			}
		PROGRESS_END:
	*/
	compress_buffer := bytes.NewBuffer(make([]byte, 0, 1024*1024))
	compress_count, err := common.Compress(*src, compress_buffer, common.ProgressEach)

	// initialize websocket connection
	uri := url.URL{Scheme: "ws", Host: *addr, Path: "/rfd"}
	websocket_conn, _, err := websocket.DefaultDialer.Dial(uri.String(), nil)
	if err != nil {
		log.Fatalln("Failed to connect with WebSocket!", err)
	}
	defer websocket_conn.Close()

	go remoteMessageLoop(websocket_conn, &remote_message_chan)

	sendMetaData(websocket_conn, compress_buffer.Len(), compress_count, destinations)

	sendData(websocket_conn, compress_buffer)

	closeConnection(websocket_conn, &remote_message_chan)

	// waits until remote message chan is triggered/closed
	<-remote_message_chan

	// for {
	// 	select {
	// 	case <-progress_done:
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
	// 	case <-interrupt_chan:
	// 		closeConnection(websocket_conn, &progress_done)
	// 		return
	// 	}
	// }
}

func sendMetaData(conn *websocket.Conn, byte_size int, item_count int, destinations Destinations) {
	buffer := bytes.NewBuffer(make([]byte, 0, 1024))
	buffer.WriteString(common.META_BAR)
	buffer.WriteString(fmt.Sprintf("%d|%d|", byte_size, item_count))
	buffer.WriteString(strings.Join(destinations, ","))
	err := conn.WriteMessage(websocket.TextMessage, buffer.Bytes())
	if err != nil {
		log.Fatalln("Failed to write meta data to web socket!", err)
	}
}

func sendData(conn *websocket.Conn, buffer *bytes.Buffer) {
	data := buffer.Bytes()
	chunk_size := 4096
	for low := 0; low < len(data); {
		common.ProgressBytes(low, len(data), "sending data", 20)
		high := low + chunk_size
		if high > len(data) {
			high = len(data)
		}
		err := conn.WriteMessage(websocket.BinaryMessage, data[low:high])
		if err != nil {
			fmt.Println()
			log.Fatalln("Failed to write data to the web socket!", err)
		}
		low = high
	}
	common.ProgressBytes(len(data), len(data), "all data sent", 0)
	fmt.Println()
	conn.WriteMessage(websocket.TextMessage, []byte(common.DATA_DONE))
}

func remoteMessageLoop(conn *websocket.Conn, remote_message_chan *chan struct{}) {
	defer close(*remote_message_chan)
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Println("Failed to read from WebSocket!", err)
			}
			return
		}
		log.Printf("recv: %s", message)
	}
}

func closeConnection(conn *websocket.Conn, remote_message_chan *chan struct{}) {
	err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		log.Fatalln("Failed to gracefully close WebSocket!", err)
	}

	// hold off on returning out of the loop until the websocket is closed
	// gracefully or we receive a terminate interrupt from the OS
	select {
	case <-*remote_message_chan:
	case <-time.After(time.Second):
	}
}

func validationError(s string, arg ...any) {
	fatalError(s, arg...)
	flag.Usage()
}

func fatalError(s string, arg ...any) {
	fmt.Print("ERROR: ")
	if len(arg) > 0 {
		fmt.Printf(s, arg...)
		fmt.Println()
	} else {
		fmt.Println(s)
	}
	os.Exit(1)
}

func validate_dir_exists(path string) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		validationError("does not exist: %s", path)
	}

	if info.IsDir() {
		f, err := os.Open(path)
		if err != nil {
			validationError("failed to open: %s", path)
		}
		f.Close()
	} else {
		validationError("is not a directory: %s", path)
	}
}
