package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/windows/svc/debug"

	"github.com/gorilla/websocket"

	"remote_deploy/common"

	"remote_deploy/winsvc"
)

var upgrader = websocket.Upgrader{}
var log debug.Log
var deploy_agent DeployAgentService

const SERVICE_NAME = "Deploy Agent"

func main() {

	deploy_agent = DeployAgentService{listen_addr: "localhost:8081"}

	service_manager := winsvc.ServiceManager{Name: SERVICE_NAME, Desc: SERVICE_NAME, Service: &deploy_agent}

	if len(os.Args) > 1 {
		service_manager.Command(os.Args[1])
	} else {
		service_manager.Run()
	}
}

type DeployAgentService struct {
	listen_addr string
	server      http.Server
}

func (service *DeployAgentService) Start(elog debug.Log) {

	log = elog

	srvmux := http.NewServeMux()

	srvmux.HandleFunc("/rfd", handle_rfd)

	srvmux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, friend. Who are you?")
	})

	service.server = http.Server{
		Addr:    service.listen_addr,
		Handler: srvmux,
	}

	err := service.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Error(1, fmt.Sprintf("%s: failed to listen on %s, error: %v", SERVICE_NAME, service.listen_addr, err))
	}
}

func (service *DeployAgentService) Stop() {
	service.server.Close()
}

func handle_rfd(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error(1, fmt.Sprintf("%s: failed to upgrade to a WebSocket! %v", SERVICE_NAME, err))
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
				log.Error(1, fmt.Sprintf("%s: failed to read from WebSocket! %v", SERVICE_NAME, err))
			}
			return
		}

		switch {
		case mt == websocket.TextMessage && string(message[0:5]) == common.META_BAR:
			//log.Println("meta data received")
			meta_strings := strings.Split(string(message[5:]), "|")
			data_size, _ = strconv.Atoi(meta_strings[0])
			destinations = strings.Split(meta_strings[2], ",")
		case mt == websocket.TextMessage && string(message) == common.DATA_DONE:
			//log.Println("data received")
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
			log.Error(1, fmt.Sprintf("%s: failed to write to WebSocket! %v", SERVICE_NAME, err))
			return
		}
	}
}

func decompress_deploy(conn *websocket.Conn, compress_buffer []byte, destinations []string) error {
	progress := common.BeginProgress(func(count int, total int, message string) string {
		m := "PROGRESS: " + common.ProgressEachValue(count, total, message)
		_ = conn.WriteMessage(websocket.TextMessage, []byte(m))
		return m
	})
	progress.DisablePrint()
	for i := 0; i < len(destinations); i++ {
		reader := bytes.NewReader(compress_buffer)
		if err := common.Uncompress(reader, destinations[i], progress); err != nil {
			return err
		}
		_ = conn.WriteMessage(websocket.TextMessage, []byte("PROG DONE: "+destinations[i]))
	}
	_ = conn.WriteMessage(websocket.TextMessage, []byte("DONE"))
	return nil
}
