package nano

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/armon/go-radix"
	"github.com/go-redis/redis/v7"
)

type Message struct {
	Pattern        string `json:"Pattern"`
	ServiceRequest bool   `json:"ServiceRequest"`
}

// ############################################################################ //
// ################################ Client #################################### //
// ############################################################################ //

type Service struct {
	Cache          radix.Tree
	Pattern        string `json:"Pattern"`
	ServiceAddress string `json:"ServiceAddress"`
	ServicePort    int    `json:"ServicePort"`
	ServerAddress  string `json:"ServerAddress"`
	ServerPort     int    `json:"ServerPort"`
}

type ServiceAnswer struct {
	Service Service `json:"Service"`
	Payload []byte  `json:"Payload"`
}

func NewService(ServiceAddress string, ServicePort int, ServerAddress string, ServerPort int) *Service {
	svc := new(Service)
	cache := radix.New()
	svc.Cache = *cache
	svc.ServiceAddress = ServiceAddress + ":" + strconv.Itoa(ServicePort)
	svc.ServicePort = ServicePort
	svc.ServerAddress = ServerAddress + ":" + strconv.Itoa(ServerPort)
	svc.ServerPort = ServerPort
	return svc
}

func (self Service) Act(json_msg []byte, pattern string) ([]byte, error) {

	var body []byte
	cacheNeedsUpdate := false

	// Lookup cache
	cacheMatchedPattern, iface, cacheHit := self.Cache.LongestPrefix(pattern)

	if !cacheHit {
		// Cache miss, query nano server for address resolving
		url := "http://" + self.ServerAddress
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(json_msg))
		if err != nil {
			arr := make([]byte, 0)
			return arr, err
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(req)

		if err != nil {
			arr := make([]byte, 0)
			return arr, err
		}
		defer resp.Body.Close()
		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			arr := make([]byte, 0)
			return arr, err
		}
	} else {
		// Cache hit, query cache resolved service address
		svc, _ := iface.(Service)
		url := "http://" + svc.ServiceAddress
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(json_msg))
		if err != nil {
			arr := make([]byte, 0)
			return arr, err
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(req)

		if err != nil {
			// Request to cache resolved address failed, second option server
			url := "http://" + svc.ServerAddress
			req, err = http.NewRequest("POST", url, bytes.NewBuffer(json_msg))
			if err != nil {
				arr := make([]byte, 0)
				return arr, err
			}
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{}
			resp, err = client.Do(req)
			if err != nil {
				// Both requests failed, delete cache entry and return
				go self.Cache.Delete(cacheMatchedPattern)
				arr := make([]byte, 0)
				return arr, err
			}
			// Server answer ok
			cacheNeedsUpdate = true
		}

		defer resp.Body.Close()
		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			arr := make([]byte, 0)
			return arr, err
		}
	}

	var result ServiceAnswer
	json.Unmarshal(body, &result)

	// Insert or update cache
	if !cacheHit && result.Service.ServiceAddress != "" || cacheNeedsUpdate {
		go self.Cache.Insert(result.Service.Pattern, result.Service)
	}

	return result.Payload, nil

}

func (self Service) Add(pattern string, handle func([]byte) ([]byte, error)) {

	// Store pattern
	self.Pattern = pattern

	// Contact server for registering
	svcJson, _ := json.Marshal(self)
	url := "http://" + self.ServerAddress + "/register"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(svcJson))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic("Could not register service!")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		panic("Could not register service! \n Reason: " + resp.Status)
	}

	fmt.Println("Registering successful!")
	fmt.Println("Listening on port: ", strconv.Itoa(self.ServicePort))

	http.HandleFunc("/", self.service(handle))
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(self.ServicePort), nil))
}

func (self Service) service(handle func([]byte) ([]byte, error)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		// Only post requests allowed
		if r.Method != http.MethodPost {
			errorHandler(w, r, http.StatusBadRequest)
			return
		}

		// Fetch request body
		msgBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			errorHandler(w, r, http.StatusBadRequest)
			return
		}

		// Decode json
		var request Message
		if err = json.Unmarshal(msgBody, &request); err != nil {
			errorHandler(w, r, http.StatusBadRequest)
			return
		}

		// Check pattern
		if !strings.Contains(request.Pattern, self.Pattern) {
			errorHandler(w, r, http.StatusBadRequest)
			return
		}

		payload, err := handle(msgBody)

		if err != nil {
			fmt.Println(err)
			errorHandler(w, r, http.StatusInternalServerError)
			return
		}

		var result []byte

		if request.ServiceRequest {
			svcAnswer := ServiceAnswer{
				Service: self,
				Payload: payload,
			}
			jsonSvcAnswer, err := json.Marshal(svcAnswer)
			if err != nil {
				fmt.Println(err)
				errorHandler(w, r, http.StatusInternalServerError)
				return
			}
			result = jsonSvcAnswer
		} else {
			result = payload
		}

		// Answer request
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write(result)
		return
	}
}

// ############################################################################ //
// ################################ Server #################################### //
// ############################################################################ //

type Server struct {
	Cache      radix.Tree
	Redis      *redis.Client
	listenPort int
}

func NewServer(redisAddress string, redisPassword string, redisDB int, listenPort int) *Server {
	s := new(Server)
	cache := radix.New()
	s.Cache = *cache
	r := redis.NewClient(&redis.Options{
		Addr:     redisAddress,
		Password: redisPassword, // no password set
		DB:       redisDB,       // use default DB
	})
	_, err := r.Ping().Result()
	if err != nil {
		panic(err)
	}
	s.Redis = r
	s.listenPort = listenPort
	return s
}

