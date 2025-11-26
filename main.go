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
	bot *tgbotapi.BotAPI
	games = make(map[string]*Game)
	challenges = make(map[string]*Challenge)
	mutex sync.RWMutex
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
	ToName    string    `json:"to_name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	ChatID    int64     `json:"chat_id"`
}

func main() {
	// Инициализация бота
	var err error
	bot, err = tgbotapi.NewBotAPI("7870811469:AAEy5PaUbqhg-OjugPte-Gp4F0bSHUmZkSk")
	if err != nil {
		log.Printf("Bot init error: %v", err)
		log.Println("Continuing without Telegram bot...")
	} else {
		log.Printf("Bot authorized as @%s", bot.Self.UserName)
		
		// Удаляем webhook если был (для чистоты)
		_, _ = bot.Request(tgbotapi.DeleteWebhookConfig{})
		
		// Запускаем поллинг в горутине (более надежно на Railway)
		go startBotPolling()
	}

	// HTTP роуты
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		botStatus := "Bot: Not available"
		if bot != nil {
			botStatus = fmt.Sprintf("Bot: @%s - Active", bot.Self.UserName)
		}
		fmt.Fprintf(w, "🚀 Go Game Server is running!\n\n%s\n\nAPI Endpoints:\n- POST /api/challenge\n- GET /api/game?id=123\n- POST /api/game/move\n- GET /api/games?user_id=123", botStatus)
	})

	http.HandleFunc("/api/challenge", handleChallenge)
	http.HandleFunc("/api/game", handleGame)
	http.HandleFunc("/api/game/move", handleMove)
	http.HandleFunc("/api/games", listGames)
	
	port := ":8080"
	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func startBotPolling() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)
	log.Println("Bot polling started...")

	for update := range updates {
		if update.Message != nil {
			handleMessage(update.Message)
		}
	}
}

func handleMessage(message *tgbotapi.Message) {
	log.Printf("Message from %s (%d): %s", message.From.UserName, message.From.ID, message.Text)

	userID := fmt.Sprint(message.From.ID)
	userName := message.From.UserName

	switch {
	case message.Text == "/start":
		msg := tgbotapi.NewMessage(message.Chat.ID, "🎮 Добро пожаловать в игру Го!\n\nКоманды:\n/challenge @username - бросить вызов\n/mygames - мои игры\n/board - показать доску")
		msg.ReplyMarkup = getMainKeyboard()
		bot.Send(msg)
		
	case message.Text == "/mygames":
		userGames := getUserGames(userID)
		if len(userGames) == 0 {
			msg := tgbotapi.NewMessage(message.Chat.ID, "У вас нет активных игр. Используйте /challenge @username чтобы начать!")
			bot.Send(msg)
		} else {
			msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Ваши активные игры: %d\nИспользуйте /board чтобы посмотреть доску", len(userGames)))
			bot.Send(msg)
		}
		
	case message.Text == "/board":
		sendBoard(message.Chat.ID, userID)
		
	case message.Text == "🎮 Начать игру":
		msg := tgbotapi.NewMessage(message.Chat.ID, "Чтобы бросить вызов, используйте команду:\n/challenge @username\n\nИли выберите опцию ниже:")
		msg.ReplyMarkup = getMainKeyboard()
		bot.Send(msg)
		
	case message.Text == "📊 Мои игры":
		userGames := getUserGames(userID)
		if len(userGames) == 0 {
			msg := tgbotapi.NewMessage(message.Chat.ID, "У вас нет активных игр. Используйте /challenge @username чтобы начать!")
			bot.Send(msg)
		} else {
			msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Ваши активные игры: %d", len(userGames)))
			bot.Send(msg)
		}
		
	case len(message.Text) > 11 && message.Text[:11] == "/challenge ":
		if len(message.Text) > 12 && message.Text[11] == '@' {
			targetUsername := message.Text[12:]
			createChallenge(userID, userName, targetUsername, message.Chat.ID)
		} else {
			msg := tgbotapi.NewMessage(message.Chat.ID, "Используйте: /challenge @username")
			bot.Send(msg)
		}
		
	default:
		msg := tgbotapi.NewMessage(message.Chat.ID, "Используйте /start для списка команд или выберите опцию:")
		msg.ReplyMarkup = getMainKeyboard()
		bot.Send(msg)
	}
}

func getMainKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🎮 Начать игру"),
			tgbotapi.NewKeyboardButton("📊 Мои игры"),
		),
	)
}

func createChallenge(fromUserID, fromUserName, toUsername string, chatID int64) {
	challengeID := generateID()
	challenge := &Challenge{
		ID:        challengeID,
		FromUser:  fromUserID,
		FromName:  fromUserName,
		ToUser:    "", // Пока неизвестно
		ToName:    toUsername,
		Status:    "pending",
		CreatedAt: time.Now(),
		ChatID:    chatID,
	}

	mutex.Lock()
	challenges[challengeID] = challenge
	mutex.Unlock()

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("🎯 Вызов отправлен пользователю @%s!\nОжидайте подтверждения.", toUsername))
	bot.Send(msg)
}

func sendBoard(chatID int64, userID string) {
	userGames := getUserGames(userID)
	if len(userGames) == 0 {
		// Создаем тестовую игру для демонстрации
		gameID := generateID()
		game := &Game{
			ID:            gameID,
			Player1:       userID,
			Board:         [19][19]string{},
			CurrentPlayer: "B",
			Status:        "playing",
			CreatedAt:     time.Now(),
			ChatID:        chatID,
		}
		// Добавляем несколько тестовых камней
		game.Board[3][3] = "B"
		game.Board[3][4] = "W"
		game.Board[4][3] = "W"
		game.Board[4][4] = "B"
		
		mutex.Lock()
		games[gameID] = game
		mutex.Unlock()
		
		userGames = []*Game{game}
	}

	// Берем первую игру
	game := userGames[0]
	
	// Создаем простое текстовое представление доски (9x9 для читаемости)
	boardText := "⚫️⚪️ *Доска Го (9x9)*:\n\n"
	boardText += "🔢1️⃣2️⃣3️⃣4️⃣5️⃣6️⃣7️⃣8️⃣9️⃣\n"
	
	for y := 0; y < 9; y++ {
		boardText += string(rune('Ⓐ' + y)) + " "
		for x := 0; x < 9; x++ {
			switch game.Board[x][y] {
			case "B":
				boardText += "⚫️"
			case "W":
				boardText += "⚪️"
			default:
				boardText += "➕"
			}
		}
		boardText += "\n"
	}
	
	boardText += fmt.Sprintf("\nСейчас ход: %s", getPlayerColor(game.CurrentPlayer))
	boardText += "\n\nЧтобы сделать ход, используйте API или напишите координаты (например: A1)"
	
	msg := tgbotapi.NewMessage(chatID, boardText)
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

func getPlayerColor(player string) string {
	if player == "B" {
		return "⚫️ Черные"
	}
	return "⚪️ Белые"
}

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

// API Handlers
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