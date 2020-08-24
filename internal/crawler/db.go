package crawler

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"time"
)

func ResultExists(collection *mongo.Collection, name string) bool {
	//r := whois.Result{}
	filter := bson.M{"name": name}
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	_, err := collection.FindOne(ctx, filter).DecodeBytes()
	if err != nil {
		return false
	}
	return true
}
func GetResult(collection *mongo.Collection, name string) bool {
	ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
	cur, err := collection.Find(ctx, bson.M{"name": name})
	if err != nil {
		log.Fatal(err)
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var result bson.M
		err := cur.Decode(&result)
		if err != nil {
			log.Fatal(err)
		}

	}
	if err := cur.Err(); err != nil {
		log.Fatal(err)
	}
	return true
}
