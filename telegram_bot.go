package main

import (
	"strings"
	"log"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

// Send a notification about a post to the telegram chat.
func sendPost(post streamablePost, score float64) {
	// Make message with score and link
	msgText := fmt.Sprintf("Score: %.2f\n%s", score, post.formatLink())
	msg := tgbotapi.NewMessage(chatID, msgText)
	// Define inline keyboard
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("🔗", post.formatLink()),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✔", fmt.Sprintf("cb_true %s %s", post.siteName(), post.getID())),
			tgbotapi.NewInlineKeyboardButtonData("❌", fmt.Sprintf("cb_false %s %s", post.siteName(), post.getID())),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚫", fmt.Sprintf("cb_hide %s %s", post.siteName(), post.getID())),
			tgbotapi.NewInlineKeyboardButtonData("🗑", fmt.Sprintf("cb_delete %s %s", post.siteName(), post.getID())),
		),
	)
	telegramBot.Send(msg)
}

// telegramCallbackHandler defines a goroutine that responds to messages and callbacks from the telegram chat.
func telegramCallbackHandler() {
	// Create updates channel.
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates, err := telegramBot.GetUpdatesChan(u)
	if err != nil {
		log.Panicln(err)
	}

	for update := range updates {
	switch {
	// If update is a callback, handle the keyboard button
	case update.CallbackQuery != nil:
		// Answer callback.
		telegramBot.AnswerCallbackQuery(tgbotapi.NewCallback(update.CallbackQuery.ID,update.CallbackQuery.Data))
		// Switch over each button
		switch fields := strings.Fields(update.CallbackQuery.Data); fields[0] {
		case "cb_hide":
			// Hide message
			telegramBot.DeleteMessage(tgbotapi.NewDeleteMessage(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID))
			log.Printf("Hide post %s\n", fields[2])
		case "cb_delete":
			// Delete post from the database.
			deletePost(fields[1], fields[2])
			// Hide message.
			telegramBot.DeleteMessage(tgbotapi.NewDeleteMessage(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID))
			log.Printf("Delete post %s\n", fields[2])
		case "cb_true":
			// Update post notify status
			updatePostNotify(fields[1], fields[2], true)
			log.Printf("Set notification true on post %s\n", fields[2])
		case "cb_false":
			// Update post notify status.
			updatePostNotify(fields[1], fields[2], false)
			log.Printf("Set notification false on post %s\n", fields[2])
		}

	case update.Message.IsCommand():
		//TODO: handle commands
		var msg tgbotapi.MessageConfig
		switch update.Message.Command(){
		default:
			// If command isn't recognised, reply with error.
			msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Sorry, I didn't understand what you said. Try /help for commands.")		
			msg.ReplyToMessageID = update.Message.MessageID
		}
		telegramBot.Send(msg)
	}
	}
}