package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func assertAddHashReturnsId(t *testing.T, server *Server, expectedId int) {
	request := httptest.NewRequest(http.MethodPost, "/hash", nil)
	responseRecorder := httptest.NewRecorder()

	server.hashEndpoint(responseRecorder, request)

	if responseRecorder.Code != 200 {
		t.Errorf("Did not return 200 OK")
	}

	bodyString := responseRecorder.Body.String()

	id, err := strconv.Atoi(strings.TrimSpace(bodyString))
	if err != nil {
		t.Errorf("Could not parse response body to id: %s", err)
	}
	if id != expectedId {
		t.Errorf("Did not return %d", expectedId)
	}
}

func TestHashEndpoint(t *testing.T) {
	t.Run("first id returned should be 1", func(t *testing.T) {
		server := Server{hashCmds: startHashLoop(make(chan int))}

		assertAddHashReturnsId(t, &server, 1)
	})

	t.Run("serial calls should return increasing ids", func(t *testing.T) {
		server := Server{hashCmds: startHashLoop(make(chan int))}

		assertAddHashReturnsId(t, &server, 1)
		assertAddHashReturnsId(t, &server, 2)
		assertAddHashReturnsId(t, &server, 3)
		assertAddHashReturnsId(t, &server, 4)
	})

	t.Run("hashing roundtrip", func(t *testing.T) {
		httpServer := httptest.NewServer(router(make(chan int), 0))
		defer httpServer.Close()

		postResp, err := http.PostForm(httpServer.URL+"/hash", map[string][]string{"password": {"angryMonkey"}})
		if err != nil {
			t.Errorf("Unable to post to /hash: %s", err)
		}
		defer postResp.Body.Close()

		postBodyBytes, err := io.ReadAll(postResp.Body)
		if err != nil {
			t.Errorf("Could not read from postResp body %s", err)
		}
		postBodyStr := string(postBodyBytes)

		id, err := strconv.Atoi(strings.TrimSpace(postBodyStr))
		if err != nil {
			t.Errorf("Could not parse postResp body to id: %s", err)
		}
		if id != 1 {
			t.Errorf("Did not return %d", 1)
		}

		getResp, err := http.Get(httpServer.URL + "/hash/1")
		if err != nil {
			t.Errorf("Unable to GET /hash/1 %s", err)
		}

		getBodyBytes, err := io.ReadAll(getResp.Body)
		if err != nil {
			t.Errorf("Could not read from getResp body %s", err)
		}
		getBodyStr := strings.TrimSpace(string(getBodyBytes))

		if getBodyStr != "ZEHhWB65gUlzdVwtDQArEyx+KVLzp/aTaRaPlBzYRIFj6vjFdqEb0Q5B8zVKCZ0vKbZPZklJz0Fd7su2A+gf7Q==" {
			t.Errorf("Did not return expected body, got %s", getBodyStr)
		}
	})

	t.Run("stats return 0 if no requests", func(t *testing.T) {
		httpServer := httptest.NewServer(router(make(chan int), 0))
		defer httpServer.Close()

		getResp, err := http.Get(httpServer.URL + "/stats")
		if err != nil {
			t.Errorf("Unable to make GET call %s", err)
		}
		defer getResp.Body.Close()

		getBodyBytes, err := io.ReadAll(getResp.Body)
		if err != nil {
			t.Errorf("Unable to read from getResp body %s", err)
		}

		var statsJson StatsJson
		err = json.Unmarshal(getBodyBytes, &statsJson)
		if err != nil {
			t.Errorf("Unable to unmarshal into StatsJson %s", err)
		}

		if statsJson.Total != 0 || statsJson.Average != 0 {
			t.Errorf("Stats should have been 0")
		}
	})

	t.Run("graceful shutdown", func(t *testing.T) {
		shutdown := make(chan int)
		httpServer := httptest.NewServer(router(shutdown, 0))

		http.Get(httpServer.URL + "/shutdown")

		<-shutdown
		httpServer.Close()
	})
}
