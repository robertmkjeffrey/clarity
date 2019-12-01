package main

import (
	"log"
	"go.mongodb.org/mongo-driver/bson"
	"context"
	"fmt"
)

// deletePost deletes a post from the database based on its site and id. 
func deletePost(site string, id string) {
	collection := database.Collection(fmt.Sprintf("%sPosts", site))
	_, err := collection.DeleteMany(
		context.Background(),
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
		context.Background(),
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"notify": notification}},
	)
	if err != nil {
		log.Panicln(err)
	}
}