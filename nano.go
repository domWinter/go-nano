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

// ############################################################################ //
// ################################ Service ################################### //
// ############################################################################ //

// Service struct with radix tree as local cache
type Service struct {
	Cache          radix.Tree
	Pattern        string `json:"Pattern"`
	ServiceAddress string `json:"ServiceAddress"`
	ServicePort    int    `json:"ServicePort"`
	ServerAddress  string `json:"ServerAddress"`
	ServerPort     int    `json:"ServerPort"`
}

type serviceMessage struct {
	Pattern        string `json:"Pattern"`
	ServiceRequest bool   `json:"ServiceRequest"`
}

type serviceAnswer struct {
	Service Service `json:"Service"`
	Payload []byte  `json:"Payload"`
}

// NewService returns a initialized service struct
func NewService(ServiceAddress string, ServicePort int, ServerAddress string, ServerPort int) *Service {
	svc := new(Service)
	cache := radix.New()
	svc.Cache = *cache
	svc.ServiceAddress = ServiceAddress + ":" + strconv.Itoa(ServicePort)
	svc.ServicePort = ServicePort
	svc.ServerAddress = ServerAddress + ":" + strconv.Itoa(ServerPort)
	svc.ServerPort = ServerPort
	log.Println("Nano service successfully initialised!")
	log.Println("Listening on:", svc.ServiceAddress)
	log.Println("Nano server at:", svc.ServerAddress)
	return svc
}

//Act sends a query to the defined microservice pattern
func (svc Service) Act(jsonMsg []byte, pattern string) ([]byte, error) {

	var body []byte
	cacheNeedsUpdate := false

	// Lookup cache
	cacheMatchedPattern, iface, cacheHit := svc.Cache.LongestPrefix(pattern)

	if !cacheHit {
		// Cache miss, query nano server for address resolving
		url := "http://" + svc.ServerAddress
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonMsg))
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
		resolvedSvc, _ := iface.(Service)
		url := "http://" + resolvedSvc.ServiceAddress
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonMsg))
		if err != nil {
			arr := make([]byte, 0)
			return arr, err
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(req)

		if err != nil {
			// Request to cache resolved address failed, second option nano server
			url := "http://" + resolvedSvc.ServerAddress
			req, err = http.NewRequest("POST", url, bytes.NewBuffer(jsonMsg))
			if err != nil {
				arr := make([]byte, 0)
				return arr, err
			}
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{}
			resp, err = client.Do(req)
			if err != nil {
				// Both requests failed, delete cache entry and return
				go svc.Cache.Delete(cacheMatchedPattern)
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

	var result serviceAnswer
	err := json.Unmarshal(body, &result)

	if err != nil {
		arr := make([]byte, 0)
		return arr, err
	}

	// Insert or update cache
	if !cacheHit && result.Service.ServiceAddress != "" || cacheNeedsUpdate {
		go svc.Cache.Insert(result.Service.Pattern, result.Service)
	}

	return result.Payload, nil

}

//Add creates a new webservice handler that processes request with refered handle function
func (svc Service) Add(pattern string, handle func([]byte) ([]byte, error)) {

	// Store pattern
	svc.Pattern = pattern

	// Contact server for registering
	svcJSON, _ := json.Marshal(svc)
	url := "http://" + svc.ServerAddress + "/register"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(svcJSON))
	if err != nil {
		panic("Creation of request failed!")
	}
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

	http.HandleFunc("/", svc.service(handle))
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(svc.ServicePort), nil))
}

func (svc Service) service(handle func([]byte) ([]byte, error)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		// Only post requests allowed
		if r.Method != http.MethodPost {
			errorHandler(w, r, http.StatusMethodNotAllowed)
			return
		}

		// Fetch request body
		msgBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			errorHandler(w, r, http.StatusBadRequest)
			return
		}

		// Decode json
		var request serviceMessage
		if err = json.Unmarshal(msgBody, &request); err != nil {
			errorHandler(w, r, http.StatusBadRequest)
			return
		}

		// Check pattern
		if !strings.Contains(request.Pattern, svc.Pattern) {
			errorHandler(w, r, http.StatusBadRequest)
			return
		}

		payload, err := handle(msgBody)

		if err != nil {
			errorHandler(w, r, http.StatusInternalServerError)
			return
		}

		var result []byte

		if request.ServiceRequest {
			svcAnswer := serviceAnswer{
				Service: svc,
				Payload: payload,
			}
			jsonSvcAnswer, err := json.Marshal(svcAnswer)
			if err != nil {
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
		_, err = w.Write(result)
		if err != nil {
			errorHandler(w, r, http.StatusInternalServerError)
			return
		}
	}
}

