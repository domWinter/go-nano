# Go nano
![Build](https://github.com/domWinter/go-nano/workflows/Build/badge.svg)
![static check](https://github.com/domWinter/go-nano/workflows/static%20check/badge.svg)
![Deployment](https://github.com/domWinter/go-nano/workflows/Deployment/badge.svg)

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

type serviceAnswer struct {
	Result bool
}

type serviceRequest struct {
	Pattern string `json:"Pattern"`
	Value   int    `json:"Value"`
}

func main() {
	svc := nano.NewService("127.0.0.1", 8080, "127.0.0.1", 9999)

	svc.Add("role:math,cmd:positive", func(body []byte) ([]byte, error) {
		var msg serviceRequest
        	json.Unmarshal(body, &msg)
		result := msg.Value > 0
		answer := serviceAnswer{result}
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
* Query a another service inside the *add* function with:
```go

        ...

type serviceQuery struct {
	Pattern        string
	ServiceRequest bool
	Value          int
}

requestJSON, _ := json.Marshal(serviceQuery{"role:math,cmd:positive", true, -1})
requestResult, _ := svc.Act(requestJSON, "role:math,cmd:positive")

var resultMap map[string]interface{}
json.Unmarshal(requestResult, &resultMap)
fmt.Println(resultMap["Result"])

        ...

```

Checkout the \_examples folder for Docker and Kubernetes deployments.

## Details
This framework uses pattern matching for routing messages to the corresponding services.<br>
A pattern in Go nano is defined as a comma seperated list of characters/words (e.g. 'role:math,cmd:sum' or 'a:1,b:2,c:3').

### Service registering
At first a new service registers itself at the nano server, which must have a valid connection to a redis database.
The server stores the registering service pattern as key for the service address in redis and also fills a local radix tree cache for future lookups with the same mapping.

![Register](https://github.com/domWinter/go-nano/blob/master/_images/nano_register_service.png)

### Service discovery 
When a client (which can also be a service) queries another service with a specific pattern for the first time, the message is routed to the nano server, which does a lookup in the redis database or local cache and proxies the request to the corresponding service with the longest matching prefix. If a nano server did not find a requested service pattern in its local cache, it will always fill the cache after successfully processing the request.

![Query1](https://github.com/domWinter/go-nano/blob/master/_images/nano_query_service_1.png)

### Caching
Any requesting service also fills a local cache with the pattern->service mapping and will query a cached service directly instead of using the nano server over and over.

![Query2](https://github.com/domWinter/go-nano/blob/master/_images/nano_query_service_2.png)



