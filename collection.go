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
	"bytes"
	"errors"
	"strconv"
)

var DefaultLastErrorCmd interface{} = map[string]int{"getLastError": 1}

var ErrNotFound = errors.New("mongo: not found")

var (
	upsertOptions      = &UpdateOptions{Upsert: true}
	updateAllOptions   = &UpdateOptions{Multi: true}
	removeFirstOptions = &RemoveOptions{Single: true}
)

// Collection represents a MongoDB collection.
type Collection struct {
	// Connection to the database.
	Conn Conn

	// String with the format "<database>.<collection>" where <database> is the
	// name of the database and <collection> is the name of the collection.
	Namespace string

	// Command used to check for errors after on insert, update or remove
	// operation on the collection. If nil, then errors are not checked.
	LastErrorCmd interface{}
}

// Name returns the collection's name.
func (c Collection) Name() string {
	_, name := SplitNamespace(c.Namespace)
	return name
}

// Db returns the database for this collection.
func (c Collection) Db() Database {
	name, _ := SplitNamespace(c.Namespace)
	return Database{
		Conn:         c.Conn,
		Name:         name,
		LastErrorCmd: c.LastErrorCmd,
	}
}

func (c Collection) checkError(err error) (*MongoError, error) {
	if err != nil {
		return nil, err
	}
	if c.LastErrorCmd == nil {
		return nil, nil
	}
	return c.Db().LastError(c.LastErrorCmd)
}

// Insert adds document to the collection.
func (c Collection) Insert(documents ...interface{}) error {
	_, err := c.checkError(c.Conn.Insert(c.Namespace, nil, documents...))
	return err
}

// Update updates the first document in the collection found by selector with
// update. If a matching document is not found, then mongo.ErrNotFound is
// returned.
func (c Collection) Update(selector, update interface{}) error {
	merr, err := c.checkError(c.Conn.Update(c.Namespace, selector, update, nil))
	if merr != nil && err == nil && !merr.Updated {
		err = ErrNotFound
	}
	return err
}

// UpdateAll updates all documents matching selector with update. If no
// matching documents are found, then mongo.ErrNotFound is returned.
func (c Collection) UpdateAll(selector interface{}, update interface{}) error {
	merr, err := c.checkError(c.Conn.Update(c.Namespace, selector, update, updateAllOptions))
	if merr != nil && err == nil && !merr.Updated {
		err = ErrNotFound
	}
	return err
}

// Upsert updates the first document found by selector with update. If no
// document is found, then the update is inserted instead.
func (c Collection) Upsert(selector interface{}, update interface{}) error {
	_, err := c.checkError(c.Conn.Update(c.Namespace, selector, update, upsertOptions))
	return err
}

// RemoveFirst removes the first document found by selector.
func (c Collection) RemoveFirst(selector interface{}) error {
	_, err := c.checkError(c.Conn.Remove(c.Namespace, selector, removeFirstOptions))
	return err
}

// Remove removes all documents found by selector.
func (c Collection) Remove(selector interface{}) error {
	_, err := c.checkError(c.Conn.Remove(c.Namespace, selector, nil))
	return err
}

// Find returns a query object for the given filter.
func (c Collection) Find(filter interface{}) *Query {
	if filter == nil {
		filter = emptyDoc
	}
	return &Query{
		Conn:      c.Conn,
		Namespace: c.Namespace,
		Spec:      QuerySpec{Query: filter},
	}
}

// IndexName returns the standard name for an index with keys.
func IndexName(keys D) string {
	var buf bytes.Buffer
	for i, key := range keys {
		if i != 0 {
			buf.WriteByte('_')
		}
		buf.WriteString(key.Key)
		buf.WriteByte('_')
		switch v := key.Value.(type) {
		case int:
			buf.WriteString(strconv.Itoa(v))
		case string:
			buf.WriteString(v)
		default:
			panic("Index direction must be integer or string.")
		}
	}
	return buf.String()
}

// IndexOptions specifies options for the collection CreateIndex method.
//
// More information: http://www.mongodb.org/display/DOCS/Indexes
type IndexOptions struct {
	// Custom name for this index. If none specified, then a name will be generated.
	Name string `bson:"name"`

	// Should this index guarantee uniqueness?
	Unique bool `bson:"unique,omitempty"`

	// Should duplicates be dropped when creating a unique index?
	DropDups bool `bson:"dropDups,omitempty"`

	// Build index in background.
	Background bool `bson:"background,omitempty"`

	// Do not index documents with missing key fields.
	Sparse bool `bson:"sparse,omitempty"`

	// Geospatial options
	Min  interface{} `bson:"min"`
	Max  interface{} `bson:"max"`
	Bits int         `bson:"bits,omitempty"`
}

// CreateIndex creates an index on keys.
//
// More information: http://www.mongodb.org/display/DOCS/Indexes
func (c Collection) CreateIndex(keys D, options *IndexOptions) error {
	index := struct {
		Keys      D      `bson:"key"`
		Namespace string `bson:"ns"`
		IndexOptions
	}{
		Keys:      keys,
		Namespace: c.Namespace,
	}

	if options != nil {
		index.IndexOptions = *options
	}

	if index.Name == "" {
		index.Name = IndexName(keys)
	}

	if c.LastErrorCmd == nil {
		c.LastErrorCmd = DefaultLastErrorCmd
	}

	return c.Db().C("system.indexes").Insert(&index)
}
