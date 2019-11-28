package main 

import (
)

// deviation implements the WebsitePost interface, represeting a post drawn from deviantArt.
type deviation struct {
	id string
	json []byte
}

func (deviation) downloadStream(chan<- streamablePost) {
	// TODO: implement
}

func (d deviation) formatLink() string {
	// TODO: implement
	return ""
}

func (deviation) siteName() string {
	return "DeviantArt"
}

func (d deviation) ID() string {
	return d.id
}

func (d deviation) getJSON() []byte {
	// TODO: Implement
	return d.json
}