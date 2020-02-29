package main

import (
	"github.com/domWinter/go-nano"
)

func main() {
	server := nano.NewServer("127.0.0.1:6379", "", 0, 9999)
	server.Listen()
}
