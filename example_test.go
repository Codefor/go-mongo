// Copyright 2010 Gary Burd
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package mongo_test

import (
	"github.com/garyburd/go-mongo/mongo"
	"log"
)

type ExampleDoc struct {
	Id    mongo.ObjectId `bson:"_id"`
	Title string
	Body  string
}

func Example() {

	// Connect to server.

	conn, err := mongo.Dial("localhost")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	c := mongo.Collection{conn, "example-db.example-collection", mongo.DefaultLastErrorCmd}

	// Insert a document.

	id := mongo.NewObjectId()

	err = c.Insert(&ExampleDoc{Id: id, Title: "Hello", Body: "Mongo is fun."})
	if err != nil {
		log.Fatal(err)
	}

	// Find the document.

	var doc ExampleDoc
	err = c.Find(map[string]interface{}{"_id": id}).One(&doc)
	if err != nil {
		log.Fatal(err)
	}

	log.Print(doc.Title, " ", doc.Body)
}
