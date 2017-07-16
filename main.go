package main

import (
	"io/ioutil"
	"net/http"

	"os"

	"github.com/NYTimes/gziphandler"
	"github.com/gorilla/mux"
	"github.com/unrolled/render"
)

var (
	renderer = render.New()
)

func main() {
	config := Config{}
	if err := config.LoadFromFile(configPath); err != nil {
		println("Load config error,", err.Error())
		os.Exit(1)
	}

	_, err := ioutil.ReadFile("./index.html")
	if err != nil {
		println("Load index.html error,", err.Error())
		os.Exit(1)
	}

	r := mux.NewRouter()
	r = r.StrictSlash(true)
	if _, err := evernoteRouter(&config, r.PathPrefix("/evernote").Subrouter()); err != nil {
		println("Initial error,", err.Error())
		os.Exit(1)
	}

	if _, err := bloggerRouter(&config, r.PathPrefix("/blogger").Subrouter()); err != nil {
		println("Initial error,", err.Error())
		os.Exit(1)
	}

	r.HandleFunc("/favicon.ico", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("CONTENT-TYPE", "image/x-icon")
		http.ServeFile(w, req, "favicon.ico")
	})

	r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if len(config.EvernoteAccessToken) <= 0 {
			http.Redirect(w, req, "/evernote/login", http.StatusTemporaryRedirect)
			return
		} else if len(config.GoogleAPIAccessToken) <= 0 {
			http.Redirect(w, req, "/blogger/login", http.StatusTemporaryRedirect)
			return
		}

		// Always read, TODO: file cache and watch
		spa, _ := ioutil.ReadFile("./index.html")
		w.Header().Set("CONTENT-TYPE", "text/html; charset=utf-8")
		w.Write(spa)
	})

	mh := gziphandler.GzipHandler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.ServeHTTP(w, req)
	}))

	println("Visit http://127.0.0.1" + config.Addr)

	if err := http.ListenAndServe(config.Addr, mh); err != nil {
		println("Init server error,", err.Error())
		os.Exit(1)
	}
}
