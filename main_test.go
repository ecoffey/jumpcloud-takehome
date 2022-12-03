package main

import (
	"encoding/json"
	"eoinisawesome.com/jumpcloud-takehome/app"
	"eoinisawesome.com/jumpcloud-takehome/hashes"
	"eoinisawesome.com/jumpcloud-takehome/stats"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestEndpoints(t *testing.T) {
	t.Run("first currentId returned should be 1", func(t *testing.T) {
		a := app.App{HashCmds: hashes.StartHashLoop(make(chan int), 0)}

		assertAddHashReturnsId(t, &a, 1)
	})

	t.Run("serial calls should return increasing ids", func(t *testing.T) {
		a := app.App{HashCmds: hashes.StartHashLoop(make(chan int), 0)}

		assertAddHashReturnsId(t, &a, 1)
		assertAddHashReturnsId(t, &a, 2)
		assertAddHashReturnsId(t, &a, 3)
		assertAddHashReturnsId(t, &a, 4)
	})

	t.Run("hashing roundtrip", func(t *testing.T) {
		httpServer := httptest.NewServer(app.Router(make(chan int), 0))
		defer httpServer.Close()

		id := parseBodyAsInt(t, "first hash", func() (*http.Response, error) {
			return http.PostForm(httpServer.URL+"/hash", map[string][]string{"password": {"angryMonkey"}})
		})

		assert(t, "id", 1, id)

		hash := parseBodyAsStr(t, "get hash", func() (*http.Response, error) {
			return http.Get(httpServer.URL + "/hash/1")
		})

		expectedHash := "ZEHhWB65gUlzdVwtDQArEyx+KVLzp/aTaRaPlBzYRIFj6vjFdqEb0Q5B8zVKCZ0vKbZPZklJz0Fd7su2A+gf7Q=="
		assert(t, "hash", expectedHash, hash)
	})

	t.Run("stats return 0 if no requests", func(t *testing.T) {
		httpServer := httptest.NewServer(app.Router(make(chan int), 0))
		defer httpServer.Close()

		var statsJson stats.StatsJson
		parseBodyAsJson(t, "get stats", &statsJson, func() (*http.Response, error) {
			return http.Get(httpServer.URL + "/stats")
		})

		if statsJson.Total != 0 || statsJson.Average != 0 {
			t.Errorf("Stats should have been 0")
		}
	})

	t.Run("stats return non-zero after a POST", func(t *testing.T) {
		httpServer := httptest.NewServer(app.Router(make(chan int), 0))
		defer httpServer.Close()

		postResp, err := http.PostForm(httpServer.URL+"/hash", map[string][]string{"password": {"angryMonkey"}})
		if err != nil {
			t.Errorf("Unable to post to /hash: %s", err)
		}
		defer postResp.Body.Close()

		var statsJson stats.StatsJson
		parseBodyAsJson(t, "get stats", &statsJson, func() (*http.Response, error) {
			return http.Get(httpServer.URL + "/stats")
		})

		if statsJson.Total == 0 || statsJson.Average == 0 {
			t.Errorf("Stats should have been not 0")
		}
	})

	t.Run("graceful shutdown with no in-flight", func(t *testing.T) {
		shutdown := make(chan int)
		httpServer := httptest.NewServer(app.Router(shutdown, 0))

		http.Post(httpServer.URL+"/shutdown", "", nil)

		<-shutdown
		httpServer.Close()
	})
}

func assert(t *testing.T, context string, expected any, actual any) {
	if actual != expected {
		t.Errorf("[%s]: expected %s, but got actual %s", context, expected, actual)
	}
}

func parseRecorderBodyAsInt(t *testing.T, context string, recorder *httptest.ResponseRecorder) int {
	bodyString := recorder.Body.String()
	id, err := strconv.Atoi(strings.TrimSpace(bodyString))
	if err != nil {
		t.Errorf("[%s] could not parse body %s as int: %s", context, bodyString, err)
	}
	return id
}

func parseBodyAsInt(t *testing.T, context string, f func() (*http.Response, error)) int {
	resp, err := f()
	if err != nil {
		t.Errorf("[%s] response returned err %s", context, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("[%s] could not parse body %s", context, err)
	}

	bodyStr := string(bodyBytes)

	id, err := strconv.Atoi(strings.TrimSpace(bodyStr))
	if err != nil {
		t.Errorf("[%s] could not parse body %s as int: %s", context, bodyStr, err)
	}

	return id
}

func parseBodyAsStr(t *testing.T, context string, f func() (*http.Response, error)) string {
	resp, err := f()
	if err != nil {
		t.Errorf("[%s] response returned err %s", context, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("[%s] could not parse body %s", context, err)
	}

	return string(bodyBytes)
}

func parseBodyAsJson(t *testing.T, context string, v any, f func() (*http.Response, error)) {
	resp, err := f()
	if err != nil {
		t.Errorf("[%s] response returned err %s", context, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("[%s] could not parse body %s", context, err)
	}

	err = json.Unmarshal(bodyBytes, v)
}

func assertAddHashReturnsId(t *testing.T, app *app.App, expectedId int) {
	request := httptest.NewRequest(http.MethodPost, "/hash", nil)
	responseRecorder := httptest.NewRecorder()

	app.PostHashEndpoint(responseRecorder, request)

	assert(t, "post statuscode", http.StatusCreated, responseRecorder.Code)

	id := parseRecorderBodyAsInt(t, "post id", responseRecorder)

	assert(t, "hash id", expectedId, id)
}
