package app

import (
	"encoding/json"
	"eoinisawesome.com/jumpcloud-takehome/hashes"
	"eoinisawesome.com/jumpcloud-takehome/middleware"
	"eoinisawesome.com/jumpcloud-takehome/stats"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type App struct {
	hashCmds chan interface{}
	statCmds chan interface{}
}

func AppRouter(shutdown chan int, hashDelay time.Duration) http.Handler {
	app := App{
		hashCmds: hashes.StartHashLoop(shutdown, hashDelay),
		statCmds: stats.StartStatsLoop(),
	}

	mux := http.NewServeMux()

	mux.Handle(
		"/hash",
		middleware.AccessLogging(
			app.appStatsFilter( // stats collection is first to cover the case of bad method, or bad form data
				middleware.AllowedMethodFilter(http.MethodPost,
					middleware.ParseFormFilter(
						http.HandlerFunc(app.postHashEndpoint))))))
	mux.Handle("/hash/",
		middleware.AccessLogging(
			middleware.AllowedMethodFilter(http.MethodGet,
				http.HandlerFunc(app.getHashEndpoint))))

	mux.Handle("/stats",
		middleware.AccessLogging(
			middleware.AllowedMethodFilter(http.MethodGet,
				http.HandlerFunc(app.getStatsEndpoint))))

	mux.Handle("/shutdown",
		middleware.AccessLogging(
			middleware.AllowedMethodFilter(http.MethodPost,
				http.HandlerFunc(app.postShutdownEndpoint))))

	return mux
}

func (a *App) postHashEndpoint(w http.ResponseWriter, r *http.Request) {
	plaintext := r.Form.Get("password")

	// Only use TrimSpace for detecting the empty string. Users should be
	// allowed to hash/encode a password with leading/trailing spaces
	// if they want to.
	if strings.TrimSpace(plaintext) == "" {
		log.Printf("password value in POST body is empty")
		http.Error(w, "Form variable password can not be empty", http.StatusBadRequest)
		return
	}

	resp := make(chan int)
	a.hashCmds <- hashes.HashCmdReserveId{
		Plaintext: plaintext,
		Resp:      resp,
	}

	id := <-resp
	if id <= 0 {
		log.Printf("rejected POST /hash because server is waiting to shutdown")
		http.Error(w, "Server Closing", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeBody(w, id)
}

func (a *App) getHashEndpoint(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/hash/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Printf("Unable to parse %s to int: %s", idStr, err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	resp := make(chan interface{})
	a.hashCmds <- hashes.HashCmdRetrieve{Id: id, Resp: resp}

	result := <-resp

	switch result.(type) {
	case hashes.HashRetrieveFound:
		hash := result.(hashes.HashRetrieveFound).Hash
		writeBody(w, hash)
	case hashes.HashRetrieveNotFound:
		http.Error(w, "Not Found", http.StatusNotFound)
	default:
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}

}

func (a *App) getStatsEndpoint(w http.ResponseWriter, _ *http.Request) {
	resp := make(chan int64)
	a.statCmds <- stats.StatCmdRetrieve{Resp: resp}
	totalRequests := <-resp
	totalLatency := <-resp

	var jsonStruct = stats.StatsJson{}
	if totalRequests > 0 {
		jsonStruct = stats.StatsJson{
			Total:   totalRequests,
			Average: totalLatency / totalRequests,
		}
	}

	jsonBytes, err := json.Marshal(jsonStruct)
	if err != nil {
		log.Printf("Unable to serialze StatsJson %s to bytes: %s", jsonStruct, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	writeBody(w, string(jsonBytes))
}

func (a *App) postShutdownEndpoint(_ http.ResponseWriter, _ *http.Request) {
	a.hashCmds <- hashes.HashCmdGracefulShutdown{}
}

func (a *App) appStatsFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		// run the metrics collection in a go routine so that we don't
		// block returning the response.
		go func() {
			a.statCmds <- stats.StatCmdRecordRequest{
				Latency: time.Now().Sub(start),
			}
		}()
	})
}

func writeBody(w http.ResponseWriter, obj any) {
	_, err := fmt.Fprint(w, obj)
	if err != nil {
		log.Printf("Unable to write %s to body: %s", obj, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
