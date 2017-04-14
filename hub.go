package main

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	id string

	// Registered clients.
	clients map[*Client]bool

	// Inbound message from the clients.
	broadcast chan []byte

	// Refister requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

// hubs keep track of all active hub
var hubs = make(map[string]*Hub)

func newHub(id string) *Hub {
	hubs[id] = &Hub{
		id:         id,
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
	return hubs[id]
}

func getHub(id string) *Hub {
	if hub, ok := hubs[id]; ok {
		return hub
	} else {
		return nil
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true

			// Sent chatlog to the new client
			// Warning: By flooding chatlog into the client send channel, the newly registered client will be terminated
			// if the chatlog is larger than channel buffer size
			RoomLog.RLock()
			
			UserLog.RLock()
			unread := UserLog.v[UserKey{client.id, h.id}]
			UserLog.RUnlock()

			for i, m := range RoomLog.v[h.id] {
				// Sent additional message to sperate read and unread message
				if i == unread {
					select {
					case client.send <- []byte("--unread message--"):
					default:
						close(client.send)
						delete(h.clients, client)
					}
				}

				select {
				case client.send <- m:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			RoomLog.RUnlock()
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			// Save message to RoomLog
			RoomLog.Lock()
			RoomLog.v[h.id] = append(RoomLog.v[h.id], message)
			RoomLog.Unlock()

			for client := range h.clients {
				select {
				case client.send <- message:
					// Set user's read log to the last message sent to client
					// This is supposed to be done after the client reads from channel 
					// to ensure that the message is sent to user.
					// Somewhat thead safe.
					UserLog.Lock()
					RoomLog.RLock()
					UserLog.v[UserKey{client.id, h.id}] = len(RoomLog.v[h.id])
					RoomLog.RUnlock()
					UserLog.Unlock()
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
