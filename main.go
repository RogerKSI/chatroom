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
	// log.Println(r.URL.Path)
	m := validPath.FindStringSubmatch(r.URL.Path)
	if m == nil {
		http.NotFound(w, r)
		return
	}
	id := m[2]

	hub := getHub(id)

	if hub == nil {
		hub = newHub(id)
		go hub.run()
	}
	serveWs(hub, w, r)
}

// RoomLog contains log in each room since it is created.
var RoomLog = SafeRoomLog{v: make(map[string][][]byte)}

// UserLog marks the last message the user read in each room.
var UserLog = SafeUserLog{v: make(map[UserKey]int)}

func main() {
	flag.Parse()
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws/", hubHandler)

	fs := http.FileServer(http.Dir("resources"))
	http.Handle("/resources/", http.StripPrefix("/resources/", fs))

	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
