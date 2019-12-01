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

// Configuration constants
// Time to wait between polls of deviantArt
const pollingDelay = 1 * time.Minute

// Reability constants
const urlEncoded = "application/x-www-form-urlencoded"

// Global objects
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

// dATag implements a tag (as part of a deviation)
type dATag struct {
	TagName string `json:"tag_name"`
	Sponsored bool `json:"sponsored"`
	Sponsor string `json:"sponsor"`
}

// dAUser implements a user (as part of a deviation)
type dAUser struct {
	Userid string `json:"userid"`
	Username string `json:"username"`
	UserType string `json:"type"`	
}

// dAFeed defines a stream to pull data from. It consists of metadata about the previous pull and the query that generates the feed.
type dAFeed struct {
	dAFeedQuery
	lastQueryTime time.Time
	lastPostTime int64
}

type dAFeedQuery interface {
	// getDAResults queries deviantArt for the most recent posts from the follow.
	getDAResults(offset int) map[string]interface{}
}

// dATagQuery defines a query that follows a specific tag.
type dATagQuery struct {
	tag string
}

func (f dATagQuery) getDAResults(offset int) map[string]interface{} {
	// Build parameters
	params := url.Values{}
	params.Add("tag", f.tag)
	params.Add("offset", strconv.Itoa(offset))
	
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

// dAUserQuery defines a query that follows a specific user's gallary.
type dAUserQuery struct {
	user string
}

func (f dAUserQuery) getDAResults(offset int) map[string]interface{} {
	// Build parameters
	params := url.Values{}
	params.Add("username", f.user)
	params.Add("offset", strconv.Itoa(offset))
	
	dAAccessToken.RLock()
	params.Add("access_token", dAAccessToken.token)
	dAAccessToken.RUnlock()

	requestSting := params.Encode()

	// Send request
	resp, err := http.Get(fmt.Sprintf("https://www.deviantart.com/api/v1/oauth2/gallery/all?%s", requestSting))
		
	if err != nil {
		log.Panicln(err)
	}

	// Decode the results
	var result map[string]interface{}
	
	json.NewDecoder(resp.Body).Decode(&result)
	
	return result
}


// Global variable for access token storage.
var dAAccessToken struct {
	sync.RWMutex
	token string
}

// Convience wrapper to get a single deviation by id.
func getDeviation(id string) deviation {
	return getDeviations([]string{id})[0]
}

// getDeviations pulls the metadata about a list of deviations from DeviantArt.
func getDeviations(ids []string) []deviation {
	// If there are too many ids to do in one go, run two queries and append the results.
	if len(ids) > 50 {
		return append(getDeviations(ids[:50]), getDeviations(ids[50:])...)
	}

	// Build parameter list
	params := url.Values{}
	for _, id := range ids {
		params.Add("deviationids[]", id)
	}
	dAAccessToken.RLock()
	params.Add("access_token", dAAccessToken.token)
	dAAccessToken.RUnlock()

	// Send query
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

// dADownloadWorker defines a goroutine which pulls from the follow channel, downloads from the feed and puts results in the downloadQueue
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
		offset := 0
		dAResultParseLoop:
		for {
			// Pull from feed and extract results.
			query := feed.getDAResults(offset)
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
			// If we haven't hit old posts yet, move to the next page.
			offset = int(query["next_offset"].(float64))
		}
		// Set the lastQueryTime and lastPostTime to the current values
		feed.lastQueryTime = time.Now()
		feed.lastPostTime = newLastPostTime

		// Get the deviation objects.
		newDeviations := getDeviations(newIDs)

		// Put them into the output queue.
		for _, deviation := range newDeviations {
			// Set URL from the list we store before sending them off.
			deviation.URL = postURLs[deviation.Deviationid]
			downloadQueue <- deviation
		}

		// Put the current seach back into the queue.
		dAFollows <- feed
	}
}

// getDAAcessToken refreshes the access token stored in dAAccessToken
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

// createDownloadStream spawns goroutines to follow the deviantart streams.
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
	tagList := []dAFeed{dAFeed{dATagQuery{tag:"vernid"}, time.Time{}, 1575047866},
							dAFeed{dAUserQuery{user:"LiLaiRa"}, time.Time{}, 1575090419},}
	dAFollows = make(chan dAFeed, len(tagList))
	for _, tag := range tagList {
		dAFollows <- tag
	}

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

func (d deviation) getID() string {
	return d.Deviationid
}