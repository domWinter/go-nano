# Go nano
![Build](https://github.com/domWinter/go-nano/workflows/Build/badge.svg)

**Go nano** - Framwork for building nanoservices (not monolithic microservices) in Go. 

This framework covers routing of pattern
based messages to distributed services via pattern matching and caches. <br>
With go nano you do not have to worry about how messages are exchanged between your 
services so that you can focus on building your busniess logic instead. In addition, 
Go nano was built with the intention to be deployed on top of container orchestation systems like Kubernetes or Docker and thus supporting scaling via loadbalancing of the underleying
platform.

**Current Status:** *Under active development*



## Quick Start

* Start a redis cache with docker:

```docker
docker run --name redis-cache -p 6379:6379 -d redis:latest
```

* Start a nano server:
 
```go
package main

import (
	"github.com/domWinter/go-nano"
)

func main() {
	server := nano.NewServer("127.0.0.1:6379", "", 0, 9999)
	server.Listen()
}
```

* Start a nano service:

```go
package main

import (
	"encoding/json"
	"errors"

	"github.com/domWinter/go-nano"
)

type ServiceAnswer struct {
	Result bool
}

type ServiceRequest struct {
	Pattern string `json:"Pattern"`
	Value   int    `json:"Value"`
}

func main() {
	svc := nano.NewService("127.0.0.1", 8080, "127.0.0.1", 9999)

	svc.Add("role:math,cmd:positive", func(body []byte) ([]byte, error) {
		var msg ServiceRequest
        	json.Unmarshal(body, &msg)
		result := msg.Value > 0
		answer := ServiceAnswer{result}
		answerJSON, _ := json.Marshal(answer)

		return answerJSON, nil
	})
}
```
* Query the service or server port (9999, 8080)
```bash
curl -X POST http://127.0.0.1:9999/ \
-H "Content-Type: application/json" \
-d '{"Pattern":"role:math,cmd:positive","Value":1}'

{"Result":true}
```

## Query a service from within a service
* Query another service inside the *add* function with:
```go

        ...

type Request struct {
	Pattern        string
	ServiceRequest bool
	Value          int
}

requestJSON, _ := json.Marshal(Request{"role:math,cmd:positive", true, -1})
requestResult, _ := svc.Act(requestJSON, "role:math,cmd:positive")

var resultMap map[string]interface{}
json.Unmarshal(requestResult, &resultMap)
fmt.Println(resultMap["Result"])

        ...

```

## Details

More to come soon!