func (self Server) Listen() {
	http.HandleFunc("/", self.proxy())
	http.HandleFunc("/register", self.register())
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(self.listenPort), nil))
}

func (self Server) register() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		// Only handle post requests
		if r.Method != http.MethodPost {
			errorHandler(w, r, http.StatusBadRequest)
			return
		}

		// Decode body
		msg_body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			errorHandler(w, r, http.StatusBadRequest)
			return
		}
		var new_svc Service
		if err := json.Unmarshal(msg_body, &new_svc); err != nil {
			errorHandler(w, r, http.StatusBadRequest)
			return
		}

		// Add service to database
		address := new_svc.ServiceAddress
		err = self.Redis.Set(new_svc.Pattern, address, 0).Err()
		if err != nil {
			errorHandler(w, r, http.StatusInternalServerError)
			return
		}

		// Add service to local cache
		go self.Cache.Insert(new_svc.Pattern, new_svc)

		// Answer request
		w.WriteHeader(http.StatusOK)
		return

	}
}

func (self Server) proxy() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		// Only handle post requests
		if r.Method != http.MethodPost {
			errorHandler(w, r, http.StatusBadRequest)
			return
		}

		// Decode pattern
		msgBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			errorHandler(w, r, http.StatusBadRequest)
			return
		}

		var msgPayload Message
		if err := json.Unmarshal([]byte(string(msgBody)), &msgPayload); err != nil {
			errorHandler(w, r, http.StatusBadRequest)
			return
		}

		if msgPayload.Pattern == "" {
			errorHandler(w, r, http.StatusBadRequest)
			return
		}

		// Lookup cache
		cacheMatchedPattern, iface, cacheHit := self.Cache.LongestPrefix(msgPayload.Pattern)

		var address string
		var respStatusCode int
		var body []byte
		cacheNeedsUpdate := false

		if !cacheHit {

			// Cache miss, lookup redis db
			_, value, matching := redisLongestPrefixMatching(msgPayload.Pattern, self.Redis)

			if !matching {
				errorHandler(w, r, http.StatusNotFound)
				return
			}

			// Proxy request to service
			address = value
			url := "http://" + address
			req, err := http.NewRequest("POST", url, bytes.NewBuffer(msgBody))
			if err != nil {
				errorHandler(w, r, http.StatusInternalServerError)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println(err)
				errorHandler(w, r, http.StatusInternalServerError)
				return
			}
			defer resp.Body.Close()

			// Decode service response and status code
			body, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				errorHandler(w, r, http.StatusInternalServerError)
				return
			}
			respStatusCode = resp.StatusCode
		} else {

			// Cache hit, proxy request to service
			svc, _ := iface.(Service)
			address = svc.ServiceAddress
			url := "http://" + address
			req, err := http.NewRequest("POST", url, bytes.NewBuffer(msgBody))
			if err != nil {
				errorHandler(w, r, http.StatusInternalServerError)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{}
			resp, err := client.Do(req)

			if err != nil {

				fmt.Println("Cache was wrong")

				// Delete cache entry
				go self.Cache.Delete(cacheMatchedPattern)

				// Request to cache resolved address failed, second option db
				matchedPattern, value, matching := redisLongestPrefixMatching(msgPayload.Pattern, self.Redis)
				if !matching {
					errorHandler(w, r, http.StatusNotFound)
					return
				}

				// Proxy request to db resolved address
				address = value
				url := "http://" + address
				req, err := http.NewRequest("POST", url, bytes.NewBuffer(msgBody))
				if err != nil {
					errorHandler(w, r, http.StatusInternalServerError)
					return
				}
				req.Header.Set("Content-Type", "application/json")
				client := &http.Client{}
				resp, err = client.Do(req)
				if err != nil {
					go self.Redis.Del(matchedPattern)
					errorHandler(w, r, http.StatusNotFound)
					return
				}
				fmt.Println("Second loopkup success!")
				cacheNeedsUpdate = true
			}
			defer resp.Body.Close()
			// Decode service response
			body, err = ioutil.ReadAll(resp.Body)

			if err != nil {
				errorHandler(w, r, http.StatusInternalServerError)
				return
			}
			respStatusCode = resp.StatusCode
		}

		// Insert or update local cache async
		if !cacheHit || cacheNeedsUpdate {
			newSvc := Service{
				Pattern:        msgPayload.Pattern,
				ServiceAddress: address,
			}
			go self.Cache.Insert(msgPayload.Pattern, newSvc)
		}

		// Return result
		w.WriteHeader(respStatusCode)
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
		return
	}
}

func redisLongestPrefixMatching(pattern string, r *redis.Client) (string, string, bool) {

	var value string
	var searchString string
	var err error
	patternFields := strings.Split(pattern, ",")

	// Longest prefix matching for redis entries
	for len(patternFields) > 0 {
		searchString = strings.Join(patternFields, ",")

		value, err = r.Get(searchString).Result()
		if err != nil || value == "" {
			patternFields = patternFields[:len(patternFields)-1]
			continue
		} else {
			break
		}
	}
	if len(patternFields) == 0 {
		return "", "", false
	} else {
		return searchString, value, true
	}
}

func errorHandler(w http.ResponseWriter, r *http.Request, status int) {
	w.WriteHeader(status)
	fmt.Fprint(w, status)
}
