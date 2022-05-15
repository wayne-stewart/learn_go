package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"

	"remote_file_distribute/common"
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

	compress_buffer := bytes.NewBuffer(make([]byte, 0, 1024*1024))
	common.Compress(*src, compress_buffer)
	fmt.Printf("buffer length: %s\n", common.FormatBytes(compress_buffer.Len()))
	fmt.Printf("buffer capacity: %s\n", common.FormatBytes(compress_buffer.Cap()))

	for i := 0; i < len(destinations); i++ {
		reader := bytes.NewReader(compress_buffer.Bytes())
		if err := common.Uncompress(reader, destinations[i]); err != nil {
			log.Fatalln("Failed to decompress!", err)
		}
	}

	return

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
