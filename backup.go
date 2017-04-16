package main

import (
	"encoding/json"
	"bytes"
	"errors"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

type BackupMessage struct {
	Hub_id  string  `json:"hubId"`
	Message []byte  `json:"message"`
}

type BackupSlave struct {
	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

type BackupClient struct {
	hub *BackupHub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

type BackupHub struct {
	id string

	// Registered clients.
	clients map[*BackupClient]bool

	// Inbound message from the clients.
	broadcast chan BackupMessage

	// Register requests from the clients.
	register chan *BackupClient

	// Unregister requests from clients.
	unregister chan *BackupClient
}

func newBackupHub() *BackupHub {
	return &BackupHub{
		broadcast:  make(chan BackupMessage),
		register:   make(chan *BackupClient),
		unregister: make(chan *BackupClient),
		clients:    make(map[*BackupClient]bool),
	}
}

func (h *BackupHub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			log.Println("a new backup server added")

			// Send chat log to the newly registered backup by going through the map
			RoomLog.RLock()
			for key, room := range RoomLog.v {
				for _, message := range room {
					backup_message, err := json.Marshal(BackupMessage{Hub_id: key, Message: message})
					if err != nil {
						backup_message = []byte("")
					}

					select {
					case client.send <- backup_message:
					default:
						close(client.send)
						delete(h.clients, client)
					}
				}
			}	
			RoomLog.RUnlock()

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				log.Println("a backup server left")
			}

		case message := <-h.broadcast:
			backup_message, err := json.Marshal(message)
			if err != nil {
				backup_message = []byte("")
			}

			for client := range h.clients {
				select {
				case client.send <- backup_message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

// readBackupMasterPump reads messages from the slave websocket which connects to the slave.
// In the current model, the backup process will not send anything to master thus 
// nothing will come in this pump.
func (c *BackupClient) readBackupMasterPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		// c.hub.broadcast <- message
		log.Println(message)
	}
}

// writeBackupMasterPump pumps backup messages from backupHub to the websocket connection.
func (c *BackupClient) writeBackupMasterPump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			err := c.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Println("write: ", err)
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

// serveBackupMaster runs on master process to handles websocket backup requests from the slave process.
func serveBackupMaster(hub *BackupHub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &BackupClient{hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client
	go client.writeBackupMasterPump()
	client.readBackupMasterPump()
}

// readBackupSlavePump reads messages from the websocket which connects to the master,
// unmarshals it, and records to the local log.
func (s *BackupSlave) readBackupSlavePump() {
	defer func() {
		s.conn.Close()
	}()
	for {
		_, message, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("error: %v", err)
			}
			break
		}
		var data BackupMessage
		err = json.Unmarshal(message, &data)
		if err != nil {
			log.Println("cannot unmarshal: ", err)
		}

		log.Println(string(data.Message))
		// Add new message to local log
		RoomLog.Lock()
		RoomLog.v[data.Hub_id] = append(RoomLog.v[data.Hub_id], data.Message)
		RoomLog.Unlock()
	}
}

// writeBackupSlavePump pumps messages to the websocket connection.
// In the current model, the backup process will not send anything back to master
// thus this method is not in used.
func (s *BackupSlave) writeBackupSlavePump() {
	/*defer func() {
		s.conn.Close()
	}()
	for {

	}*/
}

// connectToMaster try to establish a websocket connection between the slave process
// and the master process.
func connectToMaster(u url.URL, done chan error) {
	defer func() {
		done <- errors.New("Channel Closed")
	}()

	log.Printf("connecting to %s", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Println("dial:", err)
		return
	}
	log.Println("connect to master: SUCCESS")
	slave := &BackupSlave{conn: conn, send: make(chan []byte, 256)}
	go slave.writeBackupSlavePump()
	slave.readBackupSlavePump()

}

// Process with backup flag on will try to connect to the master process at
// the address provided. If the connection cannot establish, the new process
// will promate itself to be a new master process.
func startBackupSlave(u url.URL) {
	// An error will be pushed into this channel when the connection fails.
	// This channel can be used to count a number of time the connection to
	// the provied master address fails, in case we want to try reconnecting
	// X times.
	var done = make(chan error)

	go connectToMaster(u, done)
	if <-done != nil {
		//log.Printf("waiting to reconnect")
		//time.Sleep(10 * time.Second)
		log.Printf("cannot connect to the master, promoting itself to be a new master")
	}

}