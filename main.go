package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Signal Structure
type Signal struct {
	// From whom the signal is from
	From          string        `json:"from"` // Sender of the signal
	Type          string        `json:"type"` // Type of signal
	To            string        `json:"to,omitempty"` // Optional : Receiver of the signal
	Data          interface{}   `json:"data,omitempty"` // Optional :Answer/Offer 
	ICECandidates []interface{} `json:"iceCandidates,omitempty"` // Optional :Array of ICECandidates
}

// A map of client IDs to WebSocket Connections
var (
	clients = make(map[string]*websocket.Conn)
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
	// Start the server on port 8080
	log.Println("Server started on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
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

	// Lock access to the clients map to prevent access concurrently
	mu.Lock()
	// Store WebSocket connection in the clients map
	clients[clientID] = conn
	// Store the number of clients present
	numClients := len(clients)
	log.Println("Client connected:", clientID)
	log.Println("Number of clients connected:", numClients)
	mu.Unlock()

	// Send the clientID to the client
	sendSignal(conn, Signal{
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
		for clientId, client := range clients {
			if clientID != clientId {
				// Signal the creation of PeerConnection 
				// to the client with data from other
				// clients to establish a flow between each client
				// with seperate PeerConnections
				sendSignal(client, Signal{
					From: clientId,
					Type: "create_pc",
                    To: clientID,
				})
				sendSignal(conn, Signal{
					From: clientID,
					Type: "create_pc",
                    To: clientId,
				})

				// Signal offer creation to the client that joined
				// and send to all clients in the map individually
                sendSignal(conn, Signal{
                    From: clientID,
                    Type: "create_offer",
                    To: clientId,
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
func sendSignal(conn *websocket.Conn, signal Signal) {
	// Marshal the Signal struct to JSON
	signalJSON, err := json.Marshal(signal)
	if err != nil {
		log.Println("Error marshaling signal message:", err)
		return
	}

	// Get client ID from connection
	clientID := conn.RemoteAddr().String()

	// Send the signal to the client
	log.Printf("Sending signal type %s to client %s", signal.Type, clientID)
	if err := conn.WriteMessage(websocket.TextMessage, signalJSON); err != nil {
		log.Println("Error sending signal:", err)
		removeClient(clientID)
	}
}

// Function to send signal to a client based on the signal data
func sendToClient(signal Signal) {
	// Lock concurrent access
    mu.Lock()
    defer mu.Unlock()

	// Marshal the Signal struct to JSON
    signalJSON, err := json.Marshal(signal)
    if err != nil {
        log.Println("Error marshaling signal message:", err)
        return
    }

	// Get respective WebSocket Connection
    client, ok := clients[signal.To]
    if !ok {
        log.Printf("Client with ID %s not found", signal.To)
        return
    }

	// Send the signal to the client
    log.Printf("Sending signal type %s to client %s", signal.Type, signal.To)
    if err := client.WriteMessage(websocket.TextMessage, signalJSON); err != nil {
        log.Printf("Error sending signal to client %v: %v", signal.To, err)
        removeClient(signal.To)
    }
}

// Function to remove a client from the map and send a signal
// to the connection on disconnection
func removeClient(clientID string) {
	// Lock concurrent access
	mu.Lock()
	defer mu.Unlock()

	// Check if client exists in map
	if _, ok := clients[clientID]; ok {
		log.Printf("Removing client %v", clientID)

		// Delete client from clients map
		delete(clients, clientID)

		// Send "client_disconnect" signal to all clients
        for _, client := range clients {
            sendSignal(client, Signal{
                Type: "client_disconnect",
                From: clientID,
            })
        }
	}
}
