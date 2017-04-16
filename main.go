package main

import (
	"flag"
	"log"
	"net/http"
	"net/url"
	"regexp"
)

var addr = flag.String("addr", ":8080", "http service address")

// Address of master service
var backUpMasterAddr = flag.String("master_addr", "localhost:8080", "master http service address")

var backupFlag = flag.Bool("backup", false, "run in backup mode")

var validWsPath = regexp.MustCompile("^/ws/([0-9]+)$")
var validRoomPath = regexp.MustCompile("^/room/([0-9]+)$")

func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)

	if r.URL.Path != "/" {
		http.Error(w, "Not found", 404)
		return
	}

	if r.Method == "GET" {
		http.ServeFile(w, r, "home.html")
		return
		/*} else if r.Method == "POST" {
		username := r.FormValue("username")
		room := r.FormValue("room")
		*/
	} else {
		http.Error(w, "Method not allowed", 405)
		return
	}

}

func serveRoom(w http.ResponseWriter, r *http.Request) {
	//log.Println(r.URL)
	m := validRoomPath.FindStringSubmatch(r.URL.Path)
	if m == nil {
		http.Error(w, "Not found", 404)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	http.ServeFile(w, r, "chatroom.html")
}

func hubHandler(w http.ResponseWriter, r *http.Request, b chan BackupMessage) {
	// log.Println(r.URL.Path)
	m := validWsPath.FindStringSubmatch(r.URL.Path)
	if m == nil {
		http.NotFound(w, r)
		return
	}
	id := m[1]

	hub := getHub(id)

	if hub == nil {
		hub = newHub(id, b)
		go hub.run()
	}
	cookie, _ := r.Cookie("username")
	log.Println(cookie.Value)

	serveWs(cookie.Value, hub, w, r)
}

// RoomLog contains log in each room since it is created.
var RoomLog = SafeRoomLog{v: make(map[string][][]byte)}

// UserLog marks the last message the user read in each room.
var UserLog = SafeUserLog{v: make(map[UserKey]int)}

func main() {
	flag.Parse()

	// If the process fails to start as a backup process, it will promote itself
	// to be a new master process.
	if *backupFlag {
		u := url.URL{Scheme: "ws", Host: *backUpMasterAddr, Path: "/backup"}
		startBackupSlave(u)
	}

	// backupHub manages connections from backup processs.
	backupHub := newBackupHub()
	go backupHub.run()

	http.HandleFunc("/", serveHome)
	http.HandleFunc("/room/", serveRoom)
	http.HandleFunc("/backup", func (w http.ResponseWriter, r *http.Request) {
		serveBackupMaster(backupHub, w, r)
	})
	http.HandleFunc("/ws/", func (w http.ResponseWriter, r *http.Request) {
		hubHandler(w, r, backupHub.broadcast)
	})

	fs := http.FileServer(http.Dir("resources"))
	http.Handle("/resources/", http.StripPrefix("/resources/", fs))

	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
