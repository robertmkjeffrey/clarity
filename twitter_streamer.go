package main 

import (
	"fmt"
)

// tweet implements the WebsitePost interface, represeting a tweet drawn from twitter.
type tweet struct {
	id string
	json []byte
}

func (tweet) createStream(chan streamablePost) {
	// TODO: implement
}

func (t tweet) formatLink() string {
	return fmt.Sprintf("http://twitter.com/statuses/%s", t.id)
}

func (t tweet) getID() string {
	return t.id
}

func (t tweet) getJSON() []byte {
	// TODO: Implement
	return t.json
}