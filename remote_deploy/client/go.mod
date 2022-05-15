module remote_file_distribute/client

go 1.18

require github.com/gorilla/websocket v1.5.0 // indirect

replace remote_file_distribute/common => ../common
require remote_file_distribute/common v0.0.0