package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

var (
	games      = make(map[string]*Game)
	challenges = make(map[string]*Challenge)
	mutex      sync.RWMutex
)

type Game struct {
	ID           string        `json:"id"`
	Player1      string        `json:"player1"`
	Player2      string        `json:"player2"`
	Board        [19][19]string `json:"board"`
	CurrentPlayer string       `json:"current_player"`
	Status       string        `json:"status"`
	CreatedAt    time.Time     `json:"created_at"`
}

type Challenge struct {
	ID        string    `json:"id"`
	FromUser  string    `json:"from_user"`
	ToUser    string    `json:"to_user"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	// Простые HTTP роуты
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "🚀 Go Game Server is running!\n\nAPI Endpoints:\n- POST /api/challenge\n- GET /api/game?id=123\n- POST /api/game/move\n- GET /api/games?user_id=123")
	})

	http.HandleFunc("/api/challenge", handleChallenge)
	http.HandleFunc("/api/game", handleGame)
	http.HandleFunc("/api/game/move", handleMove)
	http.HandleFunc("/api/games", listGames)
	
	port := ":8080"
	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func handleChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FromUser string `json:"from_user"`
		ToUser   string `json:"to_user"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	challengeID := generateID()
	challenge := &Challenge{
		ID:        challengeID,
		FromUser:  req.FromUser,
		ToUser:    req.ToUser,
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	mutex.Lock()
	challenges[challengeID] = challenge
	mutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(challenge)
}

func handleGame(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		gameID := r.URL.Query().Get("id")
		mutex.RLock()
		game, exists := games[gameID]
		mutex.RUnlock()
		
		if !exists {
			http.Error(w, "Game not found", http.StatusNotFound)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(game)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		GameID string `json:"game_id"`
		Player string `json:"player"`
		X      int    `json:"x"`
		Y      int    `json:"y"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	game, exists := games[req.GameID]
	if !exists {
		// Если игры нет - создаем новую
		game = &Game{
			ID:            req.GameID,
			Player1:       req.Player,
			Board:         [19][19]string{},
			CurrentPlayer: "B",
			Status:        "playing",
			CreatedAt:     time.Now(),
		}
		games[req.GameID] = game
	}

	if req.X < 0 || req.X >= 19 || req.Y < 0 || req.Y >= 19 {
		http.Error(w, "Invalid coordinates", http.StatusBadRequest)
		return
	}

	if game.Board[req.X][req.Y] != "" {
		http.Error(w, "Position occupied", http.StatusBadRequest)
		return
	}

	game.Board[req.X][req.Y] = req.Player
	
	// Меняем игрока
	if req.Player == "B" {
		game.CurrentPlayer = "W"
	} else {
		game.CurrentPlayer = "B"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(game)
}

func listGames(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	
	mutex.RLock()
	defer mutex.RUnlock()

	userGames := []*Game{}
	for _, game := range games {
		if game.Player1 == userID || game.Player2 == userID {
			userGames = append(userGames, game)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userGames)
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}