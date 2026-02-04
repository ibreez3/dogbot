package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	telegrambotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/openclaw/go-openclaw/pkg/channels"
)

// Bot represents a Telegram bot instance
type Bot struct {
	config       *Config
	api          *telegrambotapi.BotAPI
	handler      *Handler
	messageHandler channels.MessageHandler
	running      bool
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewBot creates a new Telegram bot
func NewBot(config *Config) (*Bot, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	botAPI, err := telegrambotapi.NewBotAPI(config.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}

	bot := &Bot{
		config: config,
		api:    botAPI,
		handler: NewHandler(nil), // Will be set after bot is created
	}

	bot.handler.bot = bot

	return bot, nil
}

// Name returns channel name
func (b *Bot) Name() string {
	return "telegram"
}

// Start starts Telegram bot
func (b *Bot) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return fmt.Errorf("bot is already running")
	}

	b.ctx, b.cancel = context.WithCancel(ctx)
	b.running = true
	b.mu.Unlock()

	log.Printf("🤖 Starting Telegram bot...")

	// Enable debug mode if configured
	b.api.Debug = b.config.Debug

	// Get bot info
	botInfo, err := b.api.GetMe()
	if err != nil {
		return fmt.Errorf("failed to get bot info: %w", err)
	}

	log.Printf("✅ Telegram Bot initialized: @%s (ID: %d)", botInfo.UserName, botInfo.ID)

	// Start handler
	b.handler.Start(b.ctx)

	// Choose mode based on configuration
	if b.config.UseWebhook {
		if err := b.startWebhook(); err != nil {
			return err
		}
	} else {
		if err := b.startLongPolling(); err != nil {
			return err
		}
	}

	log.Printf("🚀 Telegram bot started (mode: %s)", b.getModeName())

	return nil
}

// Stop stops Telegram bot
func (b *Bot) Stop(ctx context.Context) error {
	log.Println("🛑 Stopping Telegram bot...")

	b.cancel()

	// Stop handler
	b.handler.Stop()

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("✅ Telegram bot stopped")
		return nil
	case <-time.After(10 * time.Second):
		log.Println("⚠️  Telegram bot stop timeout")
		return fmt.Errorf("timeout waiting for bot to stop")
	}
}

// Send sends a message to specified target
func (b *Bot) Send(ctx context.Context, target string, content string, options map[string]interface{}) error {
	// Parse target (chat ID)
	var chatID int64
	_, err := fmt.Sscanf(target, "%d", &chatID)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}

	// Create send options
	sendOpts := &SendOptions{}
	if options != nil {
		if parseMode, ok := options["parse_mode"].(string); ok {
			sendOpts.ParseMode = parseMode
		}
		if replyTo, ok := options["reply_to"].(float64); ok {
			sendOpts.ReplyTo = int64(replyTo)
		}
		if disablePreview, ok := options["disable_preview"].(bool); ok {
			sendOpts.DisableWebPagePreview = disablePreview
		}
		if disableNotification, ok := options["disable_notification"].(bool); ok {
			sendOpts.DisableNotification = disableNotification
		}
	}

	return b.SendMessage(ctx, chatID, content, sendOpts)
}

// SendMessage sends a text message to a Telegram chat
func (b *Bot) SendMessage(ctx context.Context, chatID int64, text string, opts *SendOptions) error {
	if text == "" {
		return fmt.Errorf("message text cannot be empty")
	}

	// Limit message length (Telegram max is 4096 characters)
	const maxLength = 4096
	if len(text) > maxLength {
		text = text[:maxLength-3] + "..."
	}

	// Create message
	msg := telegrambotapi.NewMessage(chatID, text)

	// Apply options
	if opts != nil {
		if opts.ParseMode != "" {
			msg.ParseMode = opts.ParseMode
		}
		if opts.ReplyTo != 0 {
			msg.ReplyToMessageID = int(opts.ReplyTo)
		}
		if opts.DisableWebPagePreview {
			msg.DisableWebPagePreview = opts.DisableWebPagePreview
		}
		if opts.DisableNotification {
			msg.DisableNotification = opts.DisableNotification
		}
	}

	// Send message
	_, err := b.api.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	if b.config.Debug {
		log.Printf("📤 Sent message to chat %d: %s", chatID, truncateString(text, 50))
	}

	return nil
}

// SetMessageHandler sets handler for incoming messages
func (b *Bot) SetMessageHandler(handler channels.MessageHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messageHandler = handler
}

// GetMessageHandler returns current message handler
func (b *Bot) GetMessageHandler() channels.MessageHandler {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.messageHandler
}

// IsRunning returns true if bot is running
func (b *Bot) IsRunning() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.running
}

// Status returns current status of bot
func (b *Bot) Status() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.running {
		return "running"
	}
	return "stopped"
}

// startLongPolling starts bot in long polling mode
func (b *Bot) startLongPolling() error {
	u := telegrambotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()

		for {
			select {
			case <-b.ctx.Done():
				return
			case update, ok := <-updates:
				if !ok {
					return
				}
				b.handleUpdate(&update)
			}
		}
	}()

	return nil
}

// startWebhook starts bot in webhook mode
func (b *Bot) startWebhook() error {
	// Set webhook
	webhookURL, err := url.Parse(b.config.WebhookURL)
	if err != nil {
		return fmt.Errorf("failed to parse webhook URL: %w", err)
	}
	_, err = b.api.Request(telegrambotapi.WebhookConfig{
		URL: webhookURL,
	})
	if err != nil {
		return fmt.Errorf("failed to set webhook: %w", err)
	}

	// Create webhook server
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()

		srv := &http.Server{
			Addr:    fmt.Sprintf(":%d", b.config.WebhookPort),
			Handler: b,
		}

		log.Printf("🪝 Webhook server listening on :%d", b.config.WebhookPort)

		go func() {
			<-b.ctx.Done()
			srv.Shutdown(context.Background())
		}()

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("❌ Webhook server error: %v", err)
		}
	}()

	return nil
}

// ServeHTTP implements http.Handler for webhook mode
func (b *Bot) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	update, err := b.handleWebhook(w, r)
	if err != nil {
		return
	}

	b.handleUpdate(update)
}

// handleWebhook processes incoming webhook requests
func (b *Bot) handleWebhook(w http.ResponseWriter, r *http.Request) (*telegrambotapi.Update, error) {
	defer r.Body.Close()

	var update telegrambotapi.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil, err
	}

	w.WriteHeader(http.StatusOK)
	return &update, nil
}

// handleUpdate processes a single update
func (b *Bot) handleUpdate(update *telegrambotapi.Update) {
	// Handle callback queries
	if update.CallbackQuery != nil {
		b.handler.HandleCallbackQuery(update)
		return
	}

	// Handle messages
	if update.Message != nil {
		b.handler.HandleUpdate(update)
		return
	}

	// Handle other update types (channel posts, edited messages, etc.)
	if b.config.Debug {
		log.Printf("ℹ️  Received update type: %+v", update)
	}
}

// getModeName returns name of current mode
func (b *Bot) getModeName() string {
	if b.config.UseWebhook {
		return "webhook"
	}
	return "long polling"
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
