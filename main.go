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
	ID           string      `json:"id"`
	Player1      string      `json:"player1"` // Telegram User ID
	Player2      string      `json:"player2"`
	Board        [19][19]string `json:"board"`
	CurrentPlayer string     `json:"current_player"`
	Status       string      `json:"status"`
	CreatedAt    time.Time   `json:"created_at"`
	ChatID       int64       `json:"chat_id"` // Для отправки сообщений
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

	// Настройка webhook
	webhookURL := "https://my-fourth-telegram-app-production.up.railway.app/telegram"
	_, err = bot.SetWebhook(tgbotapi.NewWebhook(webhookURL))
	if err != nil {
		log.Panic(err)
	}

	// Роуты
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
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
	update, err := bot.HandleUpdate(r)
	if err != nil {
		log.Println("Webhook error:", err)
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
		// Показать активные игры пользователя
		userGames := getUserGames(fmt.Sprint(message.From.ID))
		if len(userGames) == 0 {
			msg := tgbotapi.NewMessage(message.Chat.ID, "У вас нет активных игр. Используйте /challenge @username чтобы начать!")
			bot.Send(msg)
		} else {
			msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Ваши активные игры: %d", len(userGames)))
			bot.Send(msg)
		}
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

// Остальные функции (handleChallenge, handleGame, handleMove, listGames) остаются без изменений
// ... [остальной код из предыдущей версии]