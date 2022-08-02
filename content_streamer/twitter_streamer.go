package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"go.mongodb.org/mongo-driver/mongo"
)

const twitterFeedCollection = "twitterFeeds"

// tweet implements the WebsitePost interface, represeting a tweet drawn from twitter.
type tweet struct {
	Id   string
	json []byte
}

type twitterFeed struct {
	Username     string `bson:"username"`
	LastPostTime int64  `bson:"last_post_time"`
}

func (tweet) createDownloadStream(downloadQueue chan<- postMessage, workers int) {
	// TODO: implement
}

func (t tweet) formatLink() string {
	return fmt.Sprintf("http://twitter.com/statuses/%s", t.Id)
}

func (f tweet) formatPost() string {
	panic("not implemented") // TODO: Implement
}

func (tweet) siteName() string {
	return "twitter"
}

func (tweet) prettySiteName() string {
	return "Twitter"
}

func (t tweet) getID() string {
	return string(t.Id)
}

func (f tweet) addFollowHandler() func(tgbotapi.Update) (bool, interface{}) {

	msg := tgbotapi.NewMessage(chatID, "What user would you like to add?")
	telegramBot.Send(msg)

	return handleAddUser
}

func handleAddUser(update tgbotapi.Update) (bool, interface{}) {

	// Check string contains whitespace, in which case break.
	if len(strings.Fields(update.Message.Text)) != 1 {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Invalid user - user must not contain whitespace.")
		telegramBot.Send(msg)
		return false, nil
	}

	username := strings.ToLower(update.Message.Text)

	if username[0:1] == "@" {
		username = username[1:]
	}

	// Create a new feed from the parameters and insert it.
	newFeed := twitterFeed{
		Username:     username,
		LastPostTime: 0,
	}

	_, err := database.Collection(twitterFeedCollection).InsertOne(context.TODO(), newFeed)
	if err != nil {
		log.Panicln(err)
	}

	// Send message to confirm.
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Added feed with username \"%s\"!", username))
	_, err = telegramBot.Send(msg)
	if err != nil {
		log.Panicln(err)
	}

	return false, nil
}

func (f tweet) downloadPost(_ string) (postMessage, error) {
	panic("not implemented") // TODO: Implement
}

func (f tweet) decodeDBResult(_ *mongo.SingleResult) (streamablePost, error) {
	panic("not implemented") // TODO: Implement
}
