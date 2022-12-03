package app

import (
	"encoding/json"
	"eoinisawesome.com/jumpcloud-takehome/hashes"
	"eoinisawesome.com/jumpcloud-takehome/middleware"
	"eoinisawesome.com/jumpcloud-takehome/stats"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type App struct {
	HashCmds chan interface{}
	StatCmds chan interface{}
}

func (a *App) PostHashEndpoint(w http.ResponseWriter, r *http.Request) {
	resp := make(chan int)
	a.HashCmds <- hashes.HashCmdReserveId{
		Plaintext: r.Form.Get("password"),
		Resp:      resp,
	}
	id := <-resp
	// TODO if id < 0 then return nack or error status
	fmt.Fprintf(w, "%d", id)
}

func (a *App) GetHashEndpoint(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/hash/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	resp := make(chan string)
	a.HashCmds <- hashes.HashCmdRetrieve{Id: id, Resp: resp}
	hash := <-resp
	fmt.Fprintf(w, "%s", hash)
}

func (a *App) GetStatsEndpoint(w http.ResponseWriter, r *http.Request) {
	resp := make(chan int64)
	a.StatCmds <- stats.StatCmdRetrieve{Resp: resp}
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
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, string(jsonBytes))
}

func (a *App) postShutdownEndpoint(_ http.ResponseWriter, _ *http.Request) {
	a.HashCmds <- hashes.HashCmdGracefulShutdown{}
}

func (a *App) AppStatsFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		// run the metrics collection in a go routine so that we don't
		// block returning the response
		go func() {
			a.StatCmds <- stats.StatCmdRecordRequest{
				Latency: time.Now().Sub(start),
			}
		}()
	})
}

func Router(shutdown chan int, hashDelay time.Duration) http.Handler {
	app := App{
		HashCmds: hashes.StartHashLoop(shutdown, hashDelay),
		StatCmds: stats.StartStatsLoop(),
	}

	mux := http.NewServeMux()

	mux.Handle(
		"/hash",
		app.AppStatsFilter( // stats collection is first to cover the case of bad method, or bad form data
			middleware.AllowedMethodFilter(http.MethodPost,
				middleware.ParseFormFilter(
					http.HandlerFunc(app.PostHashEndpoint)))))
	mux.Handle("/hash/",
		middleware.AllowedMethodFilter(http.MethodGet,
			http.HandlerFunc(app.GetHashEndpoint)))

	mux.Handle("/stats",
		middleware.AllowedMethodFilter(http.MethodGet,
			http.HandlerFunc(app.GetStatsEndpoint)))

	mux.Handle("/shutdown",
		middleware.AllowedMethodFilter(http.MethodPost,
			http.HandlerFunc(app.postShutdownEndpoint)))

	return mux
}
