// Copyright 2011 Gary Burd
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package mongo

import (
	"os"
	"strings"
)

var (
	runFindOptions = &FindOptions{BatchSize: -1, SlaveOk: false}
)

// SplitNamespace splits a namespace into database name and collection name
// components.
func SplitNamespace(s string) (string, string) {
	if i := strings.Index(s, "."); i > 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// MongoError represents an error for the connection mutation operations.
type MongoError struct {
	Err        string      `bson:"err"`
	N          int         `bson:"n"`
	Code       int         `bson:"code"`
	Updated    bool        `bson:"updatedExisting"`
	UpsertedId interface{} `bson:"upserted"`
}

func (e *MongoError) String() string {
	return e.Err
}

// CommandResponse contains the common fields in command responses from the
// server. 
type CommandResponse struct {
	Ok     bool   `bson:"ok"`
	Errmsg string `bson:"errmsg"`
}

// Error returns the error from the response or nil.
func (s CommandResponse) Error() os.Error {
	if s.Ok {
		return nil
	}

	errmsg := s.Errmsg
	if errmsg == "" {
		errmsg = "unspecified error"
	}

	return os.NewError(errmsg)
}

// Database represents a MongoDb database.
type Database struct {
	// Connection to the database.
	Conn Conn

	// Database name.
	Name string

	// Command used to check for errors after on insert, update or remove
	// operation on the collection. If nil, then errors are not checked.
	LastErrorCmd interface{}
}

// C returns the collection with name. This is a lightweight operation. The
// method does not check to see if the collection exists in the database.
func (db Database) C(name string) Collection {
	return Collection{
		Conn:         db.Conn,
		Namespace:    db.Name + "." + name,
		LastErrorCmd: db.LastErrorCmd,
	}
}

// Run runs the command cmd on the database.
// 
// More information: http://www.mongodb.org/display/DOCS/Commands
func (db Database) Run(cmd interface{}, result interface{}) os.Error {
	cursor, err := db.Conn.Find(db.Name+".$cmd", cmd, runFindOptions)
	if err != nil {
		return err
	}

	var d BSONData
	if err := cursor.Next(&d); err != nil {
		return err
	}

	var r CommandResponse
	if err := Decode(d.Data, &r); err != nil {
		return err
	}
	if err := r.Error(); err != nil {
		return err
	}

	if result != nil {
		if err := d.Decode(result); err != nil {
			return err
		}
	}
	return nil
}

// LastError returns the last error for the database using cmd. If cmd is nil,
// then the command {"getLasetError": 1} is used to get the error.
//
// More information: http://www.mongodb.org/display/DOCS/Last+Error+Commands
func (db Database) LastError(cmd interface{}) (*MongoError, os.Error) {
	if cmd == nil {
		cmd = DefaultLastErrorCmd
	}
	var r struct {
		CommandResponse
		MongoError
	}
	cursor, err := db.Conn.Find(db.Name+".$cmd", cmd, runFindOptions)
	if err != nil {
		return &r.MongoError, err
	}
	defer cursor.Close()
	if err := cursor.Next(&r); err != nil {
		return &r.MongoError, err
	}
	if err := r.CommandResponse.Error(); err != nil {
		return &r.MongoError, err
	}
	if r.MongoError.Err != "" {
		return &r.MongoError, &r.MongoError
	}
	return &r.MongoError, nil
}

// DBRef is a reference to a document in a database. Use the Database
// Dereference method to get the referenced document. 
//
// More information: http://www.mongodb.org/display/DOCS/Database+References 
type DBRef struct {
	// The target document's collection.
	Collection string `bson:"$ref"`

	// The target document's id.
	Id ObjectId `bson:"$id"`

	// The target document's database (optional).
	Database string `bson:"$db/c"`
}

// Deference fetches the document specified by a database reference.
func (db Database) Dereference(ref DBRef, slaveOk bool, result interface{}) os.Error {
	if ref.Database != "" {
		db.Name = ref.Database
	}
	return db.C(ref.Collection).Find(M{"_id": ref.Id}).SlaveOk(slaveOk).One(result)
}
