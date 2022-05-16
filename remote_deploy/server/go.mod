module remote_deploy/server/v0.1.0

go 1.18

require (
	github.com/gorilla/websocket v1.5.0 // indirect
	golang.org/x/sys v0.0.0-20220503163025-988cb79eb6c6 // indirect
)

replace remote_deploy/common => ../common

require remote_deploy/common v0.1.0