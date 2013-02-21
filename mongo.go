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

// Package mongo is a driver for MongoDB.
//
// The core interface to MongoDB is defined by the Conn interface. This
// interface provides access to all MongoDB functionality, but it is is not
// always convenient to use for common tasks.
//
// The Database, Collection and Query types provide a number of convenience
// methods for working with Conn objects.
//
// Conn objects are not thread-safe. Multi-threaded applications are
// responsible for serializing access to Conn objects.
package mongo

import "errors"

// Cursor has no more results.
var Done = errors.New("mongo: cursor has no more results")

// InsertOptions specifies options for the Conn.Insert method.
type InsertOptions struct {
	// If true, the server will not stop processing a bulk insert if one insert fails.
	ContinueOnError bool
}

// RemoveOptions specifies options for the Conn.Remove method.
type RemoveOptions struct {
	// If true, then the database removes the first matching document in the
	// collection. Otherwise all matching documents are removed.
	Single bool
}

// UpdateOptions specifies options for the Conn.Update method.
type UpdateOptions struct {
	// If true, then the database inserts the supplied object into the
	// collection if no matching document is found.
	Upsert bool

	// If true, then the database updates all objects matching the query.
	Multi bool
}

// FindOptions specifies options for the Conn.Find method.
type FindOptions struct {
	// Optional document that limits the fields in the returned documents.
	// Fields contains one or more elements, each of which is the name of a
	// field that should be returned, and the integer value 1.
	Fields interface{}

	// Do not close the cursor when no more data is available on the server.
	Tailable bool

	// Allow query of replica slave.
	SlaveOk bool

	// Do not close the cursor on the server after a period of inactivity (10
	// minutes).
	NoCursorTimeout bool

	// Block at server for a short time if there's no data for a tailable cursor.
	AwaitData bool

	// Stream the data down from the server full blast. Normally the server
	// waits for a "get more" message before sending a batch of data to the
	// client. With this option set, the server sends batches of data without
	// waiting for the "get more" messages.
	Exhaust bool

	// Allow partial results in sharded environment. Normally the query
	// will fail if a shard is not available.
	PartialResults bool

	// Skip specifies the number of documents the server should skip at the
	// beginning of the result set.
	Skip int

	// Sets the number of documents to return.
	Limit int

	// Sets the batch size used for sending documents from the server to the
	// client.
	BatchSize int
}

// A Conn represents a connection to a MongoDB server.
//
// When the application is done using the connection, the application must call
// the connection Close() method to release the resources used by the
// connection.
//
// The methods in this interface use a namespace string to specify the database
// and collection. A namespace string has the format "<database>.<collection>"
// where <database> is the name of the database and <collection> is the name of
// the collection.
type Conn interface {
	// Close releases the resources used by this connection.
	Close() error

	// Error returns non-nil if the connection has a permanent error.
	Err() error

	// Update document specified by selector with update.
	Update(namespace string, selector, update interface{}, options *UpdateOptions) error

	// Insert documents.
	Insert(namespace string, options *InsertOptions, documents ...interface{}) error

	// Remove documents specified by selector.
	Remove(namespace string, selector interface{}, options *RemoveOptions) error

	// Find documents specified by selector. The returned cursor must be closed.
	Find(namespace string, query interface{}, options *FindOptions) (Cursor, error)
}

// Cursor iterates over the results from a Find operation.
//
// When the application is done using a cursor, the application must call the
// cursor Close() method to release the resources used by the cursor.
//
// An example use of a cursor is:
//
//  cursor, err := c.Find("db.coll", mongo.Doc{}, nil)
//  if err != nil {
//      return err
//  }
//  defer cursor.Close()
//
//  for cursor.HasNext() {
//      var m map[string]interface{}
//      err = r.Next(&m)
//      if err != nil {
//          return err
//      }
//      // Do something with result document m.
//	}
//
// Tailable cursors are supported. When working with a tailable cursor, use the
// expression cursor.Err() != nil to determine if the cursor is "dead." See
// http://www.mongodb.org/display/DOCS/Tailable+Cursors for more discussion on
// tailable cursors.
type Cursor interface {
	// Close releases the resources used by this connection.
	Close() error

	// Err returns non-nil if the cursor has a permanent error.
	Err() error

	// HasNext returns true if there are more documents to retrieve.
	HasNext() bool

	// Next fetches the next document from the cursor. Value must be a map or
	// a non-nil pointer to struct or map.
	Next(value interface{}) error
}
