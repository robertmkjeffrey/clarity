package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
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
	formatReplyMarkup(post, score, &msg)
	telegramBot.Send(msg)
}

func formatReplyMarkup(post streamablePost, score float64, msg *tgbotapi.MessageConfig) {
	// Define inline keyboard
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("🔗", post.formatLink()),
			tgbotapi.NewInlineKeyboardButtonData("💬", fmt.Sprintf("cb_print %s %s", post.siteName(), post.getID())),
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
}

func siteSelectKeyboard() tgbotapi.ReplyKeyboardMarkup {
	keyboard := tgbotapi.ReplyKeyboardMarkup{}
	keyboard.OneTimeKeyboard = true
	keyboard.ResizeKeyboard = true
	keyboard.Selective = false
	for _, site := range siteTypes {
		keyboard.Keyboard = append(keyboard.Keyboard, tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(site.prettySiteName())))
	}
	return keyboard
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
func telegramCallbackHandler(downloadQueue chan<- postMessage) {
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

	log.Println("Started telegram callback handler.")

	for update := range updates {
		switch {
		case update.CallbackQuery != nil:
			// If update is a callback, handle the keyboard button
			// Answer callback.
			telegramBot.AnswerCallbackQuery(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))

			fields := strings.Fields(update.CallbackQuery.Data)
			button := fields[0]
			site := fields[1]
			id := fields[2]
			// Switch over each button
			switch button {
			case "cb_hide":
				// Hide message
				telegramBot.DeleteMessage(tgbotapi.NewDeleteMessage(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID))
				if debug {
					log.Printf("Hide post %s\n", id)
				}
			case "cb_delete":
				// Delete post from the database.
				deletePost(site, id)
				// Hide message.
				telegramBot.DeleteMessage(tgbotapi.NewDeleteMessage(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID))
				if debug {
					log.Printf("Delete post %s\n", id)
				}
			case "cb_true":
				// Update post notify status
				updatePostNotify(site, id, true)
				log.Printf("Set notification true on post %s\n", id)
			case "cb_false":
				// Update post notify status.
				updatePostNotify(site, id, false)
				if debug {
					log.Printf("Set notification false on post %s\n", id)
				}
			case "cb_print":
				post, err := getPost(site, id)

				if err != nil {
					log.Fatalf("Got callback on post %s but could not find it in database.", id)
				}

				// Update post with a new score.
				result := classifyPost(post)
				score := result.Score

				// Delete the old message.
				telegramBot.DeleteMessage(tgbotapi.NewDeleteMessage(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID))

				msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, fmt.Sprintf("Score: %.2f\n%s", score, post.formatPost()))
				formatReplyMarkup(post, score, &msg)

				_, err = telegramBot.Send(msg)
				if err != nil {
					if err.Error() == "Bad Request: message is too long" {
						// TODO: Better handling of too-long posts.
						log.Printf("Error: post %s too long\n", post.getID())
						sendPost(post, score)
					} else {
						log.Panicln(err)
					}
				}
			}

		case update.Message != nil && waitingForResponse:
			var newResponseHandler interface{}
			// If waiting for a response and got a message, run the response handler.
			waitingForResponse, newResponseHandler = responseHandler(update)
			// If still waiting for a response, cast the handler.
			if waitingForResponse {
				responseHandler = newResponseHandler.(func(tgbotapi.Update) (bool, interface{}))
			} else {
				responseHandler = nil
			}

		case update.Message != nil && update.Message.IsCommand():
			// If the message is a command, perform the action.
			var msg tgbotapi.MessageConfig
			switch update.Message.Command() {
			case "start":
				msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Welcome! Try /help to get a list of commands.")
			case "help":
				msg = tgbotapi.NewMessage(update.Message.Chat.ID, `
Wagyl (c) 2020 - @DingoDingus
Currently implemented sites:
	* [DeviantArt](https://www.deviantart.com/)
Commands:
	* /help - Print this message.
	* /follow - Begin a dialogue to add a new data stream to Wagyl's followed users. 
	* /add site post_id - Add a post to the database and request it to be labelled.
	* /label site count - Get count posts from site to be labelled. Posts are chosen to maximise the training of the site's notification model.
	* /retrain [site] - Retrain a site's notification model. TODO - If no site is specified, all sites will be retrained.
	* /stats [site] - Print statistics about a certain site. If no site is specified, all site statistics will be printed. TODO - Currently unimplemented.
`)
			case "follow":
				// Open a dialogue to add a new query to the follow list.
				msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Which site do you want to add a follow for?")
				msg.ReplyMarkup = siteSelectKeyboard()
				// Save that we're waiting for a response.
				waitingForResponse = true
				responseHandler = followHandler
			case "retrain":
				// Trigger a certain model to be retrained based on the latest data.
				arguments := strings.Fields(update.Message.CommandArguments())

				// Site to be retrained.
				var siteName string

				// If we have one arg, use it as the site name.
				// If we have none, retrain all sites.
				// Otherwise, send an error.
				if len(arguments) > 1 {
					msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Sorry, I don't know how to parse that many parameters. Check /help for usage.")
					break
				} else if len(arguments) == 0 {
					siteName = "all"
				} else { // Exactly one argument.
					siteArg := arguments[0]
					site, err := parseSiteName(siteArg)
					// Make sure site is valid.
					if err != nil {
						msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Sorry, I don't recognise that site. Check /help for the implemented sites.")
						break
					}

					siteName = site.siteName()
				}

				// Request classifier for retraining.
				// Send web request to the python script
				params := url.Values{}
				params.Add("site", siteName)
				requestParams := params.Encode()
				resp, err := http.Get(fmt.Sprintf("http://localhost:5000/retrain?%s", requestParams))
				if err != nil {
					log.Panicln(err)
				}

				// Check if the retrain was successful and send an appropriate message.
				var result struct {
					Success bool
					Error   string
				}
				json.NewDecoder(resp.Body).Decode(&result)

				if result.Success {
					msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Successfully retrained model.")
				} else {
					msg = tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Failed to retrain model \"%s\".\nError: %s\n", siteName, result.Error))
				}

			case "stats":
				// TODO: trigger a certain model to be retrained based on the latest data.
				arguments := strings.Fields(update.Message.CommandArguments())

				// Site to be retrained.
				var siteName string

				// If we have one arg, use it as the site name.
				// If we have none, retrain all sites.
				// Otherwise, send an error.
				if len(arguments) > 1 {
					msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Sorry, I don't know how to parse that many parameters. Check /help for usage.")
					break
				} else if len(arguments) == 0 {
					siteName = "all"
				} else { // Exactly one argument.
					siteArg := arguments[0]
					site, err := parseSiteName(siteArg)
					// Make sure site is valid.
					if err != nil {
						msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Sorry, I don't recognise that site. Check /help for the implemented sites.")
						break
					}

					siteName = site.siteName()
				}

				// Request classifier for retraining.
				// Send web request to the python script
				params := url.Values{}
				params.Add("site", siteName)
				requestParams := params.Encode()
				resp, err := http.Get(fmt.Sprintf("http://localhost:5000/stats?%s", requestParams))
				if err != nil {
					log.Panicln(err)
				}

				// Check if the retrain was successful and send an appropriate message.
				var result struct {
					Success    bool
					Statistics string
					Error      string
				}
				json.NewDecoder(resp.Body).Decode(&result)

				if result.Success {
					msg = tgbotapi.NewMessage(update.Message.Chat.ID, result.Statistics)
				} else {
					msg = tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Failed to get statistics for model \"%s\".\nError: %s\n", siteName, result.Error))
				}

			case "label":
				// Send a series of posts to be labelled based on active-learning maths.

				arguments := strings.Fields(update.Message.CommandArguments())

				// If there's not the right number of args, send an error message.
				if len(arguments) != 2 {
					msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Sorry, I don't know how to parse that many parameters. Check /help for usage.")
					break
				}

				siteArg := arguments[0]
				count := arguments[1]

				site, err := parseSiteName(siteArg)
				// Make sure site is valid.
				if err != nil {
					msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Sorry, I don't recognise that site. Check /help for the implemented sites.")
					break
				}

				// Request classifier for labelling.
				// Send web request to the python script
				params := url.Values{}
				params.Add("site", site.siteName())
				params.Add("count", count)
				requestParams := params.Encode()
				resp, err := http.Get(fmt.Sprintf("http://localhost:5000/label?%s", requestParams))
				if err != nil {
					log.Panicln(err)
				}
				// Decode the results
				var result struct {
					IDs  []string
					Site string
				}

				json.NewDecoder(resp.Body).Decode(&result)

				for _, postID := range result.IDs {
					post, err := getPost(siteArg, postID)
					if err != nil {
						msg = tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Couldn't find post %s", postID))
						_, err := telegramBot.Send(msg)
						if err != nil {
							log.Panicln(err)
						}
						continue
					}
					message := postMessage{
						post:      post,
						setNotify: BoolPointer(true),
						skipWrite: true,
					}
					downloadQueue <- message
				}
				// Don't send a message.
				msg = tgbotapi.NewMessage(update.Message.Chat.ID, "")

			case "add":

				// Add a post to be labelled.

				arguments := strings.Fields(update.Message.CommandArguments())

				// If there's not the right number of args, send an error message.
				if len(arguments) != 2 {
					msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Sorry, I don't know how to parse that many parameters. Check /help for usage.")
					break
				}

				siteArg := arguments[0]
				postIDArg := arguments[1]

				site, err := parseSiteName(siteArg)

				if err != nil {
					msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Sorry, I don't recognise that site. Check /help for the implemented sites.")
					break
				}

				post, err := site.downloadPost(postIDArg)
				if err != nil {
					msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Sorry, I couldn't find that post. Please try again.")
					break
				}
				post.setNotify = BoolPointer(true)
				downloadQueue <- post

			default:
				// If command isn't recognised, reply with error.
				msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Sorry, I don't recognise that command. Try /help for commands.")
			}

			if msg.Text != "" {
				_, err := telegramBot.Send(msg)
				if err != nil {
					log.Panicln(err)
				}
			}

		default:
			// If message isn't recognised, reply with error.
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Sorry, I didn't understand what you said. Try /help for commands.")
			_, err := telegramBot.Send(msg)
			if err != nil {
				log.Panicln(err)
			}
		}
	}
}
