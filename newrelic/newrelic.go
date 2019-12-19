package newrelic

//
// Copyright 2019 - New Relic
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/logspout/router"
)

// Adapter is an adapter for streaming JSON from logspout to NR collector.
type Adapter struct {
	Endpoint        string
	Hostname        string
	Key             string
	AuthHeader      string
	Verbose         bool
	FlushInterval   time.Duration
	MaxBufferSize   uint64
	MaxLineLength   uint64
	MaxRequestRetry uint64
	Log             *log.Logger
	Queue           chan Line
	HTTPClient      *http.Client
}

// Line contains the message and number of post retries
type Line struct {
	Log     []byte `json:"_"`
	Retried uint64 `json:"-"`
}

// ContainerInfo contains the container attributes
type ContainerInfo struct {
	Name  string `json:"name"`
	ID    string `json:"id"`
	PID   int    `json:"pid",omitempty`
	Image string `json:"image"`
}

// Message is the actual Log structure
type Message struct {
	Timestamp int64         `json:"timestamp"`
	Message   string        `json:"message"`
	Container ContainerInfo `json:"container"`
	Level     string        `json:"level"`
	Hostname  string        `json:"hostname"`
}

func init() {
	router.AdapterFactories.Register(NewNewRelicAdapter, "newrelic")

	filterLabels := make([]string, 0)
	if filterLabelsValue := os.Getenv("FILTER_LABELS"); filterLabelsValue != "" {
		filterLabels = strings.Split(filterLabelsValue, ",")
	}

	filterSources := make([]string, 0)
	if filterSourcesValue := os.Getenv("FILTER_SOURCES"); filterSourcesValue != "" {
		filterSources = strings.Split(filterSourcesValue, ",")
	}

	// Add log Router specific variables
	r := &router.Route{
		Adapter:       "newrelic",
		FilterName:    getStringOpt("FILTER_NAME", ""),
		FilterID:      getStringOpt("FILTER_ID", ""),
		FilterLabels:  filterLabels,
		FilterSources: filterSources,
	}

	if err := router.Routes.Add(r); err != nil {
		log.Fatal("Cannot Add New Route: ", err.Error())
	}

}

