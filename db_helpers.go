package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/bson"
)

func parseSiteName(text string) (streamablePost, error) {
	for _, site := range siteTypes {
		if text == site.siteName() || text == site.prettySiteName() {
			return site, nil
		}
	}
	return nil, errors.New("invalid site name")
}

func getPost(site string, id string) (streamablePost, error) {
	siteType, err := parseSiteName(site)
	if err != nil {
		return nil, err
	}

	collection := database.Collection(fmt.Sprintf("%sPosts", site))
	singleResult := collection.FindOne(
		context.TODO(),
		bson.M{"_id": id},
	)

	result, err := siteType.decodeDBResult(singleResult)

	if err != nil {
		log.Println("got error")
		return nil, err
	}

	return result, nil
}

// deletePost deletes a post from the database based on its site and id.
func deletePost(site string, id string) {
	collection := database.Collection(fmt.Sprintf("%sPosts", site))
	_, err := collection.DeleteMany(
		context.TODO(),
		bson.M{"_id": id},
	)
	if err != nil {
		log.Panicln(err)
	}
}

// updatePostNotify sets the notify parameter of a post in the database.
func updatePostNotify(site string, id string, notification bool) {
	collection := database.Collection(fmt.Sprintf("%sPosts", site))
	_, err := collection.UpdateOne(
		context.TODO(),
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"notify": notification}},
	)
	if err != nil {
		log.Panicln(err)
	}
}
