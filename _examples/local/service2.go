package main

import (
	"encoding/json"
	"errors"

	"github.com/domWinter/go-nano"
)

type Answer struct {
	Result bool
}

type Message struct {
	Pattern string `json:"Pattern"`
	Value   int    `json:"Value"`
}

func main() {
	svc := nano.NewService("127.0.0.1", 4444, "127.0.0.1", 9999)

	svc.Add("role:math,cmd:positive", func(body []byte) ([]byte, error) {

		var msg Message
		err := json.Unmarshal(body, &msg)
		if err != nil {
			arr := make([]byte, 0)
			return arr, errors.New("Parsing error!")
		}

		result := msg.Value > 0

		answer := Answer{result}
		b, _ := json.Marshal(answer)

		return b, nil
	})
}