// NewNewRelicAdapter is a new logspout Adapter
func NewNewRelicAdapter(route *router.Route) (router.LogAdapter, error) {
	licenceKey := os.Getenv("LICENSE_KEY")
	apiKey := os.Getenv("API_KEY")
	if licenceKey == "" && apiKey == "" {
		return nil, errors.New("Cannot Find Environment Variable \"LICENSE_KEY\" or \"API_KEY\"")
	}
	key := licenceKey
	authHeader := "X-License-Key"
	if key == "" {
		key = apiKey
		authHeader = "X-Insert-Key"
	}
	// A failure in the API may cause a log stream to hang. Logspout can detect and restart inactive Docker log streams
	// This environment variable enables this feature
	if os.Getenv("INACTIVITY_TIMEOUT") == "" {
		os.Setenv("INACTIVITY_TIMEOUT", "1m")
	}

	transport := &http.Transport{} // Tiemout after 60s
	proxyURLValue := getStringOpt("PROXY_URL", "")
	if proxyURLValue != "" {
		proxyURL, err := url.Parse(proxyURLValue)
		if err != nil {
			log.Fatal("Parsing Proxy URL Error: ", err.Error())
		}
		transport.Proxy = http.ProxyURL(proxyURL)
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	// Create the client
	client := &http.Client{Transport: transport, Timeout: time.Second * 60}

	adapter := &Adapter{
		Endpoint:        getStringOpt("NEW_RELIC_URL", "https://log-api.newrelic.com/log/v1"),
		Hostname:        getStringOpt("HOSTNAME", ""),
		Key:             key,
		AuthHeader:      authHeader,
		Verbose:         os.Getenv("VERBOSE") != "0",
		FlushInterval:   getDurationOpt("FLUSH_INTERVAL", 250) * time.Millisecond,
		MaxBufferSize:   getUintOpt("MAX_BUFFER_SIZE", 1) * 1024 * 1024,
		MaxLineLength:   getUintOpt("MAX_LINE_LENGTH", 15000),
		MaxRequestRetry: getUintOpt("MAX_REQUEST_RETRY", 5),
		Log:             log.New(os.Stdout, getStringOpt("HOSTNAME", "")+" ", log.LstdFlags),
		Queue:           make(chan Line),
		HTTPClient:      client,
	}

	go adapter.readQueue()

	return adapter, nil
}

// Stream logs
func (adapter *Adapter) Stream(logstream chan *router.Message) {
	for m := range logstream {
		fmt.Println("line 158: ", m.Container.Config.Image)
		fmt.Println("line 158: ", adapter.Verbose)
		if adapter.Verbose || m.Container.Config.Image != "newrelic/logspout" {
			fmt.Println("GET INSIDE")
			messageStr, err := json.Marshal(Message{
				Timestamp: time.Now().Unix(),
				Message:   adapter.sanitizeMessage(m.Data),
				Container: ContainerInfo{
					Name:  m.Container.Name,
					ID:    m.Container.ID,
					PID:   m.Container.State.Pid,
					Image: m.Container.Config.Image,
				},
				Level:    adapter.getLevel(m.Source),
				Hostname: adapter.getHost(m.Container.Config.Hostname),
			})

			if err != nil {
				adapter.Log.Println(
					fmt.Errorf(
						"JSON Marshalling Error: %s",
						err.Error(),
					),
				)
			} else {
				adapter.Queue <- Line{
					Log:     messageStr,
					Retried: 0,
				}
			}
		}
	}
}

// Read lines from the queue
func (adapter *Adapter) readQueue() {

	buffer := make([]Line, 0)
	timeout := time.NewTimer(adapter.FlushInterval)
	byteSize := 0

	for {
		select {
		case msg := <-adapter.Queue:
			if uint64(byteSize) >= adapter.MaxBufferSize {
				timeout.Stop()
				adapter.flushBuffer(buffer)
				timeout.Reset(adapter.FlushInterval)
				buffer = make([]Line, 0)
				byteSize = 0
			}
			buffer = append(buffer, msg)
			byteSize += len(string(msg.Log))

		case <-timeout.C:
			fmt.Println("timeout close: ", len(buffer))
			if len(buffer) > 0 {
				adapter.flushBuffer(buffer)
				timeout.Reset(adapter.FlushInterval)
				buffer = make([]Line, 0)
				byteSize = 0
			} else {
				timeout.Reset(adapter.FlushInterval)
			}
		}
	}
}

// send logs and flushBuffer
func (adapter *Adapter) flushBuffer(buffer []Line) {
	var data bytes.Buffer
	var msg Message
	logs := make([]Message, 0)

	for _, line := range buffer {
		error := json.Unmarshal(line.Log, &msg)
		adapter.Log.Println(
			fmt.Errorf(
				"JSON UnMarshalling Error: %s",
				error.Error(),
			),
		)
		logs = append(logs, msg)
	}
	fmt.Println("line 242: ", logs)
	body := struct {
		Logs []Message `json:"logs"`
	}{
		Logs: logs,
	}

	if error := json.NewEncoder(&data).Encode(body); error != nil {
		adapter.Log.Println(
			fmt.Errorf(
				"JSON Encoding Error: %s",
				error.Error(),
			),
		)
		return
	}
	fmt.Println("DATA:", &data)
	req, err := http.NewRequest("POST", adapter.Endpoint, &data)
	if err != nil {
		adapter.Log.Println(
			fmt.Errorf(
				"http: error on http.NewRequest: %s",
				err.Error(),
			),
		)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(adapter.AuthHeader, adapter.Key)

	resp, err := adapter.HTTPClient.Do(req)

	if err != nil {
		if _, ok := err.(net.Error); ok {
			go adapter.retry(buffer)
		} else {
			adapter.Log.Println(
				fmt.Errorf(
					"HTTP Client Post Request Error: %s",
					err.Error(),
				),
			)
		}
		return
	}

	if resp != nil {
		if resp.StatusCode != http.StatusOK {
			adapter.Log.Println(
				fmt.Errorf(
					"Received Status Code: %s While Sending Message",
					resp.StatusCode,
				),
			)
		}
		defer resp.Body.Close()
	}
}

func (adapter *Adapter) retry(buffer []Line) {
	for _, line := range buffer {
		if line.Retried < adapter.MaxRequestRetry {
			line.Retried++
			adapter.Queue <- line
		}
	}
}

func (adapter *Adapter) sanitizeMessage(message string) string {
	if uint64(len(message)) > adapter.MaxLineLength {
		return message[0:adapter.MaxLineLength] + " (cut off, too long...)"
	}
	return message
}

func (adapter *Adapter) getLevel(source string) string {
	switch source {
	case "stdout":
		return "INFO"
	case "stderr":
		return "ERROR"
	}
	return ""
}

func (adapter *Adapter) getHost(containerHostname string) string {
	host := containerHostname
	if adapter.Hostname != "" {
		host = adapter.Hostname
	}
	return host
}

// Getting Uint Variable from Environment:
func getUintOpt(name string, dfault uint64) uint64 {
	if result, err := strconv.ParseUint(os.Getenv(name), 10, 64); err == nil {
		return result
	}
	return dfault
}

// Getting Duration Variable from Environment:
func getDurationOpt(name string, dfault time.Duration) time.Duration {
	if result, err := strconv.ParseInt(os.Getenv(name), 10, 64); err == nil {
		return time.Duration(result)
	}
	return dfault
}

// Getting String Variable from Environment:
func getStringOpt(name, dfault string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return dfault
}
