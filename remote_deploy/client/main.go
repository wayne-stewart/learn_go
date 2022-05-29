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
		fatalError(true, "src is required")
	}
	validate_dir_exists(*src)

	remote_message_chan := make(chan struct{})

	interrupt_chan := make(chan os.Signal, 1)
	signal.Notify(interrupt_chan, os.Interrupt)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	compress_buffer := bytes.NewBuffer(make([]byte, 0, 1024*1024))
	compress_count, _ := common.Compress(*src, compress_buffer, common.ProgressEach)

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
	chars_written := 0
	ts := time.Now()
	for low := 0; low < len(data); {
		since := time.Since(ts)
		if since > 200*time.Millisecond {
			ts = time.Now()
			chars_written = common.ProgressBytes(low, len(data), "sending data", chars_written)
		}
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
	common.ProgressBytes(len(data), len(data), "all data sent\n", chars_written)
	conn.WriteMessage(websocket.TextMessage, []byte(common.DATA_DONE))
}

func remoteMessageLoop(conn *websocket.Conn, remote_message_chan *chan struct{}) {
	defer close(*remote_message_chan)
	chars_written := 0
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Println("\nFailed to read from WebSocket!", err)
			}
			return
		}
		switch {
		case string(message[0:7]) == "ERROR: ":
			log.Fatalln(string(message[7:]))
		case string(message[0:10]) == "PROGRESS: ":
			if chars_written > 0 {
				fmt.Print(strings.Repeat("\b", chars_written))
			}
			chars_written, _ = fmt.Print(string(message[10:]))
		case string(message[0:11]) == "PROG DONE: ":
			if chars_written > 0 {
				fmt.Print(strings.Repeat("\b", chars_written))
			}
			fmt.Printf("[100%%] %s\n", strings.TrimSpace(string(message[11:])))
			chars_written = 0
		case string(message) == "DONE":
			fmt.Println("deploy complete")
			closeConnection(conn, remote_message_chan)
		}
	}
}

func closeConnection(conn *websocket.Conn, remote_message_chan *chan struct{}) {
	conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	// if err != nil {
	// 	log.Fatalln("Failed to gracefully close WebSocket!", err)
	// }

	// hold off on returning out of the loop until the websocket is closed
	// gracefully or we receive a terminate interrupt from the OS
	select {
	case <-*remote_message_chan:
	case <-time.After(time.Second):
	}
}

func validationError(s string, arg ...any) {
	fatalError(true, s, arg...)
}

func fatalError(show_usage bool, s string, arg ...any) {
	fmt.Print("ERROR: ")
	if len(arg) > 0 {
		fmt.Printf(s, arg...)
		fmt.Println()
	} else {
		fmt.Println(s)
	}
	if show_usage {
		flag.Usage()
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
