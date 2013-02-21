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

var countTests = []struct {
	query interface{}
	limit int
	skip  int
	count int64
}{
	{limit: 100, count: 10},
	{limit: 5, count: 5},
	{skip: 5, count: 5},
	{skip: 100, count: 0},
	{query: D{{"x", 1}}, count: 1},
}

func TestCount(t *testing.T) {
	c := dialAndDrop(t, "go-mongo-test", "test")
	defer c.Conn.Close()

	for i := 0; i < 10; i++ {
		err := c.Insert(map[string]int{"x": i})
		if err != nil {
			t.Fatal("insert", err)
		}
	}

	for _, tt := range countTests {
		n, err := c.Find(tt.query).Limit(tt.limit).Skip(tt.skip).Count()
		if err != nil {
			t.Fatal("count", err)
		}
		if n != tt.count {
			t.Errorf("test: %+v, actual: %d", tt, n)
		}
	}
}

func TestQuery(t *testing.T) {
	c := dialAndDrop(t, "go-mongo-test", "test")
	defer c.Conn.Close()

	for i := 0; i < 10; i++ {
		err := c.Insert(map[string]int{"x": i})
		if err != nil {
			t.Fatal("insert", err)
		}
	}

	var m M
	err := c.Find(nil).Sort(D{{"x", -1}}).One(&m)
	if err != nil {
		t.Fatal("findone", err)
	}

	if m["x"] != 9 {
		t.Fatal("expect max value for descending sort")
	}
}

func TestFillAll(t *testing.T) {
	c := dialAndDrop(t, "go-mongo-test", "test")
	defer c.Conn.Close()

	for i := 0; i < 10; i++ {
		err := c.Insert(map[string]int{"x": i})
		if err != nil {
			t.Fatal("insert", err)
		}
	}

	p := make([]M, 11)
	n, err := c.Find(nil).Fill(p)
	if err != nil {
		t.Fatalf("fill() = %v", err)
	}
	if n != 10 {
		t.Fatalf("n=%d, want 10", n)
	}

	for i, m := range p[:n] {
		if m["x"] != i {
			t.Fatalf("p[%d][x]=%v, want %d", i, m["x"], i)
		}
	}

	p = nil
	err = c.Find(nil).All(&p)
	if err != nil {
		t.Fatalf("all() = %v", err)
	}
	if len(p) != 10 {
		t.Fatalf("len(p)=%d, want 10", n)
	}
	for i, m := range p {
		if m["x"] != i {
			t.Fatalf("p[%d][x]=%v, want %d", i, m["x"], i)
		}
	}
}

func TestDistinct(t *testing.T) {
	c := dialAndDrop(t, "go-mongo-test", "test")
	defer c.Conn.Close()

	for i := 0; i < 10; i++ {
		err := c.Insert(map[string]int{"x": i, "filter": i % 2})
		if err != nil {
			t.Fatal("insert", err)
		}
	}

	var r []int
	err := c.Find(nil).Distinct("x", &r)
	if err != nil {
		t.Fatal("Distinct returned error", err)
	}

	if len(r) != 10 {
		t.Fatalf("Distinct returned %d results, want 10", len(r))
	}

	r = nil
	err = c.Find(M{"filter": 1}).Distinct("x", &r)
	if err != nil {
		t.Fatal("Distinct w/ filter returned error", err)
	}

	if len(r) != 5 {
		t.Fatalf("Distinct  w/ filterreturned %d results, want 5", len(r))
	}
}

func TestFindAndModify(t *testing.T) {

	c := dialAndDrop(t, "go-mongo-test", "test")
	defer c.Conn.Close()

	var m M
	err := c.Find(M{"_id": "users"}).Upsert(M{"$inc": M{"seq": 1}}, true, &m)
	if err != nil {
		t.Fatal("upsert", err)
	}

	if m["seq"] != 1 {
		t.Fatalf("m[seq]=%v, want 1", m["seq"])
	}

	m = nil
	err = c.Find(M{"_id": "users"}).Update(M{"$inc": M{"seq": 1}}, false, &m)
	if err != nil {
		t.Fatal("update", err)
	}

	if m["seq"] != 1 {
		t.Fatalf("m[seq]=%v, want 1", m["seq"])
	}

	m = nil
	err = c.Find(M{"_id": "users"}).Remove(&m)
	if err != nil {
		t.Fatal("remove", err)
	}

	if m["seq"] != 2 {
		t.Fatalf("expect m[seq]=%v, want 2", m["seq"])
	}

	m = nil
	err = c.Find(M{"_id": "users"}).One(&m)
	if err != Done {
		t.Fatal("findone, expect EOF, got", err)
	}

	err = c.Insert(M{"x": "string"})
	if err != nil {
		t.Fatal("insert(x: string)", err)
	}

	m = nil
	err = c.Find(M{"x": "string"}).Update(M{"$inc": M{"x": 1}}, false, &m)
	if err == nil {
		t.Error("bad update did not return error")
	}
}
