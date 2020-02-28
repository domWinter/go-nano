package main

import (
	"encoding/json"
	"errors"

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
	svc := nano.NewService("127.0.0.1", 5555, "127.0.0.1", 9999)

	svc.Add("role:math,cmd:sum,return:zero", func(body []byte) ([]byte, error) {

		var msg Message
		if err := json.Unmarshal(body, &msg); err != nil {
			arr := make([]byte, 0)
			return arr, errors.New("Parsing error!")
		}

		result := 0

		answer := Answer{result}
		b, _ := json.Marshal(answer)

		return b, nil
	})
}
