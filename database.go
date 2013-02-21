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
	"crypto/md5"
	"encoding/hex"
	"errors"
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

func (e *MongoError) Error() string {
	return e.Err
}

// CommandResponse contains the common fields in command responses from the
// server.
type CommandResponse struct {
	Ok     bool   `bson:"ok"`
	Errmsg string `bson:"errmsg"`
}

// Error returns the error from the response or nil.
func (s CommandResponse) Err() error {
	if s.Ok {
		return nil
	}

	errmsg := s.Errmsg
	if errmsg == "" {
		errmsg = "unspecified error"
	}

	return errors.New(errmsg)
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

func runInternal(conn Conn, dbname string, cmd interface{}, options *FindOptions, result interface{}) error {
	cursor, err := conn.Find(dbname+".$cmd", cmd, options)
	if err != nil {
		return err
	}
	defer cursor.Close()
	return cursor.Next(result)
}

// Run runs the command cmd on the database.
//
// More information: http://www.mongodb.org/display/DOCS/Commands
func (db Database) Run(cmd interface{}, result interface{}) error {
	var d BSONData
	err := runInternal(db.Conn, db.Name, cmd, runFindOptions, &d)
	if err != nil {
		return err
	}
	var r CommandResponse
	if err := Decode(d.Data, &r); err != nil {
		return err
	}
	if err := r.Err(); err != nil {
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
func (db Database) LastError(cmd interface{}) (*MongoError, error) {
	if cmd == nil {
		cmd = DefaultLastErrorCmd
	}
	var r struct {
		CommandResponse
		MongoError
	}
	err := runInternal(db.Conn, db.Name, cmd, runFindOptions, &r)
	if err == nil {
		err = r.CommandResponse.Err()
		if err == nil && r.MongoError.Err != "" {
			err = &r.MongoError
		}
	}
	return &r.MongoError, err
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
	Database string `bson:"$db,omitempty"`
}

func passwordDigest(name, password string) string {
	h := md5.New()
	h.Write([]byte(name + ":mongo:" + password))
	return hex.EncodeToString(h.Sum(nil))
}

// Deference fetches the document specified by a database reference.
func (db Database) Dereference(ref DBRef, slaveOk bool, result interface{}) error {
	if ref.Database != "" {
		db.Name = ref.Database
	}
	return db.C(ref.Collection).Find(M{"_id": ref.Id}).SlaveOk(slaveOk).One(result)
}

// AddUser creates a user with name and password. If the user already exists,
// then the password is updated.
func (db Database) AddUser(name, password string, readOnly bool) error {
	users := db.C("system.users")
	return users.Upsert(
		M{"user": name},
		M{"$set": M{
			"user":     name,
			"pwd":      passwordDigest(name, password),
			"readOnly": readOnly}})
}

// RemoveUser removes user with name from the database.
func (db Database) RemoveUser(name string) error {
	users := db.C("system.users")
	return users.Remove(M{"user": name})
}

// Authenticate authenticates user with name and password to this database.
func (db Database) Authenticate(name, password string) error {
	var r struct {
		CommandResponse
		Nonce string `bson:"nonce"`
	}
	if err := runInternal(db.Conn, db.Name, M{"getnonce": 1}, runFindOptions, &r); err != nil {
		return err
	}
	if err := r.Err(); err != nil {
		return err
	}
	h := md5.New()
	h.Write([]byte(r.Nonce + name))
	h.Write([]byte(passwordDigest(name, password)))
	key := hex.EncodeToString(h.Sum(nil))

	cmd := D{{"authenticate", 1}, {"user", name}, {"nonce", r.Nonce}, {"key", key}}

	var s CommandResponse
	if err := runInternal(db.Conn, db.Name, cmd, runFindOptions, &s); err != nil {
		return err
	}
	return s.Err()
}
