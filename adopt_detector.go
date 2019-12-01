package main

// TODO: Implement telegram callbacks.
// TODO: Implement python webhook calls.

import (
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"time"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo"
	"context"
	"io/ioutil"
	"os"
	"fmt"
	"log"
	"flag"
	"gopkg.in/yaml.v2"
)

// Configuration constants.
const mongoConnectTimeout = 5 * time.Second
const databaseName = "adopt-detector-DB"
const keyFileName = "keys.yaml"

// Global shared objects.
var keys map[interface{}]interface{}
var telegramBot *tgbotapi.BotAPI
var chatID int64
var database *mongo.Database

// postTypes stores a list of nil pointers of each type implementing streamablePost to generalise certain operations.
var postTypes = [...]streamablePost{tweet{}, deviation{}}

// streamablePost represents a post from a website that can be downloaded in a "streamed".
type streamablePost interface {
	createDownloadStream(downloadQueue chan<- streamablePost, workers int) // Stream posts from the site and put them into the channel.
	formatLink() string // Format a link to the post.
	siteName() string
}

func databaseWriter(postDownloadQueue <-chan streamablePost, postNotifyQueue chan<- streamablePost) {
	for post := range postDownloadQueue {
		// Add post to the appropriate collection.
		log.Printf("Added %s\n", post.formatLink())
		collection := database.Collection(fmt.Sprintf("%s-posts", post.siteName()))
		collection.InsertOne(context.TODO(), post)
		// Send request to classifier
		postNotifyQueue <- post
	}
}

func postNotifier(postNotifyQueue <-chan streamablePost) {
	for post := range postNotifyQueue {
		// TODO: Send web request to the python script
		
		// TODO: Check if positive before sending notification.
		telegramBot.Send(tgbotapi.NewMessage(chatID, post.formatLink()))
	}
}

func main() {

	var init = flag.Bool("init", false, "Initalise all necessary databases and files.")

	if *init {
		//TODO: run setup code
		fmt.Println("Setting up system...")
		return
	}

	// Load keys into memory
	keyFile, err := os.Open(keyFileName)
	if err != nil {
		log.Panicln(err)
	}
	keyBytes, _ := ioutil.ReadAll(keyFile)
	yaml.Unmarshal(keyBytes, &keys)
	keyFile.Close()

	// Create telegram bot object by getting key from the key object.
	telegramKeys := keys["telegram"].(map[interface{}]interface{})
	telegramBot, err = tgbotapi.NewBotAPI(telegramKeys["api_key"].(string))
	if err != nil {
		log.Panicln(err)
	}
	telegramBot.Debug = true
	chatID = int64(telegramKeys["chat_id"].(int))

	// Connect to mongoDB database.
	mongoOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	ctx, cancel := context.WithTimeout(context.Background(), mongoConnectTimeout)
	client, err := mongo.Connect(ctx, mongoOptions)
	cancel()
	database = client.Database(databaseName)

	// Make channels for passing around posts.
	postDownloadQueue := make(chan streamablePost, 100)
	postNotifyQueue := make(chan streamablePost, 100)

	// Create a stream for each type of post to be downloaded.
	for _, postType := range postTypes {
		go postType.createDownloadStream(postDownloadQueue, 1)
	}

	// Spawn the writers.
	go databaseWriter(postDownloadQueue, postNotifyQueue)
	go postNotifier(postNotifyQueue)

	select{}

}