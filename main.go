package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

const (
	// CommandPrefix is the prefix for bot commands
	CommandPrefix = "!"
	StartCommand  = "start"
	GameTimeout   = 10 * time.Second
)

var sentences = []string{
	"The quick brown fox jumps over the lazy dog",
	"Discord API is fun",
	"Golang is fast",
	"Speed typing is challenging",
	"Practice makes perfect",
	"Code with confidence",
	"Discord bots are awesome",
	"Go routines are powerful",
	"Clean code matters",
	"Type fast and accurate",
}

type GameState struct {
	TargetSentence string
	UserID         string
	ChannelID      string
	StartTime      time.Time
}

type GameManager struct {
	games map[string]*GameState // Key: userID+channelID
	mu    sync.RWMutex
}

func NewGameManager() *GameManager {
	return &GameManager{
		games: make(map[string]*GameState),
	}
}

func getGameKey(userID, channelID string) string {
	return userID + ":" + channelID
}

func (gm *GameManager) StartGame(userID, channelID, sentence string) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	key := getGameKey(userID, channelID)
	gm.games[key] = &GameState{
		TargetSentence: sentence,
		UserID:         userID,
		ChannelID:      channelID,
		StartTime:      time.Now(),
	}
}

func (gm *GameManager) GetGame(userID, channelID string) (*GameState, bool) {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	key := getGameKey(userID, channelID)
	game, exists := gm.games[key]
	return game, exists
}

func (gm *GameManager) EndGame(userID, channelID string) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	key := getGameKey(userID, channelID)
	delete(gm.games, key)
}

func (gm *GameManager) HasGame(userID, channelID string) bool {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	key := getGameKey(userID, channelID)
	_, exists := gm.games[key]
	return exists
}

func loadConfig() (string, error) {
	_ = godotenv.Load()

	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		return "", fmt.Errorf("DISCORD_TOKEN environment variable is not set")
	}

	return token, nil
}

func pickRandomSentence() string {
	return sentences[rand.Intn(len(sentences))]
}

func startTimeoutGoroutine(s *discordgo.Session, gm *GameManager, userID, channelID string, expectedStartTime time.Time, ctx context.Context) {
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(GameTimeout):
			game, exists := gm.GetGame(userID, channelID)
			if exists && game.StartTime.Equal(expectedStartTime) {
				duration := time.Since(game.StartTime)
				log.Printf("[GAME_TIMEOUT] UserID=%s ChannelID=%s TargetSentence=%q Duration=%.2fs",
					userID, channelID, game.TargetSentence, duration.Seconds())
				_, err := s.ChannelMessageSend(channelID, "⏰ Time's up! You lost.")
				if err != nil {
					log.Printf("Error sending timeout message: %v", err)
				}
				gm.EndGame(userID, channelID)
			}
		}
	}()
}

func main() {
	token, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	s, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalf("Failed to create Discord session: %v", err)
	}

	gameManager := NewGameManager()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Bot is ready! Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	})

	s.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID {
			return
		}

		if strings.HasPrefix(m.Content, CommandPrefix+StartCommand) {
			if gameManager.HasGame(m.Author.ID, m.ChannelID) {
				_, err := s.ChannelMessageSend(m.ChannelID, "You already have an active game! Finish it first.")
				if err != nil {
					log.Printf("Error sending message: %v", err)
				}
				return
			}

			sentence := pickRandomSentence()

			log.Printf("[GAME_START] UserID=%s Username=%s ChannelID=%s TargetSentence=%q",
				m.Author.ID, m.Author.Username, m.ChannelID, sentence)

			msg := fmt.Sprintf("You have 10 seconds to retype this!\n\n**%s**", sentence)
			_, err := s.ChannelMessageSend(m.ChannelID, msg)
			if err != nil {
				log.Printf("Error sending challenge message: %v", err)
				return
			}

			gameManager.StartGame(m.Author.ID, m.ChannelID, sentence)

			game, _ := gameManager.GetGame(m.Author.ID, m.ChannelID)

			// start timeout goroutine
			startTimeoutGoroutine(s, gameManager, m.Author.ID, m.ChannelID, game.StartTime, ctx)

			return
		}

		game, hasGame := gameManager.GetGame(m.Author.ID, m.ChannelID)
		if !hasGame {
			return
		}

		userInput := strings.TrimSpace(m.Content)
		timeSinceStart := time.Since(game.StartTime)
		log.Printf("[USER_INPUT] UserID=%s Username=%s ChannelID=%s Input=%q TargetSentence=%q TimeSinceStart=%.2fs",
			m.Author.ID, m.Author.Username, m.ChannelID, userInput, game.TargetSentence, timeSinceStart.Seconds())

		if userInput == game.TargetSentence {
			duration := time.Since(game.StartTime)
			log.Printf("[GAME_WIN] UserID=%s Username=%s ChannelID=%s TargetSentence=%q Duration=%.2fs",
				m.Author.ID, m.Author.Username, m.ChannelID, game.TargetSentence, duration.Seconds())
			_, err := s.ChannelMessageSend(m.ChannelID, "✅ You won! Great speed!")
			if err != nil {
				log.Printf("Error sending win message: %v", err)
			}
			gameManager.EndGame(m.Author.ID, m.ChannelID)
		} else {
			log.Printf("[GAME_WRONG] UserID=%s Username=%s ChannelID=%s Input=%q Expected=%q",
				m.Author.ID, m.Author.Username, m.ChannelID, userInput, game.TargetSentence)
			_, err := s.ChannelMessageSend(m.ChannelID, "❌ Wrong text! Try again!")
			if err != nil {
				log.Printf("Error sending wrong message: %v", err)
			}
		}
	})

	s.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent

	err = s.Open()
	if err != nil {
		log.Fatalf("Failed to open Discord session: %v", err)
	}
	defer s.Close()

	log.Println("Bot is running. Press Ctrl+C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	cancel()

	log.Println("Shutting down gracefully...")
}