// ############################################################################ //
// ################################ Server #################################### //
// ############################################################################ //

// Server struct with a radix tree as local cache and redis client for state handeling
type Server struct {
	Cache      radix.Tree
	Redis      *redis.Client
	listenPort int
}

// NewServer initalises a server struct, establishes redis connection and returns
func NewServer(redisAddress string, redisPassword string, redisDB int, listenPort int) *Server {
	srv := new(Server)
	cache := radix.New()
	srv.Cache = *cache
	r := redis.NewClient(&redis.Options{
		Addr:     redisAddress,
		Password: redisPassword,
		DB:       redisDB,
	})
	_, err := r.Ping().Result()
	if err != nil {
		err = fmt.Errorf("connection to redis failed!\n%w", err)
		panic(err)
	}
	srv.Redis = r
	srv.listenPort = listenPort
	log.Println("Nano server successfully initialised!")
	log.Println("Listening on port:", srv.listenPort)
	return srv
}

// Listen function sets up 2 handler for serivce registering and requests
func (srv Server) Listen() {
	http.HandleFunc("/", srv.proxy())
	http.HandleFunc("/register", srv.register())
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(srv.listenPort), nil))
}

func (srv Server) register() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		// Only handle post requests
		if r.Method != http.MethodPost {
			errorHandler(w, r, http.StatusMethodNotAllowed)
			return
		}

		// Decode body
		msgBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			errorHandler(w, r, http.StatusBadRequest)
			return
		}
		var newSvc Service
		err = json.Unmarshal(msgBody, &newSvc)
		if err != nil {
			errorHandler(w, r, http.StatusBadRequest)
			return
		}

		// Add service to database
		address := newSvc.ServiceAddress
		err = srv.Redis.Set(newSvc.Pattern, address, 0).Err()
		if err != nil {
			errorHandler(w, r, http.StatusInternalServerError)
			return
		}

		// Add service to local cache
		go srv.Cache.Insert(newSvc.Pattern, newSvc)

		log.Println("New service registered!")
		log.Println("Service address:", newSvc.ServiceAddress)
		log.Println("Service pattern:", newSvc.Pattern)

		// Answer request
		w.WriteHeader(http.StatusOK)
	}
}

func (srv Server) proxy() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		// Only handle post requests
		if r.Method != http.MethodPost {
			errorHandler(w, r, http.StatusMethodNotAllowed)
			return
		}

		// Decode pattern
		msgBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			errorHandler(w, r, http.StatusBadRequest)
			return
		}

		var msgPayload serviceMessage
		err = json.Unmarshal([]byte(string(msgBody)), &msgPayload)
		if err != nil {
			errorHandler(w, r, http.StatusBadRequest)
			return
		}

		if msgPayload.Pattern == "" {
			errorHandler(w, r, http.StatusBadRequest)
			return
		}

		// Lookup cache
		cacheMatchedPattern, iface, cacheHit := srv.Cache.LongestPrefix(msgPayload.Pattern)

		var address string
		var respStatusCode int
		var body []byte
		cacheNeedsUpdate := false

		if !cacheHit {

			// Cache miss, lookup redis db
			_, value, matching := redisLongestPrefixMatching(msgPayload.Pattern, srv.Redis)

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
				// Delete cache entry
				go srv.Cache.Delete(cacheMatchedPattern)

				// Request to cache resolved address failed, second option db
				matchedPattern, value, matching := redisLongestPrefixMatching(msgPayload.Pattern, srv.Redis)
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
					go srv.Redis.Del(matchedPattern)
					errorHandler(w, r, http.StatusNotFound)
					return
				}
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
			go srv.Cache.Insert(msgPayload.Pattern, newSvc)
		}

		// Return result
		w.WriteHeader(respStatusCode)
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(body)
		if err != nil {
			errorHandler(w, r, http.StatusInternalServerError)
			return
		}
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
	}

	return searchString, value, true

}

func errorHandler(w http.ResponseWriter, r *http.Request, status int) {
	w.WriteHeader(status)
}
