package main

import (
	"encoding/json"
	"eoinisawesome.com/jumpcloud-takehome/app"
	"eoinisawesome.com/jumpcloud-takehome/stats"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestEndpoints(t *testing.T) {
	t.Run("serial calls should return increasing ids", func(t *testing.T) {
		httpServer := httptest.NewServer(app.AppRouter(make(chan int), 0))
		defer httpServer.Close()

		for i := 1; i <= 5; i++ {
			id := parseBodyAsInt(t, "POST /hash", func() (*http.Response, error) {
				return http.PostForm(httpServer.URL+"/hash", map[string][]string{"password": {"angryMonkey"}})
			})

			assert(t, "id", i, id)
		}
	})

	t.Run("hashing round trip", func(t *testing.T) {
		httpServer := httptest.NewServer(app.AppRouter(make(chan int), 0))
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
		httpServer := httptest.NewServer(app.AppRouter(make(chan int), 0))
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
		httpServer := httptest.NewServer(app.AppRouter(make(chan int), 0))
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
		httpServer := httptest.NewServer(app.AppRouter(shutdown, 0))

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

func parseBodyAsInt(t *testing.T, context string, f func() (*http.Response, error)) int {
	bodyBytes := parseBodyToBytes(t, context, f)

	bodyStr := string(bodyBytes)

	id, err := strconv.Atoi(strings.TrimSpace(bodyStr))
	if err != nil {
		t.Errorf("[%s] could not parse body %s as int: %s", context, bodyStr, err)
	}

	return id
}

func parseBodyAsStr(t *testing.T, context string, f func() (*http.Response, error)) string {
	bodyBytes := parseBodyToBytes(t, context, f)

	return string(bodyBytes)
}

func parseBodyAsJson(t *testing.T, context string, v any, f func() (*http.Response, error)) {
	bodyBytes := parseBodyToBytes(t, context, f)

	err := json.Unmarshal(bodyBytes, v)
	if err != nil {
		t.Errorf("[%s] could not unmarshal to json struct %s", context, err)
	}
}

func parseBodyToBytes(t *testing.T, context string, f func() (*http.Response, error)) []byte {
	resp, err := f()
	if err != nil {
		t.Errorf("[%s] response returned err %s", context, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("[%s] could not parse body %s", context, err)
	}

	return bodyBytes
}
