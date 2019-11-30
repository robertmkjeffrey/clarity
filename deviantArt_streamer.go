package main 

import (
	"fmt"
	"strconv"
	"encoding/json"
	"log"
	"bytes"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Time to wait between polls of deviantArt
const pollingRate = 5 * time.Minute

const urlEncoded = "application/x-www-form-urlencoded"
var dAFollows chan dAFollow // Circular channel of followed users

// deviation implements the streamablePost interface, represeting a post drawn from deviantArt.
type deviation struct {
	id string
	url string
	json []byte
}

type dAFollow interface {
	// getResults queries deviantArt for the most recent posts from the follow.
	getResults(offset int) map[string]interface{}
	getLastQueryTime() int64
}

type dATagFollow struct {
	tag string
	lastQueryTime int64
}

func (f dATagFollow) getLastQueryTime() int64 {
	return f.lastQueryTime
}

func (f dATagFollow) getResults(offset int) map[string]interface{} {
	params := url.Values{}
	params.Add("tag", f.tag)
	// params.Add("offset", string(offset))
	
	dAAccessToken.RLock()
	params.Add("access_token", dAAccessToken.token)
	dAAccessToken.RUnlock()

	requestSting := params.Encode()

	// Send request
	resp, err := http.Get(fmt.Sprintf("https://www.deviantart.com/api/v1/oauth2/browse/tags?%s", requestSting))
		
	if err != nil {
		log.Fatalln(err)
	}

	// Decode the results
	var result map[string]interface{}
	
	json.NewDecoder(resp.Body).Decode(&result)
	
	return result
}

type dAUserFollow struct {
	user string
	lastQueryTime int64
}

// Global variable for access token storage.
var dAAccessToken struct {
	sync.RWMutex
	token string
}

func getDAAccessToken() {

	dAKeys := keys["deviantArt"].(map[interface{}]interface{})
	
	// Build url encoding of request.
	params := url.Values{}
	params.Add("grant_type", "client_credentials")
	params.Add("client_id", dAKeys["client_id"].(string))
	params.Add("client_secret", dAKeys["client_secret"].(string))

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
	dAAccessToken.Lock()
	dAAccessToken.token = token.(string)
	dAAccessToken.Unlock()
	
}

func dADownloadWorker(downloadQueue chan<- streamablePost) {
	//TODO: implement
	for {
		// Get the next search and wait until the next polling opportunity
		currentSearch := <-dAFollows
		lastQueryTime := currentSearch.getLastQueryTime()
		// Find time since last query
		ellapsedTime := time.Since(time.Unix(lastQueryTime, 0))
		// Wait for polling time from last query
		time.After(pollingRate - ellapsedTime)
		
		nextPage := 0
		dAResultParseLoop:
		for {
			query := currentSearch.getResults(nextPage)
			// log.Println(query)
			results := query["results"].([]interface{})
			for _, result := range results {
				result := result.(map[string]interface{})
				// Parse the published time return value
				publishedTime, err := strconv.ParseInt(result["published_time"].(string), 10,64)
				if err != nil {
					log.Fatalln(err)
				}
				// If the result is older than the last parse time, end the query.
				if publishedTime <= lastQueryTime {
					break dAResultParseLoop
				}
				// TODO: Make query to get deviation object etc.
				downloadQueue <- deviation{}
			}
			nextPage++
		}


	}
}

func (deviation) createDownloadStream(downloadQueue chan<- streamablePost, workers int) {

	// Request an access token.
	getDAAccessToken()

	// Every 59 minutes, get a new access token. Token expires every 60 minutes.
	go func(){
		for {
		time.Sleep(59 * time.Minute)
		getDAAccessToken()
		}
	}()

	// TODO: Read follow files from database.
	dAFollows = make(chan dAFollow, 1)
	dAFollows <- dATagFollow{tag:"vernid", lastQueryTime:1574935102}

	// Spawn a worker for each in the range of workers.
	for i := 0; i < workers; i++ {
		go dADownloadWorker(downloadQueue)
	}

	return

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