package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/mongo"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"go.mongodb.org/mongo-driver/bson"

	"jaytaylor.com/html2text"
)

// Current Version: 20200519

// Configuration constants
// Time to wait between polls of deviantArt
const pollingDelay = 5 * time.Minute

// When a new feed is added, get all posts in the last initialHistoryAmount seconds.
const initialHistoryAmount = 50_000_000

// Maximum number of pages to download before ending
const maxPages = 5

// Reability constants
const urlEncoded = "application/x-www-form-urlencoded"
const deviantartFeedCollection = "deviantartFeeds"

// Global objects
var dAFollows struct {
	sync.RWMutex
	feedChannel chan dAFeed // Circular channel of followed users
}

var dANewFeedSignal chan struct{}

// TODO: Can I clean up the json/bson for these ones?
// deviation implements the streamablePost interface, represeting a post drawn from deviantArt.
type deviation struct {
	Deviationid     string  `json:"deviationid" bson:"_id"`
	URL             string  `json:"url" bson:"url"`
	Author          dAUser  `json:"author" bson:"author"`
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	License         string  `json:"license"`
	AllowsComments  bool    `json:"allows_comments" bson:"allows_comments"`
	Tags            []dATag `json:"tags"`
	IsMature        bool    `json:"is_mature" bson:"is_mature"`
	forceNotifyFlag bool
}

// dATag implements a tag (as part of a deviation)
type dATag struct {
	TagName   string `json:"tag_name" bson:"tag_name"`
	Sponsored bool   `json:"sponsored"`
	Sponsor   string `json:"sponsor"`
}

// dAUser implements a user (as part of a deviation)
type dAUser struct {
	Userid   string `json:"userid"`
	Username string `json:"username"`
	UserType string `json:"type" bson:"user_type"`
}

// dAFeed defines a stream to pull data from. It consists of metadata about the previous pull and the query that generates the feed.
type dAFeed struct {
	FeedType      string    `bson:"feed_type"`
	Query         string    `bson:"query"`
	LastQueryTime time.Time `bson:"last_query_time"`
	LastPostTime  int64     `bson:"last_post_time"`
}

