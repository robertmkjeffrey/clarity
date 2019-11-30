package main 

import (
	"encoding/json"
	"log"
	"bytes"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const urlEncoded = "application/x-www-form-urlencoded"

// deviation implements the streamablePost interface, represeting a post drawn from deviantArt.
type deviation struct {
	id string
	json []byte
}

// Global variable for access token storage.
var deviantArtAccessToken struct {
	sync.RWMutex
	token string
}

func getDeviantArtAccessToken() {

	deviantArtKeys := keys["deviantArt"].(map[interface{}]interface{})
	
	// Build url encoding of request.
	params := url.Values{}
	params.Add("grant_type", "client_credentials")
	params.Add("client_id", deviantArtKeys["client_id"].(string))
	params.Add("client_secret", deviantArtKeys["client_secret"].(string))

	requestSting := params.Encode()

	// Send request
	resp, err := http.Post(	"https://www.deviantart.com/oauth2/token",
						  	urlEncoded,
						  	bytes.NewBufferString(requestSting))
		
	if err != nil {
		log.Fatalln(err)
	}

	// Decode the results
	var result map[string]interface{}
	
	json.NewDecoder(resp.Body).Decode(&result)
	
	// If the response doesn't contain a valid token, throw an error.
	token, ok := result["access_token"]
	if !ok {
		log.Fatalf("Error: DeviantArt token refresh failed with error %s\n", result["error"])
	}

	// Set the token globally.
	deviantArtAccessToken.Lock()
	deviantArtAccessToken.token = token.(string)
	deviantArtAccessToken.Unlock()
	
}

func (deviation) downloadStream(chan<- streamablePost) {
	// TODO: make this work if spawned multiple times
	// TODO: implement

	// Request an access token.
	getDeviantArtAccessToken()

	// Every 59 minutes, get a new access token. Token expires every 60 minutes.
	go func(){
		for {
		time.Sleep(59 * time.Minute)
		getDeviantArtAccessToken()
		}
	}()

	// Actually download data
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