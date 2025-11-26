package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "sync"
    "time"
)

type Game struct {
    ID        string          `json:"id"`
    Player1   string          `json:"player1"` // Telegram User ID
    Player2   string          `json:"player2"`
    Board     [19][19]string  `json:"board"`   // "B", "W", ""
    CurrentPlayer string      `json:"current_player"`
    Status    string          `json:"status"`  // "waiting", "playing", "finished"
    CreatedAt time.Time       `json:"created_at"`
}

type Challenge struct {
    ID          string    `json:"id"`
    FromUser    string    `json:"from_user"`
    ToUser      string    `json:"to_user"`
    Status      string    `json:"status"` // "pending", "accepted", "rejected"
    CreatedAt   time.Time `json:"created_at"`
}

var (
    games      = make(map[string]*Game)
    challenges = make(map[string]*Challenge)
    mutex      sync.RWMutex
)

func main() {
    http.HandleFunc("/api/challenge", handleChallenge)
    http.HandleFunc("/api/game", handleGame)
    http.HandleFunc("/api/game/move", handleMove)
    http.HandleFunc("/api/games", listGames)
    
    port := ":8080"
    log.Printf("Server starting on port %s", port)
    log.Fatal(http.ListenAndServe(port, nil))
}

// Бросить вызов
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

    json.NewEncoder(w).Encode(challenge)
}

// Принять/отклонить вызов и начать игру
func handleGame(w http.ResponseWriter, r *http.Request) {
    if r.Method == "POST" {
        var req struct {
            ChallengeID string `json:"challenge_id"`
            Action      string `json:"action"` // "accept" or "reject"
        }
        
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        mutex.Lock()
        defer mutex.Unlock()

        challenge, exists := challenges[req.ChallengeID]
        if !exists {
            http.Error(w, "Challenge not found", http.StatusNotFound)
            return
        }

        if req.Action == "accept" {
            // Создаем новую игру
            gameID := generateID()
            game := &Game{
                ID:           gameID,
                Player1:      challenge.FromUser,
                Player2:      challenge.ToUser,
                Board:        [19][19]string{},
                CurrentPlayer: "B", // Black starts
                Status:       "playing",
                CreatedAt:    time.Now(),
            }
            games[gameID] = game
            
            challenge.Status = "accepted"
            json.NewEncoder(w).Encode(game)
        } else {
            challenge.Status = "rejected"
            json.NewEncoder(w).Encode(challenge)
        }
    } else if r.Method == "GET" {
        // Получить информацию об игре
        gameID := r.URL.Query().Get("id")
        mutex.RLock()
        game, exists := games[gameID]
        mutex.RUnlock()
        
        if !exists {
            http.Error(w, "Game not found", http.StatusNotFound)
            return
        }
        
        json.NewEncoder(w).Encode(game)
    }
}

// Сделать ход
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
        http.Error(w, "Game not found", http.StatusNotFound)
        return
    }

    // Проверка хода
    if game.CurrentPlayer != req.Player {
        http.Error(w, "Not your turn", http.StatusBadRequest)
        return
    }

    if game.Board[req.X][req.Y] != "" {
        http.Error(w, "Position already occupied", http.StatusBadRequest)
        return
    }

    // Выполняем ход
    game.Board[req.X][req.Y] = req.Player
    
    // Меняем игрока
    if req.Player == "B" {
        game.CurrentPlayer = "W"
    } else {
        game.CurrentPlayer = "B"
    }

    json.NewEncoder(w).Encode(game)
}

// Список игр пользователя
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

    json.NewEncoder(w).Encode(userGames)
}

func generateID() string {
    return fmt.Sprintf("%d", time.Now().UnixNano())
}