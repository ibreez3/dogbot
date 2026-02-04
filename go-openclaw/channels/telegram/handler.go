package channels

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	telegrambotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/openclaw/go-openclaw/pkg/channels"
)

// Handler handles incoming Telegram messages
type Handler struct {
	bot      *Bot
	incoming chan *IncomingMessage
	mu       sync.RWMutex
}

// NewHandler creates a new message handler
func NewHandler(bot *Bot) *Handler {
	return &Handler{
		bot:      bot,
		incoming: make(chan *IncomingMessage, 100),
	}
}

// Start starts the message handler
func (h *Handler) Start(ctx context.Context) {
	go h.processMessages(ctx)
}

// Stop stops the message handler
func (h *Handler) Stop() {
	close(h.incoming)
}

// HandleUpdate handles an incoming update from Telegram
func (h *Handler) HandleUpdate(update *telegrambotapi.Update) {
	if update.Message == nil {
		return
	}

	msg := h.convertMessage(update.Message)
	if msg == nil {
		return
	}

	select {
	case h.incoming <- msg:
	default:
		log.Printf("⚠️  Incoming message queue full, dropping message from %s", msg.From.String())
	}
}

// convertMessage converts a Telegram API message to our internal format
func (h *Handler) convertMessage(apiMsg *telegrambotapi.Message) *IncomingMessage {
	if apiMsg == nil {
		return nil
	}

	msg := &IncomingMessage{
		MessageID: int64(apiMsg.MessageID),
		Chat:      h.convertChat(apiMsg.Chat),
		Date:      time.Unix(int64(apiMsg.Date), 0),
		Text:      apiMsg.Text,
		Caption:   apiMsg.Caption,
	}

	if apiMsg.From != nil {
		msg.From = h.convertUser(apiMsg.From)
	}

	if apiMsg.ReplyToMessage != nil {
		msg.ReplyTo = h.convertMessage(apiMsg.ReplyToMessage)
	}

	if apiMsg.EditDate != 0 {
		editDate := time.Unix(int64(apiMsg.EditDate), 0)
		msg.EditDate = &editDate
	}

	msg.MediaGroupID = apiMsg.MediaGroupID

	return msg
}

// convertUser converts a Telegram API user to our internal format
func (h *Handler) convertUser(apiUser *telegrambotapi.User) *UserInfo {
	if apiUser == nil {
		return nil
	}
	return &UserInfo{
		ID:        apiUser.ID,
		FirstName: apiUser.FirstName,
		LastName:  apiUser.LastName,
		Username:  apiUser.UserName,
		IsBot:     apiUser.IsBot,
	}
}

// convertChat converts a Telegram API chat to our internal format
func (h *Handler) convertChat(apiChat *telegrambotapi.Chat) *ChatInfo {
	if apiChat == nil {
		return nil
	}
	return &ChatInfo{
		ID:        apiChat.ID,
		Type:      apiChat.Type,
		Title:     apiChat.Title,
		Username:  apiChat.UserName,
		FirstName: apiChat.FirstName,
		LastName:  apiChat.LastName,
	}
}

// processMessages processes incoming messages
func (h *Handler) processMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-h.incoming:
			if !ok {
				return
			}
			if err := h.handleMessage(ctx, msg); err != nil {
				log.Printf("❌ Error handling message: %v", err)
			}
		}
	}
}

// handleMessage handles a single message
func (h *Handler) handleMessage(ctx context.Context, msg *IncomingMessage) error {
	// Check if user is allowed
	if !h.isUserAllowed(msg.From.ID) {
		if h.bot.config.Debug {
			log.Printf("🚫 Message from user %d (not in allowed list)", msg.From.ID)
		}
		return nil
	}

	// Check if group is allowed
	if msg.Chat.IsGroup() && !h.isGroupAllowed(msg.Chat.ID) {
		if h.bot.config.Debug {
			log.Printf("🚫 Message from group %d (not in allowed list)", msg.Chat.ID)
		}
		return nil
	}

	// Convert to channel message
	channelMsg := h.toChannelMessage(msg)

	// Send typing indicator if it's a direct message
	if !msg.Chat.IsGroup() {
		h.sendTypingIndicator(msg.Chat.ID)
	}

	// Call registered message handler
	handler := h.bot.GetMessageHandler()
	if handler != nil {
		if err := handler(ctx, channelMsg); err != nil {
			log.Printf("❌ Error in message handler: %v", err)
			// Send error message to user
			if !msg.Chat.IsGroup() {
				_ = h.bot.SendMessage(ctx, msg.Chat.ID, "Sorry, I encountered an error processing your message.", nil)
			}
			return err
		}
	}

	return nil
}

