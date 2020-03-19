package main

// TODO: Implement telegram callbacks.
// TODO: Implement python webhook calls.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/yaml.v2"
)

// Configuration constants.
const mongoConnectTimeout = 5 * time.Second
const databaseName = "adopt-detector-DB"
const keyFileName = "keys.yaml"

// Global shared objects.
var shutdownWG sync.WaitGroup
var cleanupWG sync.WaitGroup
var keys map[interface{}]interface{}
var telegramBot *tgbotapi.BotAPI
var chatID int64
var database *mongo.Database
var debug = false

// siteTypes stores a list of nil pointers of each type implementing streamablePost to generalise certain operations.
var siteTypes = [...]streamablePost{deviation{}}

// streamablePost represents a post from a website that can be downloaded in a "streamed".
type streamablePost interface {
	createDownloadStream(downloadQueue chan<- streamablePost, workers int) // Stream posts from the site and put them into the channel.
	formatLink() string                                                    // Format a link to the post.
	siteName() string                                                      // Return a computer-ready version of the site name (lowercase, no hypens etc.)
	prettySiteName() string                                                // Return a pretty version of the site name (e.g. with capitalisation)
	getID() string                                                         // Return the field used as "_id" in the mongodb database.
	addFollowHandler() func(tgbotapi.Update) (bool, interface{})           // Start the process of adding a follow through the telegram bot.
}

// webhookHandler starts the ml_webhook python server to handle classification requests.
func webhookHandler() {
	// Add a wait to the cleanup counter
	cleanupWG.Add(1)
	defer cleanupWG.Done()

	cmd := exec.Command("python", "ml_webhook.py")
	log.Println("Starting python script.")

	// Define a writer to write python output to stout.
	mwriter := io.MultiWriter(os.Stdout)
	cmd.Stdout = mwriter
	cmd.Stderr = mwriter

	cmd.Start()

	// Wait for shutdown
	shutdownWG.Wait()
	err := cmd.Process.Kill()
	if err != nil {
		log.Panicf("Cannot shut down webhook handler! Error code:\n%s\n", err)
	}
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
		// Send web request to the python script
		params := url.Values{}
		params.Add("id", post.getID())
		params.Add("site", post.siteName())
		requestParams := params.Encode()
		resp, err := http.Get(fmt.Sprintf("http://localhost:5000/classify?%s", requestParams))
		if err != nil {
			log.Panicln(err)
		}
		// Decode the results
		var result struct {
			ID     string
			Site   string
			Notify bool
			Score  float64
		}

		json.NewDecoder(resp.Body).Decode(&result)
		fmt.Println(result)

		// Check if positive before sending notification.
		if result.Notify {
			sendPost(post, result.Score)
		}
	}
}

func main() {

	// Define command-line options.
	var init = flag.Bool("init", false, "Initalise all necessary databases and files.")
	var debugFlag = flag.Bool("debug", false, "Log more information to the terminal.")

	// Parse flags
	flag.Parse()

	debug = *debugFlag

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

	// Initialise a shutdown waitgroup for all processes needing shutdown to wait on.
	shutdownWG.Add(1)

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

	// Start webhook handler
	go webhookHandler()

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

	// Defer a shutdown message to send the panic to the user and then repanic.
	defer func() {
		if r := recover(); r != nil {
			sendShutdownMessage(r)
			panic(r)
		}
	}()

	// Wait for Control-C and execute a safe shutdown.
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt)
	<-shutdownChan
	log.Println("Shutting down...")

	// Signal time to shut down.
	shutdownWG.Done()
	// Wait for cleanup to finish
	cleanupWG.Wait()

}