func (f dAFeed) getDAResults(offset int) map[string]interface{} {
	// Create parameter object to build url
	params := url.Values{}
	var apiURL string

	switch f.FeedType {
	case "user":
		params.Add("username", f.Query)
		apiURL = "https://www.deviantart.com/api/v1/oauth2/gallery/all"
	case "tag":
		params.Add("q", f.Query)
		apiURL = "https://www.deviantart.com/api/v1/oauth2/browse/newest"
	default:
		log.Panicf("Error: Invalid feed type \"%s\"\n", f.FeedType)
	}
	// Build parameters
	params.Add("offset", strconv.Itoa(offset))
	params.Add("mature_content", "true")

	dAAccessToken.RLock()
	params.Add("access_token", dAAccessToken.token)
	dAAccessToken.RUnlock()

	requestSting := params.Encode()

	// Send request
	resp, err := http.Get(fmt.Sprintf("%s?%s", apiURL, requestSting))
	// Every time response fails, do exponential backoff and retry
	attempts := 1
	for err != nil && attempts < 10 {
		// Calculate sleep time (2 ^ attempts)
		backoff := int(math.Pow(float64(2), float64(attempts)))
		log.Printf("Failed query to %v, retrying in %v seconds.", f.Query, backoff)
		time.Sleep(time.Duration(backoff) * time.Second)
		// Make response
		resp, err = http.Get(fmt.Sprintf("%s?%s", apiURL, requestSting))
		attempts++
	}
	// If after 10 attempts the error is still present, throw it.
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
func getDeviation(id string) (deviation, error) {
	deviations := getDeviations([]string{id})
	if len(deviations) == 0 {
		return deviation{}, errors.New("deviation not found")
	}
	return deviations[0], nil
}

// getDeviations pulls the metadata about a list of deviations from DeviantArt.
func getDeviations(ids []string) []deviation {
	// NOTE: This won't download URLs! Use getURL in addition for that.

	// If there are too many ids to do in one go, run two queries and append the results.
	if len(ids) > 50 {
		return append(getDeviations(ids[:50]), getDeviations(ids[50:])...)
	}

	// Build parameter list
	params := url.Values{}
	params.Add("mature_content", "true")
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

func (d *deviation) addURL() {
	// Get URL by looking up deviation
	// Build parameter list
	params := url.Values{}
	dAAccessToken.RLock()
	params.Add("access_token", dAAccessToken.token)
	dAAccessToken.RUnlock()

	// Send query
	resp, err := http.Get(fmt.Sprintf("https://www.deviantart.com/api/v1/oauth2/deviation/%s?%s", d.Deviationid, params.Encode()))
	if err != nil {
		log.Panicln(err)
	}

	// Decode the results. Anonomous struct to remove the top level metadata field.
	var results struct {
		URL string `json:"url"`
	}

	json.NewDecoder(resp.Body).Decode(&results)

	d.URL = results.URL
}

func (d deviation) decodeDBResult(dbResult *mongo.SingleResult) (result streamablePost, err error) {
	post := deviation{}
	err = dbResult.Decode(&post)
	result = post
	return
}

// dADownloadWorker defines a goroutine which pulls from the follow channel, downloads from the feed and puts results in the downloadQueue
func dADownloadWorker(writeQueue chan<- postMessage) {

	for {
		// Get the next search and wait until the next polling opportunity
		dAFollows.RLock()
		feed := <-dAFollows.feedChannel
		dAFollows.RUnlock()
		// Wait for polling time from last query
		<-time.After(pollingDelay - time.Since(feed.LastQueryTime))

		if debug {
			log.Printf("Polling deviantart feed \"%s\"\n", feed.Query)
		}

		// Store the new ids to analyse in one go.
		newIDs := make([]string, 0)
		postURLs := make(map[string]string)

		newLastPostTime := feed.LastPostTime
		offset := 0
	dAResultParseLoop:
		for page := 0; page < maxPages; page++ {
			// Pull from feed and extract results.
			query := feed.getDAResults(offset)
			// TODO: Remove this type switch after finding the error (DEBUG)
			var results []interface{}
			switch r := query["results"].(type) {
			case []interface{}:
				results = r
			case nil:
				log.Println("Got nil results!")
				log.Println("Query: ", query)
				continue
			}
			// results := query["results"].([]interface{})

			// If the result list is empty, skip.
			if debug && len(results) == 0 {
				log.Printf("Skipping query %s (empty result list).\n", feed.Query)
				break dAResultParseLoop
			}

			for _, result := range results {
				result := result.(map[string]interface{})

				// Parse the published time return value
				publishedTime, err := strconv.ParseInt(result["published_time"].(string), 10, 64)
				if err != nil {
					log.Panicln(err)
				}
				// If the result is older than the last parse time, end the query.
				if publishedTime <= feed.LastPostTime {
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
			// If we're out of posts, quit the loop.
			if !query["has_more"].(bool) {
				break dAResultParseLoop
			}
			// If we haven't hit old posts yet, move to the next page.
			offset = int(query["next_offset"].(float64))
		}
		// Set the lastQueryTime and lastPostTime to the current values
		feed.LastQueryTime = time.Now()
		feed.LastPostTime = newLastPostTime

		// Update the feed object in the database.
		filter := bson.M{"feed_type": feed.FeedType, "query": feed.Query}
		update := bson.M{"$set": bson.M{"last_query_time": feed.LastQueryTime, "last_post_time": feed.LastPostTime}}
		_, err := database.Collection(deviantartFeedCollection).UpdateOne(context.TODO(), filter, update)
		if err != nil {
			log.Panicln(err)
		}

		// Get the deviation objects.
		newDeviations := getDeviations(newIDs)

		// Put them into the output queue.
		for _, deviation := range newDeviations {
			// Set URL from the list we store before sending them off.
			deviation.URL = postURLs[deviation.Deviationid]
			writeQueue <- postMessage{
				post:        deviation,
				forceNotify: false,
				skipWrite:   false,
			}
		}

		// Put the current seach back into the queue.
		dAFollows.RLock()
		dAFollows.feedChannel <- feed
		dAFollows.RUnlock()
	}
}

// getDAAcessToken refreshes the access token stored in dAAccessToken
func getDAAccessToken() {

	dAKeys, ok := keys["deviantArt"].(map[interface{}]interface{})
	if !ok {
		log.Printf("Error with DeviantArt keys. \n Message: \n")
		dAKeys = keys["deviantArt"].(map[interface{}]interface{})
	}

	// Build url encoding of request.
	params := url.Values{}
	params.Add("grant_type", "client_credentials")
	dA_client_id, ok := dAKeys["client_id"].(string)
	if !ok {
		dA_client_id_int, ok := dAKeys["client_id"].(int)
		if !ok {
			log.Panicln("DeviantArt client_id is missing or malformed.")
		}
		dA_client_id = fmt.Sprint(dA_client_id_int)
	}
	dA_client_secret, ok := dAKeys["client_secret"].(string)
	if !ok {
		log.Panicln("DeviantArt client_secret is missing or malformed.")
	}

	params.Add("client_id", dA_client_id)
	params.Add("client_secret", dA_client_secret)

	requestSting := params.Encode()

	// Send request
	resp, err := http.Post("https://www.deviantart.com/oauth2/token",
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

func dASupervisor() {
	// TODO: Implement
}

// createDownloadStream spawns goroutines to follow the deviantart streams.
func (deviation) createDownloadStream(writeQueue chan<- postMessage, workers int) {

	// Read follow files from database and add to queue.
	var tagList []dAFeed
	cursor, err := database.Collection(deviantartFeedCollection).Find(context.TODO(), bson.D{})
	if err != nil {
		log.Fatalln(err)
	}

	// Decode all results
	err = cursor.All(context.TODO(), &tagList)
	if err != nil {
		log.Fatalln(err)
	}

	dAFollows.Lock()
	// Create the follow queue and add all tags to it.
	dAFollows.feedChannel = make(chan dAFeed, len(tagList))
	for _, tag := range tagList {
		dAFollows.feedChannel <- tag
	}
	dAFollows.Unlock()

	dANewFeedSignal = make(chan struct{})

	// If there are no feeds, we don't want to start polling.
	// Print a warning. Wait for new feeds to be added before continuing.
	if len(tagList) == 0 {
		log.Println("No DeviantArt feeds found. Waiting for new feeds via telegram.")
		<-dANewFeedSignal
	}

	// Request an access token.
	getDAAccessToken()

	// Every 59 minutes, get a new access token. Token expires every 60 minutes.
	go func() {
		for {
			time.Sleep(59 * time.Minute)
			getDAAccessToken()
		}
	}()

	// Spawn a supervisor task
	go dASupervisor()

	// Spawn a worker for each in the range of workers.
	for i := 0; i < workers; i++ {
		go dADownloadWorker(writeQueue)
	}

	log.Printf("Started %d DeviantArt workers.\n", workers)

}

func (d deviation) formatLink() string {
	return d.URL
}

func (d deviation) formatPost() string {
	description, err := html2text.FromString(d.Description)
	if err != nil {
		log.Panicln(err)
	}
	return fmt.Sprintf("%s\n"+
		"--------------------------------------------------------------------------------------\n"+
		"%s", d.Title, description)
}

func (deviation) siteName() string {
	return "deviantart"
}

func (deviation) prettySiteName() string {
	return "DeviantArt"
}

func (d deviation) getID() string {
	return d.Deviationid
}

func (deviation) addFollowHandler() func(tgbotapi.Update) (bool, interface{}) {
	msg := tgbotapi.NewMessage(chatID, "What type of follow would you like to add?")
	replyKeyboard := tgbotapi.NewReplyKeyboard(tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("Tag")),
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("User")))
	replyKeyboard.OneTimeKeyboard = true
	replyKeyboard.ResizeKeyboard = true
	msg.ReplyMarkup = replyKeyboard

	telegramBot.Send(msg)
	return handleFollowType
}

func handleFollowType(update tgbotapi.Update) (waitForResponse bool, responseHandler interface{}) {
	var msg tgbotapi.MessageConfig
	switch update.Message.Text {
	case "Tag":
		msg = tgbotapi.NewMessage(update.Message.Chat.ID, "And what tag would you like to follow?")
		waitForResponse = true
		responseHandler = func(update tgbotapi.Update) (bool, interface{}) {
			return handleAddFeed("tag", update)
		}
	case "User":
		msg = tgbotapi.NewMessage(update.Message.Chat.ID, "And what user would you like to follow?")
		waitForResponse = true
		responseHandler = func(update tgbotapi.Update) (bool, interface{}) {
			return handleAddFeed("user", update)
		}
	default:
		msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Sorry, I don't recognise that follow type. Please start again.")
		waitForResponse = false
		responseHandler = interface{}(nil)
	}
	telegramBot.Send(msg)
	return
}

func handleAddFeed(feedType string, update tgbotapi.Update) (bool, interface{}) {

	// Check string contains whitespace, in which case break.
	if len(strings.Fields(update.Message.Text)) != 1 {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Invalid query - query must not contain whitespace.")
		telegramBot.Send(msg)
		return false, nil
	}

	query := strings.ToLower(update.Message.Text)

	// Create a new feed from the parameters and insert it.
	newFeed := dAFeed{
		FeedType:      feedType,
		Query:         query,
		LastPostTime:  time.Now().Unix() - initialHistoryAmount,
		LastQueryTime: time.Time{},
	}
	_, err := database.Collection(deviantartFeedCollection).InsertOne(context.TODO(), newFeed)
	if err != nil {
		log.Panicln(err)
	}

	// Send message to confirm.
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Added %s feed with query \"%s\"!", feedType, update.Message.Text))
	_, err = telegramBot.Send(msg)
	if err != nil {
		log.Panicln(err)
	}

	// Expand the current buffer and add the new feed,
	// Note - we do this after sending the message this can take a while (has to aquire global lock on the feed channel).
	dAFollows.Lock()
	// Expand buffer by one
	newChan := make(chan dAFeed, cap(dAFollows.feedChannel)+1)
	newChan <- newFeed
	// Put each of the previous feeds into the new channel.
	currentFeeds := len(dAFollows.feedChannel)
	for i := 0; i < currentFeeds; i++ {
		newChan <- <-dAFollows.feedChannel
	}
	dAFollows.feedChannel = newChan
	dAFollows.Unlock()

	select {
	case dANewFeedSignal <- struct{}{}:
		if debug {
			log.Println("Signaled new DeviantArt feed.")
		}
	default:
	}

	return false, nil
}

func (deviation) downloadPost(id string) (postMessage, error) {

	post, err := getDeviation(id)

	if err != nil {
		return postMessage{}, err
	}

	post.addURL()

	// log.Printf("URL: %s", post.URL)
	return postMessage{
		post:        post,
		forceNotify: false,
		skipWrite:   false,
	}, nil

}
