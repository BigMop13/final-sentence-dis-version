package main

import (
	"crypto/rand"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

const (
	targetSentence = "The quick brown fox jumps over the lazy dog."
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

type Message struct {
	Type           string       `json:"type"`
	ChannelID      string       `json:"channelID,omitempty"`
	Username       string       `json:"username,omitempty"`
	PlayerID       string       `json:"playerID,omitempty"`
	CurrentIndex   int          `json:"currentIndex,omitempty"`
	MistakeCount   int          `json:"mistakeCount,omitempty"`
	TargetSentence string       `json:"targetSentence,omitempty"`
	Players        []PlayerData `json:"players,omitempty"`
	Winner         string       `json:"winner,omitempty"`
}

type PlayerData struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	CurrentIndex int    `json:"currentIndex"`
	MistakeCount int    `json:"mistakeCount"`
}

type Player struct {
	ID           string
	Username     string
	CurrentIndex int
	MistakeCount int
	Conn         *websocket.Conn
	Send         chan []byte
	RoomID       string
}

type GameRoom struct {
	ChannelID      string
	Players        map[string]*Player
	TargetSentence string
	Status         string // "waiting", "playing", "finished"
	StartedBy      string
	mu             sync.RWMutex
}

type RoomManager struct {
	rooms map[string]*GameRoom
	mu    sync.RWMutex
}

var roomManager = &RoomManager{
	rooms: make(map[string]*GameRoom),
}

func (rm *RoomManager) GetOrCreateRoom(channelID string) *GameRoom {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if room, exists := rm.rooms[channelID]; exists {
		return room
	}

	room := &GameRoom{
		ChannelID:      channelID,
		Players:        make(map[string]*Player),
		TargetSentence: targetSentence,
		Status:         "waiting",
	}
	rm.rooms[channelID] = room
	return room
}

func (rm *RoomManager) RemovePlayer(roomID, playerID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	room, exists := rm.rooms[roomID]
	if !exists {
		return
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	if player, exists := room.Players[playerID]; exists {
		close(player.Send)
		delete(room.Players, playerID)
	}

	if len(room.Players) == 0 {
		delete(rm.rooms, roomID)
	}
}

func (rm *RoomManager) BroadcastToRoom(roomID string, message []byte) {
	rm.mu.RLock()
	room, exists := rm.rooms[roomID]
	rm.mu.RUnlock()

	if !exists {
		return
	}

	room.mu.RLock()
	defer room.mu.RUnlock()

	for _, player := range room.Players {
		select {
		case player.Send <- message:
		default:
			close(player.Send)
			delete(room.Players, player.ID)
		}
	}
}

func (room *GameRoom) GetPlayersData() []PlayerData {
	room.mu.RLock()
	defer room.mu.RUnlock()

	players := make([]PlayerData, 0, len(room.Players))
	for _, p := range room.Players {
		players = append(players, PlayerData{
			ID:           p.ID,
			Username:     p.Username,
			CurrentIndex: p.CurrentIndex,
			MistakeCount: p.MistakeCount,
		})
	}
	return players
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add ngrok bypass header to skip interstitial page
		w.Header().Set("ngrok-skip-browser-warning", "true")

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		// Removed X-Frame-Options - using CSP frame-ancestors instead (modern standard)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer-when-downgrade")

		// Add CSP header that allows Discord to embed in iframe and ngrok resources
		w.Header().Set("Content-Security-Policy", "frame-ancestors *; default-src 'self' 'unsafe-inline' 'unsafe-eval' data: blob: https:; script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.ngrok.com https://*.ngrok.com; style-src 'self' 'unsafe-inline' https://cdn.ngrok.com https://*.ngrok.com; img-src 'self' data: blob: https: https://ngrok.com https://*.ngrok.com; font-src 'self' data: https:;")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	var player *Player
	var room *GameRoom
	playerReady := make(chan bool, 1)

	go func() {
		defer func() {
			if player != nil && room != nil {
				roomManager.RemovePlayer(room.ChannelID, player.ID)
				playersData := room.GetPlayersData()
				msg := Message{
					Type:    "PlayerProgress",
					Players: playersData,
				}
				msgBytes, _ := json.Marshal(msg)
				roomManager.BroadcastToRoom(room.ChannelID, msgBytes)
			}
			conn.Close()
		}()

		for {
			_, messageBytes, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket error: %v", err)
				}
				break
			}

			var msg Message
			if err := json.Unmarshal(messageBytes, &msg); err != nil {
				log.Printf("Error unmarshaling message: %v", err)
				continue
			}

			switch msg.Type {
			case "JoinRoom":
				channelID := msg.ChannelID
				if channelID == "" {
					channelID = "default"
				}
				room = roomManager.GetOrCreateRoom(channelID)

				playerID := generatePlayerID()
				player = &Player{
					ID:           playerID,
					Username:     msg.Username,
					CurrentIndex: 0,
					MistakeCount: 0,
					Conn:         conn,
					Send:         make(chan []byte, 256),
					RoomID:       channelID,
				}

				room.mu.Lock()
				room.Players[playerID] = player
				room.mu.Unlock()

				// Signal that player is ready
				select {
				case playerReady <- true:
				default:
				}

				joinMsg := Message{
					Type:           "Joined",
					PlayerID:       playerID,
					TargetSentence: room.TargetSentence,
					Players:        room.GetPlayersData(),
				}
				joinMsgBytes, _ := json.Marshal(joinMsg)
				player.Send <- joinMsgBytes

				playersData := room.GetPlayersData()
				broadcastMsg := Message{
					Type:    "PlayerJoined",
					Players: playersData,
				}
				broadcastMsgBytes, _ := json.Marshal(broadcastMsg)
				roomManager.BroadcastToRoom(channelID, broadcastMsgBytes)

			case "StartGame":
				if player == nil || room == nil {
					continue
				}

				room.mu.Lock()
				if room.Status == "waiting" {
					room.Status = "playing"
					room.StartedBy = player.ID
				}
				room.mu.Unlock()

				startMsg := Message{
					Type:           "GameStarted",
					TargetSentence: room.TargetSentence,
				}
				startMsgBytes, _ := json.Marshal(startMsg)
				roomManager.BroadcastToRoom(room.ChannelID, startMsgBytes)

			case "ProgressUpdate":
				if player == nil || room == nil {
					continue
				}

				room.mu.Lock()
				player.CurrentIndex = msg.CurrentIndex
				player.MistakeCount = msg.MistakeCount
				room.mu.Unlock()

				playersData := room.GetPlayersData()
				progressMsg := Message{
					Type:    "PlayerProgress",
					Players: playersData,
				}
				progressMsgBytes, _ := json.Marshal(progressMsg)
				roomManager.BroadcastToRoom(room.ChannelID, progressMsgBytes)

			case "GameFinished":
				if player == nil || room == nil {
					continue
				}

				room.mu.Lock()
				room.Status = "finished"
				room.mu.Unlock()

				finishMsg := Message{
					Type:   "GameFinished",
					Winner: player.Username,
				}
				finishMsgBytes, _ := json.Marshal(finishMsg)
				roomManager.BroadcastToRoom(room.ChannelID, finishMsgBytes)
			}
		}
	}()

	<-playerReady

	ticker := time.NewTicker(54 * time.Second)
	defer ticker.Stop()

	for {
		if player == nil {
			return
		}

		select {
		case message, ok := <-player.Send:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func generatePlayerID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(6)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		// Fallback to time-based if crypto/rand fails
		for i := range b {
			b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		}
		return string(b)
	}
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create file server for static files
	fs := http.FileServer(http.Dir("./static"))

	handler := corsMiddleware(fs)

	http.HandleFunc("/health", healthCheckHandler)
	http.HandleFunc("/ws", handleWebSocket)
	http.Handle("/", handler)

	server := &http.Server{
		Addr: ":" + port,
	}

	go func() {
		log.Printf("HTTP server listening on :%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	log.Println("Server is running. Press Ctrl+C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	log.Println("Shutting down gracefully...")
	if err := server.Close(); err != nil {
		log.Printf("Error closing server: %v", err)
	}
}