// toChannelMessage converts an IncomingMessage to channels.Message
func (h *Handler) toChannelMessage(msg *IncomingMessage) *channels.Message {
	replyTo := ""
	if msg.ReplyTo != nil {
		replyTo = fmt.Sprintf("%d", msg.ReplyTo.MessageID)
	}

	metadata := make(map[string]interface{})
	metadata["telegram_message_id"] = msg.MessageID
	if msg.EditDate != nil {
		metadata["edited"] = true
	}
	if msg.MediaGroupID != "" {
		metadata["media_group_id"] = msg.MediaGroupID
	}

	return &channels.Message{
		ID:       fmt.Sprintf("%d", msg.MessageID),
		Channel:  "telegram",
		From:     fmt.Sprintf("%d", msg.From.ID),
		FromName: msg.From.String(),
		To:       fmt.Sprintf("%d", msg.Chat.ID),
		IsGroup:  msg.Chat.IsGroup(),
		GroupID:  "",
		Content:  msg.GetContent(),
		Type:     channels.MessageTypeText,
		Timestamp: msg.Date,
		ReplyTo:  replyTo,
		Metadata: metadata,
	}
}

// isUserAllowed checks if a user is allowed to interact with the bot
func (h *Handler) isUserAllowed(userID int64) bool {
	if len(h.bot.config.AllowedUsers) == 0 {
		return true // No restriction
	}

	for _, id := range h.bot.config.AllowedUsers {
		if id == userID {
			return true
		}
	}
	return false
}

// isGroupAllowed checks if a group is allowed to interact with the bot
func (h *Handler) isGroupAllowed(groupID int64) bool {
	if len(h.bot.config.AllowedGroups) == 0 {
		return true // No restriction
	}

	for _, id := range h.bot.config.AllowedGroups {
		if id == groupID {
			return true
		}
	}
	return false
}

// sendTypingIndicator sends a typing action to the chat
func (h *Handler) sendTypingIndicator(chatID int64) {
	action := telegrambotapi.NewChatAction(chatID, telegrambotapi.ChatTyping)
	_, _ = h.bot.api.Send(action)
}

// HandleCallbackQuery handles callback queries from inline keyboards
func (h *Handler) HandleCallbackQuery(update *telegrambotapi.Update) {
	if update.CallbackQuery == nil {
		return
	}

	callback := update.CallbackQuery
	if callback.Message == nil {
		// Answer callback query even if there's no message
		callbackConfig := telegrambotapi.NewCallback(callback.ID, "")
		_, _ = h.bot.api.Request(callbackConfig)
		return
	}

	// Parse callback data
	data := callback.Data
	if h.bot.config.Debug {
		log.Printf("🔘 Callback query from %s: %s", callback.From.UserName, data)
	}

	// Answer callback query
	callbackConfig := telegrambotapi.NewCallback(callback.ID, "")
	if _, err := h.bot.api.Request(callbackConfig); err != nil {
		log.Printf("❌ Error answering callback query: %v", err)
	}

	// Handle callback data
	h.handleCallback(callback, data)
}

// handleCallback processes a callback query
func (h *Handler) handleCallback(callback *telegrambotapi.CallbackQuery, data string) {
	// Extract command and parameters
	parts := strings.SplitN(data, ":", 2)
	if len(parts) == 0 {
		return
	}

	command := parts[0]

	switch command {
	case "help":
		// Show help message
		chatID := callback.Message.Chat.ID
		_ = h.bot.SendMessage(context.Background(), chatID, getHelpText(), nil)
	default:
		if h.bot.config.Debug {
			log.Printf("ℹ️  Unknown callback command: %s", command)
		}
	}
}

// getHelpText returns help message
func getHelpText() string {
	return "🤖 *Telegram Bot Help*\n\n" +
		"I'm an AI assistant connected to OpenClaw. Here's what I can do:\n\n" +
		"*Commands:*\n" +
		"• /help - Show this help message\n" +
		"• /start - Start a conversation\n" +
		"• /status - Check bot status\n\n" +
		"*Features:*\n" +
		"• ✅ Send text messages\n" +
		"• ✅ Reply to messages\n" +
		"• ✅ Work in private chats\n" +
		"• ✅ Work in groups\n\n" +
		"Just send me a message and I'll respond!"
}
