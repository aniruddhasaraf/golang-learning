package models

import "go.mongodb.org/mongo-driver/v2/bson"

type User struct {
	ID    bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name  string        `bson:"name" json:"name"`
	Email string        `bson:"email" json:"email"`
	Age   int           `bson:"age" json:"age"`
}