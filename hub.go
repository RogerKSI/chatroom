package main

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
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
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
