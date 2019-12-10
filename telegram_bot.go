package main

import (
	"errors"
	"strings"
	"log"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

func sendShutdownMessage(r interface{}) {
	msgText := fmt.Sprint("Panic! Shuting down with following panic: /n", fmt.Sprint(r))
	msg := tgbotapi.NewMessage(chatID, msgText)

	telegramBot.Send(msg)
}

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

func siteSelectKeyboard () tgbotapi.ReplyKeyboardMarkup {
	keyboard := tgbotapi.ReplyKeyboardMarkup{}
	keyboard.OneTimeKeyboard = true
	keyboard.ResizeKeyboard = true
	keyboard.Selective = false
	for _, site := range siteTypes {
		keyboard.Keyboard = append(keyboard.Keyboard, tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(site.prettySiteName())))
	}
	return keyboard
} 

func parseSiteName(text string) (streamablePost, error) {
	for _, site := range siteTypes {
		if text == site.siteName() || text == site.prettySiteName() {
			return site, nil
		}
	}
	return nil, errors.New("invalid site name")
} 

func followHandler(update tgbotapi.Update) (waitForResponse bool, responseHandler interface{}) {
	
	site, returnErr := parseSiteName(update.Message.Text) 
	if returnErr != nil {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Invalid site name, please restart. ")		
		_, err := telegramBot.Send(msg)
		if err != nil {
			log.Panicln(err)
		}
		return false, nil
	}

	return true, site.addFollowHandler()
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

	// Keep track of a if there's an expected response.
	var waitingForResponse bool = false
	// Function to handle next step in a thread of commands.
	var responseHandler func(tgbotapi.Update) (bool, interface{})

	for update := range updates {
	switch {
	case update.CallbackQuery != nil:	
		// If update is a callback, handle the keyboard button
		// Answer callback.
		telegramBot.AnswerCallbackQuery(tgbotapi.NewCallback(update.CallbackQuery.ID,""))
		
		fields := strings.Fields(update.CallbackQuery.Data);
		button := fields[0]
		site := fields[1]
		id := fields[2]
		// Switch over each button
		switch button {
		case "cb_hide":
			// Hide message
			telegramBot.DeleteMessage(tgbotapi.NewDeleteMessage(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID))
			log.Printf("Hide post %s\n", id)
		case "cb_delete":
			// Delete post from the database.
			deletePost(site, id)
			// Hide message.
			telegramBot.DeleteMessage(tgbotapi.NewDeleteMessage(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID))
			log.Printf("Delete post %s\n", id)
		case "cb_true":
			// Update post notify status
			updatePostNotify(site, id, true)
			log.Printf("Set notification true on post %s\n", id)
		case "cb_false":
			// Update post notify status.
			updatePostNotify(site, id, false)
			log.Printf("Set notification false on post %s\n", id)
		}

	case update.Message != nil && waitingForResponse:
		// If waiting for a response and got a message, run the response handler. 
		waitingForResponse, newResponseHandler := responseHandler(update)
		// If still waiting for a response, cast the handler.
		if waitingForResponse {
			responseHandler = newResponseHandler.(func(tgbotapi.Update) (bool, interface{}))
		}

	case update.Message != nil && update.Message.IsCommand():
		// handle commands
		var msg tgbotapi.MessageConfig
		switch update.Message.Command(){
		case "start":
			msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Welcome! Try /help to get a list of commands.")
		case "follow":
			// Open a dialogue to add a new query to the follow list.
			msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Which site do you want to add a follow for?")
			msg.ReplyMarkup = siteSelectKeyboard()
			// Save that we're waiting for a response.
			waitingForResponse = true
			responseHandler = followHandler
		default:
			// If command isn't recognised, reply with error.
			msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Sorry, I didn't understand what you said. Try /help for commands.")		
		}
		_, err := telegramBot.Send(msg)
		if err != nil {
			log.Panicln(err)
		}

	default:
		// If command isn't recognised, reply with error.
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Sorry, I didn't understand what you said. Try /help for commands.")		
		_, err := telegramBot.Send(msg)
		if err != nil {
			log.Panicln(err)
		}
	}
	}
}