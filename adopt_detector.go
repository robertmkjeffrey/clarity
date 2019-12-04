package main

// TODO: Implement telegram callbacks.
// TODO: Implement python webhook calls.

import (
	"os/signal"
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
const debug = true

// Global shared objects.
var shutdownChan chan os.Signal
var keys map[interface{}]interface{}
var telegramBot *tgbotapi.BotAPI
var chatID int64
var database *mongo.Database

// siteTypes stores a list of nil pointers of each type implementing streamablePost to generalise certain operations.
var siteTypes = [...]streamablePost{deviation{}}

// streamablePost represents a post from a website that can be downloaded in a "streamed".
type streamablePost interface {
	createDownloadStream(downloadQueue chan<- streamablePost, workers int) // Stream posts from the site and put them into the channel.
	formatLink() string // Format a link to the post.
	siteName() string // Return a computer-ready version of the site name (lowercase, no hypens etc.)
	prettySiteName() string // Return a pretty version of the site name (e.g. with capitalisation)
	getID() string // Return the field used as "_id" in the mongodb database.
	addFollowHandler() func(tgbotapi.Update) (bool, interface{}) // Start the process of adding a follow through the telegram bot.
}

// databaseWriter defines a goroutine that reads from the download queue, adds each post to the database, then passes it to the notify queue.
func databaseWriter(postDownloadQueue <-chan streamablePost, postNotifyQueue chan<- streamablePost) {
	for post := range postDownloadQueue {
		// Add post to the appropriate collection.
		log.Printf("Added %s\n", post.formatLink())
		collection := database.Collection(fmt.Sprintf("%sPosts", post.siteName()))
		collection.InsertOne(context.TODO(), post)
		// Send request to classifier
		postNotifyQueue <- post
	}
}

// postNotifier defines a goroutine that reads from the notify queue, classifies it using the python webhook, and then notifies the user if positive.
func postNotifier(postNotifyQueue <-chan streamablePost) {
	for post := range postNotifyQueue {
		// TODO: Send web request to the python script
		
		// TODO: Check if positive before sending notification.
		sendPost(post, 0.0)
	}
}

func main() {

	// Define command-line options.
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
	telegramBot.Debug = false
	chatID = int64(telegramKeys["chat_id"].(int))

	// Connect to mongoDB database.
	mongoOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	ctx, cancel := context.WithTimeout(context.Background(), mongoConnectTimeout)
	client, err := mongo.Connect(ctx, mongoOptions)
	cancel()
	database = client.Database(databaseName)

	// Spawn callback handler
	go telegramCallbackHandler()
	
	// Make channels for passing around posts.
	postDownloadQueue := make(chan streamablePost, 100)
	postNotifyQueue := make(chan streamablePost, 100)

	// Create a stream for each type of post to be downloaded.
	for _, postType := range siteTypes {
		go postType.createDownloadStream(postDownloadQueue, 1)
	}

	// Spawn the writers.
	go databaseWriter(postDownloadQueue, postNotifyQueue)
	go postNotifier(postNotifyQueue)

	// Halt forever, TODO: wait for control-C and start safe shutdown. 
	shutdownChan = make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt)
	<- shutdownChan

}