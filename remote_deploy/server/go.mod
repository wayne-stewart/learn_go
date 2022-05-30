module remote_deploy/server/v0.1.0

go 1.18

replace remote_deploy/common => ../common

require remote_deploy/common v0.1.0

replace remote_deploy/winsvc => ../winsvc

require (
	github.com/gorilla/websocket v1.5.0
	golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a
	remote_deploy/winsvc v0.1.0
)
