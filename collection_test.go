// Copyright 2011 Gary Burd
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

func TestUpdate(t *testing.T) {
	c := dialAndDrop(t, "go-mongo-test", "test")
	defer c.Conn.Close()

	id := NewObjectId()
	err := c.Insert(M{"_id": id, "x": 1})
	if err != nil {
		t.Fatal("insert", err)
	}

	err = c.Update(M{"_id": id}, M{"$inc": M{"x": 1}})
	if err != nil {
		t.Fatal("update", err)
	}

	var m M
	err = c.Find(M{"_id": id}).One(&m)
	if err != nil {
		t.Fatal("findone after update", err)
	}

	if m["x"] != 2 {
		t.Error("expect x = 2, got", m["x"])
	}

	err = c.Update(M{"_id": "junk"}, M{"$inc": M{"x": 1}})
	if err != ErrNotFound {
		t.Error("update, expected NotFound, got", err)
	}

	err = c.UpdateAll(M{"_id": "junk"}, M{"$inc": M{"x": 1}})
	if err != ErrNotFound {
		t.Error("updateall, expected NotFound, got", err)
	}
}

func TestRemove(t *testing.T) {
	c := dialAndDrop(t, "go-mongo-test", "test")
	defer c.Conn.Close()

	id := NewObjectId()
	err := c.Insert(M{"_id": id, "x": 1})
	if err != nil {
		t.Fatal("insert", err)
	}

	err = c.Remove(M{"_id": id})
	if err != nil {
		t.Fatal("remove", err)
	}

	var m M
	err = c.Find(M{"_id": id}).One(&m)
	if err != Done {
		t.Fatal("findone, expect EOF, got", err)
	}
}

var indexNameTests = []struct {
	keys D
	name string
}{
	{D{{"up", 1}, {"down", -1}, {"geo", "2d"}}, "up_1_down_-1_geo_2d"},
}

func TestIndexName(t *testing.T) {
	for _, tt := range indexNameTests {
		name := IndexName(tt.keys)
		if name != tt.name {
			t.Errorf("%v, name=%s, want %s\n", tt.keys, name, tt.name)
		}
	}
}
