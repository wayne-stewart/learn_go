module remote_deploy/client

go 1.18

require github.com/gorilla/websocket v1.5.0

replace remote_deploy/common => ../common

require remote_deploy/common v0.0.0
