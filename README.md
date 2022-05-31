# learn_go
GO experiments and project incubators are here.

Build
- go build main.go
- go build main.go -o exe_name.exe

Strip debug symbols to make the binary smaller
- go build -ldflags "-s -w" main.go
- go build -ldflags "-s -w" -o exe_name.exe main.go

Run
- go run main.go

Test
- go test