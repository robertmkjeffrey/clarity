package main

// TODO: Make "id" field unqiue https://docs.mongodb.com/manual/core/index-unique/
// TODO: Implement telegram callbacks.
// TODO: Implement python webhook calls.

import (
	"time"
	"go.mongodb.org/mongo-driver/mongo/options"
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	"io/ioutil"
	"os"
	"fmt"
	"log"
	"flag"
	"gopkg.in/yaml.v2"
)

// Configuration constants.
const mongoConnectTimeout = 5 * time.Second
const databaseName = "adopt-detector"
const keyFileName string = "keys.yaml"

// Global shared objects.
var keys map[interface{}]interface{}
var database *mongo.Database

// postTypes stores a list of nil pointers of each type implementing streamablePost; 
var postTypes = [...]streamablePost{tweet{}, deviation{}}

// streamablePost represents a post from a website that can be downloaded in a "streamed".
type streamablePost interface {
	createDownloadStream(downloadQueue chan<- streamablePost, workers int) // Stream posts from the site and put them into the channel.
	formatLink() string // Format a link to the post.
	siteName() string
	ID() string // Get a unique id for the post.
	getJSON() []byte // Convert the post to a JSON object. JSON should contain a field called "_id" which stores the same ID as above.
}

func databaseWriter(postDownloadQueue <-chan streamablePost, postNotifyQueue chan<- streamablePost) {
	for post := range postDownloadQueue {
		// Add post to the appropriate collection.
		log.Printf("Added %s\n", post.formatLink())
		database.Collection(post.siteName()).InsertOne(context.TODO(), post)
		// Send request to classifier
		postNotifyQueue <- post
	}
}

func postNotifier(postNotifyQueue <-chan streamablePost) {
	for range postNotifyQueue {
		// TODO: Send web request to the python script
		
		// TODO: if positive, send notification.

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
		log.Fatalln(err)
	}
	keyBytes, _ := ioutil.ReadAll(keyFile)
	yaml.Unmarshal(keyBytes, &keys)

	keyFile.Close()

	// Make channels for passing around posts.
	postDownloadQueue := make(chan streamablePost, 100)
	postNotifyQueue := make(chan streamablePost, 100)

	// Create a stream for each type of post to be downloaded.
	for _, postType := range postTypes {
		go postType.createDownloadStream(postDownloadQueue, 1)
	}

	// //TODO: remove this
	// for {
	// 	post := <-postDownloadQueue
	// 	log.Println(post.formatLink())
	// }

	// Connect to mongoDB database.
	mongoOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	ctx, cancel := context.WithTimeout(context.Background(), mongoConnectTimeout)
	client, err := mongo.Connect(ctx, mongoOptions)
	cancel()
	database = client.Database(databaseName)

	// Spawn the writers.
	go databaseWriter(postDownloadQueue, postNotifyQueue)
	go postNotifier(postNotifyQueue)

	select{}

}