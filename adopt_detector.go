package main

// TODO: Make "id" field unqiue https://docs.mongodb.com/manual/core/index-unique/
// TODO: Implement telegram callbacks.
// TODO: Implement python webhook calls.

import (
	"fmt"
	"flag"
)

var keyFile string = "keys.yaml"


// streamablePost represents a post from a website that can be downloaded in a "streamed".
type streamablePost interface {
	createStream(chan<- streamablePost) // Spawn a goroutine to stream posts from the site and put them into the channel.
	formatLink() string // Format a link to the post.
	getID() string // Get a unique id for the post.
	getJSON() []byte // Convert the post to a JSON object. JSON should contain a field called "_id" which stores the same ID as above.
}

func databaseWriter(postDownloadQueue <-chan streamablePost, postNotifyQueue chan<- streamablePost) {
	for post := range postDownloadQueue {
		// TODO: Add post to the appropriate database

		// Send request to classifier
		postNotifyQueue <- post
	}
}

func postNotifier(postNotifyQueue <-chan streamablePost) {
	for _ = range postNotifyQueue {
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

	// Make channels for passing around posts.
	postDownloadQueue := make(chan streamablePost, 100)
	postNotifyQueue := make(chan streamablePost, 100)

	// Create a stream for each type of post to be downloaded.
	(*tweet)(nil).createStream(postDownloadQueue)

	go databaseWriter(postDownloadQueue, postNotifyQueue)
	go postNotifier(postNotifyQueue)

}