// Copyright 2013 Gary Burd
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

package mongo

import (
	"testing"
)

func TestLastError(t *testing.T) {
	c := dialAndDrop(t, "go-mongo-test", "test")
	defer c.Conn.Close()
	c.LastErrorCmd = nil

	// Insert duplicate id to create an error.
	id := NewObjectId()
	for i := 0; i < 2; i++ {
		c.Insert(M{"_id": id})
	}

	_, err := c.Db().LastError(nil)
	if err, ok := err.(*MongoError); !ok {
		t.Fatalf("expected error, got %+v", err)
	} else if err.Code == 0 {
		t.Fatalf("error code not set, %+v", err)
	}
}

func TestRunCommand(t *testing.T) {
	c, err := Dial("127.0.0.1")
	if err != nil {
		t.Fatal("dial", err)
	}
	defer c.Close()

	db := Database{c, "admin", nil}

	var m M
	err = db.Run(D{{"buildInfo", 1}}, &m)
	if err != nil {
		t.Fatal("runcommand", err)
	}
	if len(m) == 0 {
		t.Fatal("command result not set")
	}
	m = nil
	err = db.Run(D{{"thisIsNotACommand", 1}}, &m)
	if err == nil {
		t.Fatal("error not returned for bad command")
	}
}

func TestDBRef(t *testing.T) {
	c := dialAndDrop(t, "go-mongo-test", "test")
	defer c.Conn.Close()

	id := NewObjectId()
	err := c.Insert(M{"_id": id})
	if err != nil {
		t.Fatal("insert", err)
	}

	ref := DBRef{Id: id, Collection: c.Name()}
	var m M
	err = c.Db().Dereference(ref, false, &m)
	if err != nil {
		t.Fatal("dereference", err)
	}

	if m["_id"] != id {
		t.Fatalf("m[_id] = %v, want %v", m["_id"], id)
	}
}

func TestAuthenticate(t *testing.T) {
	c := dialAndDrop(t, "go-mongo-test", "test")
	defer c.Conn.Close()

	db := c.Db()
	err := db.AddUser("name", "password", false)
	if err != nil {
		t.Fatalf("AddUser %v", err)
	}

	err = db.Authenticate("name", "password")
	if err != nil {
		t.Fatalf("Authentitate(name, password) returned %v", err)
	}
	err = db.Authenticate("name", "bad")
	if err == nil {
		t.Fatalf("Authentitate(name, bad) returned %v", err)
	}
	db.RemoveUser("name")
}
