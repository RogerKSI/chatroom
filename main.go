package main

import (
	"flag"
	"log"
	"net/http"

	"regexp"
)

var addr = flag.String("addr", ":8080", "http service address")

var validPath = regexp.MustCompile("^(/ws)?/([0-9]+)$")

func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	m := validPath.FindStringSubmatch(r.URL.Path)
	if m == nil {
		http.Error(w, "Not found", 404)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	http.ServeFile(w, r, "home.html")
}

func hubHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)
	m := validPath.FindStringSubmatch(r.URL.Path)
	if m == nil {
		http.NotFound(w, r)
		return
	}
	log.Println(m[2])
	id := m[2]

	hub := getHub(id)

	if hub == nil {
		hub = newHub(id)
		go hub.run()
	}
	serveWs(hub, w, r)
}

func main() {
	flag.Parse()
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws/", hubHandler)

	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
