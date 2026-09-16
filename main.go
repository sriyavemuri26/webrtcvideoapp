package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

// Signal Structure
type Signal struct {
	// From whom the signal is from
	From          string        `json:"from"`                    // Sender of the signal
	Type          string        `json:"type"`                    // Type of signal
	To            string        `json:"to,omitempty"`            // Optional : Receiver of the signal
	Data          interface{}   `json:"data,omitempty"`          // Optional :Answer/Offer
	ICECandidates []interface{} `json:"iceCandidates,omitempty"` // Optional :Array of ICECandidates
}

type Client struct {
	id       string
	conn     *websocket.Conn
	sendChan chan []byte
}

func (c *Client) writePump() {
	for msg := range c.sendChan {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			break
		}
	}
	c.conn.Close()
}

// A map of client IDs to WebSocket Connections
var (
	clients = make(map[string]*Client)
	mu      sync.Mutex
)

// WebSocket upgrader configuration
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	http.HandleFunc("/ws", handler)
	// Serve the static files
	http.Handle("/", http.FileServer(http.Dir("./front")))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	// Start the server
	log.Printf("Server started on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// Function where signals are processed back and forth
func handler(w http.ResponseWriter, r *http.Request) {

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading to WebSocket:", err)
		return
	}
	defer conn.Close()

	// Store ClientID
	clientID := conn.RemoteAddr().String()

	client := &Client{id: clientID, conn: conn, sendChan: make(chan []byte, 256)}
	go client.writePump()

	// Lock access to the clients map to prevent access concurrently
	mu.Lock()
	// Store WebSocket connection in the clients map
	clients[clientID] = client
	// Store the number of clients present
	numClients := len(clients)
	log.Println("Client connected:", clientID)
	log.Println("Number of clients connected:", numClients)
	mu.Unlock()

	// Send the clientID to the client
	sendSignal(client, Signal{
		From: clientID,
		Type: "client_id",
		Data: clientID,
	})

	// Begin creation of PeerConnections after more than
	// one client joins the server
	if numClients != 1 {
		mu.Lock()

		// Loop over the existing clients except the
		// current WebSocket connection
		for existingClientID, existingClient := range clients {
			if clientID != existingClientID {
				// Signal the creation of PeerConnection
				// to the client with data from other
				// clients to establish a flow between each client
				// with seperate PeerConnections

				sendSignal(existingClient, Signal{
					From: existingClientID,
					Type: "create_pc",
					To:   clientID,
				})
				sendSignal(client, Signal{
					From: clientID,
					Type: "create_pc",
					To:   existingClientID,
				})

				// Signal offer creation to the client that joined
				// and send to all clients in the map individually
				sendSignal(client, Signal{
					From: clientID,
					Type: "create_offer",
					To:   existingClientID,
				})
			}
		}
		mu.Unlock()

	}

	// Listen for incoming signals from the client
	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			removeClient(clientID)
			return
		}

		var signal Signal
		if err := json.Unmarshal(p, &signal); err != nil {
			log.Println("Error unmarshaling signal:", err)
			continue
		}
		log.Printf("Received signal type: %s", signal.Type)

		// Send signal to the appropriate client
		sendToClient(signal)
	}
}

// Function to send signal to a specific connection
func sendSignal(targetClient *Client, signal Signal) {
	if targetClient == nil {
		return
	}
	// Marshal the Signal struct to JSON
	signalJSON, err := json.Marshal(signal)
	if err != nil {
		log.Println("Error marshaling signal message:", err)
		return
	}

	// Send the signal to the client
	log.Printf("Sending signal type %s to client %s", signal.Type, targetClient.id)
	select {
	case targetClient.sendChan <- signalJSON:
	default:
		log.Println("Error sending signal: channel full or closed")
	}
}

// Function to send signal to a client based on the signal data
func sendToClient(signal Signal) {
	// Lock concurrent access
	mu.Lock()
	// Get respective WebSocket Connection
	client, ok := clients[signal.To]
	mu.Unlock()

	if !ok {
		log.Printf("Client with ID %s not found", signal.To)
		return
	}

	// Marshal the Signal struct to JSON
	signalJSON, err := json.Marshal(signal)
	if err != nil {
		log.Println("Error marshaling signal message:", err)
		return
	}

	// Send the signal to the client
	log.Printf("Sending signal type %s to client %s", signal.Type, signal.To)
	select {
	case client.sendChan <- signalJSON:
	default:
		log.Printf("Error sending signal to client %v: channel full", signal.To)
	}
}

// Function to remove a client from the map and send a signal
// to the connection on disconnection
func removeClient(clientID string) {
	// Lock concurrent access
	mu.Lock()
	// Check if client exists in map
	client, ok := clients[clientID]
	if ok {
		log.Printf("Removing client %v", clientID)

		// Delete client from clients map
		delete(clients, clientID)
		close(client.sendChan)
	}

	// Collect remaining clients to broadcast disconnection outside the lock
	var remainingClients []*Client
	for _, c := range clients {
		remainingClients = append(remainingClients, c)
	}
	mu.Unlock()

	if !ok {
		return
	}

	// Send "client_disconnect" signal to all clients
	disconnectSignal, _ := json.Marshal(Signal{
		Type: "client_disconnect",
		From: clientID,
	})
	for _, c := range remainingClients {
		select {
		case c.sendChan <- disconnectSignal:
		default:
		}
	}
}
