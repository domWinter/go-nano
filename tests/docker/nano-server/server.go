package main

import (
	"github.com/domWinter/go-nano"
)

func main() {
	server := nano.NewServer("redis:6379", "", 0, 8080)
	server.Listen()
}
