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

import "testing"

func dialAndDrop(t *testing.T, dbname, collectionName string) Collection {
	c, err := Dial("127.0.0.1")
	if err != nil {
		t.Fatal("dial", err)
	}
	db := Database{c, dbname, DefaultLastErrorCmd}
	err = db.Run(D{{"drop", collectionName}}, nil)
	if err != nil && err.Error() != "ns not found" {
		db.Conn.Close()
		t.Fatal("drop", err)
	}
	return db.C(collectionName)
}

var findOptionsTests = []struct {
	limit         int
	batchSize     int
	exhaust       bool
	expectedCount int
}{
	{0, 0, false, 200},
	{0, 1, false, 200},
	{0, 2, false, 200},
	{0, 3, false, 200},
	{0, 100, false, 200},
	{0, 500, false, 200},

	{1, 0, false, 1},
	{1, 1, false, 1},
	{1, 2, false, 1},
	{1, 3, false, 1},
	{1, 100, false, 1},
	{1, 500, false, 1},

	{10, 0, false, 10},
	{10, 1, false, 10},
	{10, 2, false, 10},
	{10, 3, false, 10},
	{10, 100, false, 10},
	{10, 500, false, 10},

	{200, 3, false, 200},
	{200, 3, true, 200},
	{0, 3, true, 200},
}

func TestFindOptions(t *testing.T) {
	c := dialAndDrop(t, "go-mongo-test", "test")
	defer c.Conn.Close()

	c.CreateIndex(D{{"x", 1}}, nil)

	for i := 0; i < 200; i++ {
		err := c.Insert(map[string]int{"x": i})
		if err != nil {
			t.Fatal("insert", err)
		}
	}

	for _, tt := range findOptionsTests {
		r, err := c.Find(nil).
			Limit(tt.limit).
			BatchSize(tt.batchSize).
			Sort(D{{"x", 1}}).
			Exhaust(tt.exhaust).
			Cursor()
		if err != nil {
			t.Error("find", err)
			continue
		}
		count := 0
		for r.HasNext() {
			var m M
			err = r.Next(&m)
			if err != nil {
				t.Errorf("%+v, count=%d, next errror %v", tt, count, err)
				break
			}
			if m["x"] != count {
				t.Errorf("%+v, x=%d, want %d", tt, m["x"], count)
				break
			}
			count += 1
		}
		if count != tt.expectedCount {
			t.Errorf("%+v, count=%d, want %d", tt, count, tt.expectedCount)
		}
		r.Close()
	}
}

var twoCursorTests = [][]int{
	// 0: r[0].Next()
	// 1: r[1].Next()
	// 2: r[0].Close
	// 3: r[1].Close
	{0, 0, 0, 0, 0, 0},
	{0, 1, 0, 1, 0, 1},
	{1, 0, 1, 0, 1, 0},
	{1, 1, 0, 0, 1, 1},
	{0, 0, 1, 1, 0, 0},
	{0, 0, 0, 1, 1, 1, 0, 0, 0},
	{1, 1, 1, 0, 0, 0, 1, 1, 1},
	{2, 1, 1, 1, 1, 1, 1},
	{3, 0, 0, 0, 0, 0, 0},
	{0, 2, 1, 1, 1, 1, 1},
	{1, 3, 0, 0, 0, 0, 0},
}

func TestTwoCursors(t *testing.T) {
	c := dialAndDrop(t, "go-mongo-test", "test")
	defer c.Conn.Close()

	c.CreateIndex(D{{"x", 1}}, nil)

	for i := 0; i < 2; i++ {
		for j := 0; j < 10; j++ {
			err := c.Insert(map[string]int{"x": j, "r": i})
			if err != nil {
				t.Fatal("insert", err)
			}
		}
	}

Tests:
	for _, tt := range twoCursorTests {

		var r [2]Cursor

		for i := 0; i < 2; i++ {
			var err error
			r[i], err = c.Find(M{"r": i}).Sort(D{{"x", 1}}).BatchSize(2).Cursor()
			if err != nil {
				t.Fatal("find", err)
			}
		}

		var count [2]int

		for j, i := range tt {
			var m M
			if i >= 2 {
				r[i-2].Close()
				continue
			}
			err := r[i].Next(&m)
			if err != nil {
				t.Errorf("%v[%d] r[%d].Next() = %v", tt, j, i, err)
				continue Tests
			}
			if m["r"] != i {
				t.Errorf("%v[%d] m[r]=%v, want %d", tt, j, m["r"], i)
				continue Tests
			}
			if m["x"] != count[i] {
				t.Errorf("%v[%d] m[x]=%v, want %d", tt, j, m["x"], count[i])
				continue Tests
			}
			count[i] += 1
		}
		for i := 0; i < 2; i++ {
			r[i].Close()
		}
	}
}

func TestTailableCursor(t *testing.T) {
	c := dialAndDrop(t, "go-mongo-test", "capped")
	defer c.Conn.Close()

	err := c.Db().Run(
		D{{"create", c.Name()},
			{"capped", true},
			{"size", 1000.0}},
		nil)
	if err != nil {
		t.Fatal("create capped", err)
	}

	var r Cursor
	for n := 1; n < 4; n++ {
		for i := 0; i < n; i++ {
			err = c.Insert(M{"x": i})
			if err != nil {
				t.Fatal("insert", i, err)
			}
		}

		if r == nil {
			r, err = c.Find(nil).Tailable(true).Cursor()
			if err != nil {
				t.Fatal("find", err)
			}
			defer r.Close()
		}

		i := 0
		for r.HasNext() {
			var m M
			err = r.Next(&m)
			if err != nil {
				t.Fatal("next", n, i, err)
			}
			if m["x"] != i {
				t.Fatal("expect", i, "actual", m["x"])
			}
			i += 1
		}
		if i != n {
			t.Fatal("count: expect", n, "actual", i)
		}
	}
}

func TestCursorCloseAfterConnectionClose(t *testing.T) {

	c := dialAndDrop(t, "go-mongo-test", "test")
	defer c.Conn.Close()

	id := NewObjectId()
	err := c.Insert(M{"_id": id, "x": 1})
	if err != nil {
		t.Fatal("insert", err)
	}

	r, err := c.Find(nil).Cursor()
	if err != nil {
		t.Fatal("find", err)
	}

	var m M

	c.Conn.Close()
	r.HasNext()
	r.Next(&m)
	r.Close()
	r.Next(&m)
}
