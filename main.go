package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
	
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	bot       *tgbotapi.BotAPI
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
	ChatID       int64         `json:"chat_id"`
}

type Challenge struct {
	ID        string    `json:"id"`
	FromUser  string    `json:"from_user"`
	FromName  string    `json:"from_name"`
	ToUser    string    `json:"to_user"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	// Инициализация бота
	var err error
	bot, err = tgbotapi.NewBotAPI("7870811469:AAEy5PaUbqhg-OjugPte-Gp4F0bSHUmZkSk")
	if err != nil {
		log.Panic(err)
	}

	// Настройка webhook (исправленная версия)
	webhookConfig := tgbotapi.NewWebhook("https://my-fourth-telegram-app-production.up.railway.app/telegram")
	_, err = bot.Request(webhookConfig)
	if err != nil {
		log.Panic(err)
	}

	// Роуты
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "🚀 Go Game Server is running!\n\nBot: @%s", bot.Self.UserName)
	})

	http.HandleFunc("/telegram", handleTelegramWebhook)
	http.HandleFunc("/api/challenge", handleChallenge)
	http.HandleFunc("/api/game", handleGame)
	http.HandleFunc("/api/game/move", handleMove)
	http.HandleFunc("/api/games", listGames)
	
	port := ":8080"
	log.Printf("Bot authorized as @%s", bot.Self.UserName)
	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func handleTelegramWebhook(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var update tgbotapi.Update
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&update); err != nil {
		log.Println("Webhook decode error:", err)
		return
	}

	if update.Message != nil {
		handleMessage(update.Message)
	}
}

func handleMessage(message *tgbotapi.Message) {
	log.Printf("Message from %s: %s", message.From.UserName, message.Text)

	switch message.Text {
	case "/start":
		msg := tgbotapi.NewMessage(message.Chat.ID, "🎮 Добро пожаловать в игру Го!\n\nКоманды:\n/challenge @username - бросить вызов\n/mygames - мои игры")
		bot.Send(msg)
		
	case "/mygames":
		userGames := getUserGames(fmt.Sprint(message.From.ID))
		if len(userGames) == 0 {
			msg := tgbotapi.NewMessage(message.Chat.ID, "У вас нет активных игр. Используйте /challenge @username чтобы начать!")
			bot.Send(msg)
		} else {
			msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Ваши активные игры: %d", len(userGames)))
			bot.Send(msg)
		}
		
	default:
		msg := tgbotapi.NewMessage(message.Chat.ID, "Используйте /start для начала")
		bot.Send(msg)
	}
}

// Вспомогательные функции
func getUserGames(userID string) []*Game {
	mutex.RLock()
	defer mutex.RUnlock()
	
	userGames := []*Game{}
	for _, game := range games {
		if game.Player1 == userID || game.Player2 == userID {
			userGames = append(userGames, game)
		}
	}
	return userGames
}

// API Handlers (упрощенные версии)
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

	// Упрощенная логика хода
	mutex.Lock()
	defer mutex.Unlock()

	game, exists := games[req.GameID]
	if !exists {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	if game.Board[req.X][req.Y] != "" {
		http.Error(w, "Position occupied", http.StatusBadRequest)
		return
	}

	game.Board[req.X][req.Y] = req.Player
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

	json.NewEncoder(w).Encode(userGames)
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}