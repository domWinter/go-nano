package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/domWinter/go-nano"
)

type answer struct {
	Result int
}

type message struct {
	Pattern string `json:"Pattern"`
	Left    int    `json:"Left"`
	Right   int    `json:"Right"`
}

func main() {
	svc := nano.NewService("nano-service", 9090, "nano-server", 8080)

	svc.Add("role:math,cmd:sum", func(body []byte) ([]byte, error) {

		fmt.Println("New request!")

		var msg message
		err := json.Unmarshal(body, &msg)
		if err != nil {
			arr := make([]byte, 0)
			return arr, errors.New("parsing error!")
		}

		result := msg.Left + msg.Right

		answer := answer{result}
		b, _ := json.Marshal(answer)

		return b, nil
	})
}
