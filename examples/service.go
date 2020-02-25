package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/my/repo/nano"
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
	svc := nano.NewService("127.0.0.1", 3333, "127.0.0.1", 9999)

	svc.Add("role:math,cmd:sum", func(body []byte) ([]byte, error) {

		fmt.Println("New request!")

		var msg Message
		err := json.Unmarshal(body, &msg)
		if err != nil {
			arr := make([]byte, 0)
			return arr, errors.New("Parsing error!")
		}

		type Request struct {
			Pattern        string
			ServiceRequest bool
			Value          int
		}

		r_json, _ := json.Marshal(Request{"role:math,cmd:positive", true, msg.Left})
		r_result, err := svc.Act(r_json, "role:math,cmd:positive")

		var act_r map[string]interface{}
		json.Unmarshal(r_result, &act_r)
		fmt.Println(act_r["Result"])

		result := msg.Left + msg.Right

		answer := Answer{result}
		b, _ := json.Marshal(answer)

		return b, nil
	})
}
