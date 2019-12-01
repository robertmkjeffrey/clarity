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
const pollingDelay = 5 * time.Minute

const urlEncoded = "application/x-www-form-urlencoded"
var dAFollows chan dAFeed // Circular channel of followed users

// deviation implements the streamablePost interface, represeting a post drawn from deviantArt.
type deviation struct {
	Deviationid string `json:"deviationid" bson:"_id"`
	URL string `json:"url"`
	Author dAUser `json:"author"`
	Title string `json:"title"`
	Description string `json:"description"`
	License string `json:"license"`
	AllowsComments bool `json:"allows_comments"`
	Tags []dATag `json:"tags"`
	IsMature bool `json:"is_mature"`
}

type dATag struct {
	TagName string `json:"tag_name"`
	Sponsored bool `json:"sponsored"`
	Sponsor string `json:"sponsor"`
}

type dAUser struct {
	Userid string `json:"userid"`
	Username string `json:"username"`
	UserType string `json:"type"`	
}

type dAFeed struct {
	dAFeedQuery
	lastQueryTime time.Time
	lastPostTime int64
}

type dAFeedQuery interface {
	// getDAResults queries deviantArt for the most recent posts from the follow.
	getDAResults(offset int) map[string]interface{}
}

type dATagQuery struct {
	tag string
}

func (f dATagQuery) getDAResults(offset int) map[string]interface{} {
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
		log.Panicln(err)
	}

	// Decode the results
	var result map[string]interface{}
	
	json.NewDecoder(resp.Body).Decode(&result)
	
	return result
}

type dAUserQuery struct {
	user string
}

func (f *dAUserQuery) getDAResults(offset int) map[string]interface{} {
	//TODO: implement
	return nil
}

// Global variable for access token storage.
var dAAccessToken struct {
	sync.RWMutex
	token string
}

func getDeviation(id string) deviation {
	return getDeviations([]string{id})[0]
}

func getDeviations(ids []string) []deviation {
	// If there are too many ids to do in one go, run two queries and append the results.
	if len(ids) > 50 {
		return append(getDeviations(ids[:50]), getDeviations(ids[50:])...)
	}

	params := url.Values{}
	for _, id := range ids {
		params.Add("deviationids[]", id)
	}
	dAAccessToken.RLock()
	params.Add("access_token", dAAccessToken.token)
	dAAccessToken.RUnlock()

	resp, err := http.Get(fmt.Sprintf("https://www.deviantart.com/api/v1/oauth2/deviation/metadata?%s", params.Encode()))
	if err != nil {
		log.Panicln(err)
	}


	// Decode the results. Anonomous struct to remove the top level metadata field.
	var results struct {
		Metadata []deviation `json:"metadata"`
	}

	json.NewDecoder(resp.Body).Decode(&results)
	
	return results.Metadata
	
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
		log.Panicln(err)
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

	for {
		// Get the next search and wait until the next polling opportunity
		feed := <-dAFollows
		// Wait for polling time from last query
		<-time.After(pollingDelay - time.Since(feed.lastQueryTime))
		
		// Store the new ids to analyse in one go.
		newIDs := make([]string, 0)
		postURLs := make(map[string]string)

		newLastPostTime := feed.lastPostTime
		nextPage := 0
		dAResultParseLoop:
		for {
			query := feed.getDAResults(nextPage)
			results := query["results"].([]interface{})

			for _, result := range results {
				result := result.(map[string]interface{})
				// Parse the published time return value
				publishedTime, err := strconv.ParseInt(result["published_time"].(string), 10,64)
				if err != nil {
					log.Panicln(err)
				}
				// If the result is older than the last parse time, end the query.
				if publishedTime <= feed.lastPostTime {
					break dAResultParseLoop
				}

				// Set newQueryTime to the newest post time.
				if publishedTime > newLastPostTime {
					newLastPostTime = publishedTime
				}

				deviationid := result["deviationid"].(string)
				// Add the new post to the newID string
				newIDs = append(newIDs, deviationid)
				postURLs[deviationid] = result["url"].(string)
			}
			nextPage++
		}
		feed.lastQueryTime = time.Now()
		feed.lastPostTime = newLastPostTime

		// Get the deviation objects.
		newDeviations := getDeviations(newIDs)

		// Put them into the output queue.
		for _, deviation := range newDeviations {
			deviation.URL = postURLs[deviation.Deviationid]
			downloadQueue <- deviation
		}

		// Put the current seach back into the queue.
		dAFollows <- feed
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
	dAFollows = make(chan dAFeed, 1)
	dAFollows <- dAFeed{dATagQuery{tag:"vernid"}, time.Now().Add(-10 * time.Minute), 1575047866}

	// Spawn a worker for each in the range of workers.
	for i := 0; i < workers; i++ {
		go dADownloadWorker(downloadQueue)
	}

	return

}

func (d deviation) formatLink() string {
	return d.URL
}

func (deviation) siteName() string {
	return "deviantart"
}