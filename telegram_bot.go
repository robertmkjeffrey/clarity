package main

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"strings"
	"log"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

// Send a notification about a post to the telegram chat.
func sendPost(post streamablePost, score float64) {
	msgText := fmt.Sprintf("Score: %.2f\n%s", score, post.formatLink())
	msg := tgbotapi.NewMessage(chatID, msgText)
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

func deletePost(site string, id string) {
	collection := database.Collection(fmt.Sprintf("%sPosts", site))
	_, err := collection.DeleteMany(
		context.Background(),
		bson.M{"_id": id},
	)
	if err != nil {
		log.Panicln(err)
	}
}

func updatePostNotify(site string, id string, notification bool) {
	collection := database.Collection(fmt.Sprintf("%sPosts", site))
	_, err := collection.UpdateOne(
		context.Background(),
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"notify": notification}},
	)
	if err != nil {
		log.Panicln(err)
	}
}

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
	case update.CallbackQuery != nil:
		telegramBot.AnswerCallbackQuery(tgbotapi.NewCallback(update.CallbackQuery.ID,update.CallbackQuery.Data))
		// Switch over each button
		switch fields := strings.Fields(update.CallbackQuery.Data); fields[0] {
		case "cb_hide":
			// Hide message
			telegramBot.DeleteMessage(tgbotapi.NewDeleteMessage(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID))
			log.Printf("Hide post %s\n", fields[2])
		case "cb_delete":
			deletePost(fields[1], fields[2])
			// Hide message.
			telegramBot.DeleteMessage(tgbotapi.NewDeleteMessage(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID))
			log.Printf("Delete post %s\n", fields[2])
		case "cb_true":
			updatePostNotify(fields[1], fields[2], true)
			log.Printf("Set notification true on post %s\n", fields[2])
		case "cb_false":
			updatePostNotify(fields[1], fields[2], false)
			log.Printf("Set notification false on post %s\n", fields[2])
		}

	case update.Message.IsCommand():
		//TODO: handle commands
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Sorry, I didn't understand what you said. Try /help for commands.")
		telegramBot.Send(msg)
	}
	}
}