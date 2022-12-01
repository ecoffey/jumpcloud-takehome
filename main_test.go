package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func assertAddHashReturnsId(t *testing.T, server *Server, expectedId int) {
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	responseRecorder := httptest.NewRecorder()

	server.addHash(responseRecorder, request)

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

func TestAddHashHandler(t *testing.T) {
	t.Run("first id returned should be 1", func(t *testing.T) {
		server := Server{cmds: startHashesStoreManager()}

		assertAddHashReturnsId(t, &server, 1)
	})

	t.Run("serial calls should return increasing ids", func(t *testing.T) {
		server := Server{cmds: startHashesStoreManager()}

		assertAddHashReturnsId(t, &server, 1)
		assertAddHashReturnsId(t, &server, 2)
		assertAddHashReturnsId(t, &server, 3)
		assertAddHashReturnsId(t, &server, 4)
	})
}
