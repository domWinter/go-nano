package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/domWinter/go-nano"
)

type Answer struct {
	Result int
}

type Message struct {
	Pattern string `json:"Pattern"`
	Left    int    `json:"Left"`
	Right   int    `json:"Right"`
}

func main() {
	svc := nano.NewService("0.0.0.0", 8081, "nano-server", 8080)

	svc.Add("role:math,cmd:sum", func(body []byte) ([]byte, error) {

		fmt.Println("New request!")

		var msg Message
		err := json.Unmarshal(body, &msg)
		if err != nil {
			arr := make([]byte, 0)
			return arr, errors.New("Parsing error!")
		}

		result := msg.Left + msg.Right

		answer := Answer{result}
		b, _ := json.Marshal(answer)

		return b, nil
	})
}
