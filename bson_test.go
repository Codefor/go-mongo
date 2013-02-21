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

package mongo

import (
	"bytes"
	"math"
	"reflect"
	"testing"
	"time"
)

func testMap(value interface{}) map[string]interface{} {
	return map[string]interface{}{"test": value}
}

type stEmpty struct{}

type stFloat32 struct {
	Test float32 `bson:"test,omitempty"`
}

type stFloat64 struct {
	Test float64 `bson:"test,omitempty"`
}

type stString struct {
	Test string `bson:"test,omitempty"`
}

type stDoc struct {
	Test map[string]interface{} `bson:"test,omitempty"`
}

type stBinary struct {
	Test []byte `bson:"test,omitempty"`
}

type myBytes []byte

type stMyBytes struct {
	Test myBytes `bson:"test,omitempty"`
}

type stObjectId struct {
	Test ObjectId `bson:"test,omitempty"`
}

type stBool struct {
	Test bool `bson:"test,omitempty"`
}

type ncBool struct {
	Test bool `bson:"test"`
}

type stRegexp struct {
	Test Regexp `bson:"test,omitempty"`
}

type stSymbol struct {
	Test Symbol `bson:"test,omitempty"`
}

type stInt8 struct {
	Test int8 `bson:"test,omitempty"`
}

type stInt16 struct {
	Test int16 `bson:"test,omitempty"`
}

type stInt32 struct {
	Test int32 `bson:"test,omitempty"`
}

type stInt struct {
	Test int `bson:"test,omitempty"`
}

type stUint8 struct {
	Test uint8 `bson:"test,omitempty"`
}

type stUint16 struct {
	Test uint16 `bson:"test,omitempty"`
}

type stUint32 struct {
	Test uint32 `bson:"test,omitempty"`
}

type stUint64 struct {
	Test uint64 `bson:"test,omitempty"`
}

type stUint struct {
	Test uint `bson:"test,omitempty"`
}

type stInt64 struct {
	Test int64 `bson:"test,omitempty"`
}

type stTime struct {
	Test time.Time `bson:"test,omitempty"`
}

type stTimestamp struct {
	Test Timestamp `bson:"test,omitempty"`
}

type stMinMax struct {
	Test MinMax `bson:"test,omitempty"`
}

type stCodeWithScope struct {
	Test CodeWithScope `bson:"test,omitempty"`
}

type stAny struct {
	Test interface{} `bson:"test,omitempty"`
}

type stStringSlice struct {
	Test []string `bson:"test,omitempty"`
}

type stStringArray struct {
	Test [2]string `bson:"test,omitempty"`
}

type stId struct {
	Id   int `bson:"_id,omitempty"`
	Test int `bson:"test,omitempty"`
}

type stEmbed struct {
	Id int `bson:"_id,omitempty"`
	stInt32
}

var empty = map[string]interface{}{}

var bsonTests = []struct {
	// struct value
	sv interface{}
	// map value
	mv map[string]interface{}
	// decoded map value
	dmv map[string]interface{}
	// data
	data string
}{
	// Test omitempty
	{stFloat32{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stFloat64{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stString{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stAny{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stDoc{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stBinary{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stMyBytes{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stObjectId{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stBool{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stSymbol{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stInt8{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stInt16{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stInt32{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stInt64{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stInt{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stUint{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stUint8{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stUint16{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stUint32{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stUint64{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stUint{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stMinMax{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stCodeWithScope{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stRegexp{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stTimestamp{}, empty, empty, "\x05\x00\x00\x00\x00"},
	{stTime{}, empty, empty, "\x05\x00\x00\x00\x00"},

	{
		stEmpty{},
		empty,
		empty,
		"\x05\x00\x00\x00\x00",
	},
	{
		stFloat32{1.5},
		testMap(float32(1.5)),
		testMap(float64(1.5)),
		"\x13\x00\x00\x00\x01test\x00\x00\x00\x00\x00\x00\x00\xf8?\x00",
	},
	{
		stFloat64{1.5},
		testMap(float64(1.5)),
		testMap(float64(1.5)),
		"\x13\x00\x00\x00\x01test\x00\x00\x00\x00\x00\x00\x00\xf8?\x00",
	},
	{
		stString{"world"},
		testMap("world"),
		testMap("world"),
		"\x15\x00\x00\x00\x02test\x00\x06\x00\x00\x00world\x00\x00",
	},
	{
		stAny{"world"},
		testMap("world"),
		testMap("world"),
		"\x15\x00\x00\x00\x02test\x00\x06\x00\x00\x00world\x00\x00",
	},
	{
		stDoc{make(map[string]interface{})},
		testMap(make(map[string]interface{})),
		testMap(make(map[string]interface{})),
		"\x10\x00\x00\x00\x03test\x00\x05\x00\x00\x00\x00\x00",
	},
	{
		stBinary{[]byte("test")},
		testMap([]byte("test")),
		testMap([]byte("test")),
		"\x14\x00\x00\x00\x05\x74\x65\x73\x74\x00\x04\x00\x00\x00\x00\x74\x65\x73\x74\x00",
	},
	{
		stMyBytes{myBytes([]byte("test"))},
		testMap(myBytes([]byte("test"))),
		testMap([]byte("test")),
		"\x14\x00\x00\x00\x05\x74\x65\x73\x74\x00\x04\x00\x00\x00\x00\x74\x65\x73\x74\x00",
	},
	{
		stObjectId{ObjectId("\x4C\x9B\x8F\xB4\xA3\x82\xAA\xFE\x17\xC8\x6E\x63")},
		testMap(ObjectId("\x4C\x9B\x8F\xB4\xA3\x82\xAA\xFE\x17\xC8\x6E\x63")),
		testMap(ObjectId("\x4C\x9B\x8F\xB4\xA3\x82\xAA\xFE\x17\xC8\x6E\x63")),
		"\x17\x00\x00\x00\x07test\x00\x4C\x9B\x8F\xB4\xA3\x82\xAA\xFE\x17\xC8\x6E\x63\x00",
	},
	{
		stObjectId{ObjectId("")},
		empty,
		empty,
		"\x05\x00\x00\x00\x00",
	},
	{
		ncBool{true},
		testMap(true),
		testMap(true),
		"\x0C\x00\x00\x00\x08test\x00\x01\x00",
	},
	{
		ncBool{false},
		testMap(false),
		testMap(false),
		"\x0C\x00\x00\x00\x08test\x00\x00\x00",
	},
	{
		stSymbol{Symbol("aSymbol")},
		testMap(Symbol("aSymbol")),
		testMap(Symbol("aSymbol")),
		"\x17\x00\x00\x00\x0Etest\x00\x08\x00\x00\x00aSymbol\x00\x00",
	},
	{
		stInt8{10},
		testMap(int8(10)),
		testMap(int(10)),
		"\x0F\x00\x00\x00\x10test\x00\x0A\x00\x00\x00\x00",
	},
	{
		stInt16{10},
		testMap(int16(10)),
		testMap(int(10)),
		"\x0F\x00\x00\x00\x10test\x00\x0A\x00\x00\x00\x00",
	},
	{
		stInt32{10},
		testMap(int32(10)),
		testMap(int(10)),
		"\x0F\x00\x00\x00\x10test\x00\x0A\x00\x00\x00\x00",
	},
	{
		stInt64{256},
		testMap(int64(256)),
		testMap(int64(256)),
		"\x13\x00\x00\x00\x12test\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00",
	},
	{
		stInt{10},
		testMap(10),
		testMap(10),
		"\x0F\x00\x00\x00\x10test\x00\x0A\x00\x00\x00\x00",
	},
	{
		stUint8{10},
		testMap(uint8(10)),
		testMap(int(10)),
		"\x0F\x00\x00\x00\x10test\x00\x0A\x00\x00\x00\x00",
	},
	{
		stUint16{10},
		testMap(uint16(10)),
		testMap(int(10)),
		"\x0F\x00\x00\x00\x10test\x00\x0A\x00\x00\x00\x00",
	},
	{
		stUint32{256},
		testMap(uint32(256)),
		testMap(int(256)),
		"\x0f\x00\x00\x00\x10test\x00\x00\x01\x00\x00\x00",
	},
	{
		stUint32{math.MaxInt32 + 1},
		testMap(uint32(math.MaxInt32 + 1)),
		testMap(int64(math.MaxInt32 + 1)),
		"\x13\x00\x00\x00\x12test\x00\x00\x00\x00\x80\x00\x00\x00\x00\x00",
	},
	{
		stUint64{256},
		testMap(uint64(256)),
		testMap(int64(256)),
		"\x13\x00\x00\x00\x12test\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00",
	},
	{
		stUint{256},
		testMap(uint(256)),
		testMap(int(256)),
		"\x0f\x00\x00\x00\x10test\x00\x00\x01\x00\x00\x00",
	},
	{
		stUint{math.MaxInt32 + 1},
		testMap(uint(math.MaxInt32 + 1)),
		testMap(int64(math.MaxInt32 + 1)),
		"\x13\x00\x00\x00\x12test\x00\x00\x00\x00\x80\x00\x00\x00\x00\x00",
	},
	{
		stMinMax{MaxValue},
		testMap(MaxValue),
		testMap(MaxValue),
		"\x0B\x00\x00\x00\x7Ftest\x00\x00",
	},
	{
		stMinMax{MinValue},
		testMap(MinValue),
		testMap(MinValue),
		"\x0B\x00\x00\x00\xFFtest\x00\x00",
	},

	{
		stRegexp{Regexp{"a*b", "i"}},
		testMap(Regexp{"a*b", "i"}),
		nil,
		"\x11\x00\x00\x00\vtest\x00a*b\x00i\x00\x00",
	},

	{
		stCodeWithScope{CodeWithScope{"test", nil}},
		testMap(CodeWithScope{"test", nil}),
		nil,
		"\x1d\x00\x00\x00\x0ftest\x00\x12\x00\x00\x00\x05\x00\x00\x00test\x00\x05\x00\x00\x00\x00\x00",
	},

	{
		stTimestamp{1168216211000},
		testMap(Timestamp(1168216211000)),
		testMap(Timestamp(1168216211000)),
		"\x13\x00\x00\x00\x11test\x008\xbe\x1c\xff\x0f\x01\x00\x00\x00",
	},

	{
		stTime{time.Date(1, 1, 1, 1, 1, 1, 0, time.UTC)},
		testMap(time.Date(1, 1, 1, 1, 1, 1, 0, time.UTC)),
		testMap(time.Date(1, 1, 1, 1, 1, 1, 0, time.UTC)),
		"\x13\x00\x00\x00\ttest\x00\xc8\x04\x0b\xee|\xc7\xff\xff\x00",
	},

	{
		stTime{time.Date(9999, 12, 31, 23, 59, 59, 999*1e6, time.UTC)},
		testMap(time.Date(9999, 12, 31, 23, 59, 59, 999*1e6, time.UTC)),
		testMap(time.Date(9999, 12, 31, 23, 59, 59, 999*1e6, time.UTC)),
		"\x13\x00\x00\x00\ttest\x00\xff\xdb\x1f\xd2w\xe6\x00\x00\x00",
	},

	{
		stTime{time.Date(1959, 6, 25, 12, 16, 59, 123*1e6, time.UTC)},
		testMap(time.Date(1959, 6, 25, 12, 16, 59, 123*1e6, time.UTC)),
		testMap(time.Date(1959, 6, 25, 12, 16, 59, 123*1e6, time.UTC)),
		"\x13\x00\x00\x00\ttest\x00\xf3\xa6\xcb\xb3\xb2\xff\xff\xff\x00",
	},

	{
		stTime{time.Date(2011, 6, 14, 10, 47, 53, 456*1e6, time.UTC)},
		testMap(time.Date(2011, 6, 14, 10, 47, 53, 456*1e6, time.UTC)),
		testMap(time.Date(2011, 6, 14, 10, 47, 53, 456*1e6, time.UTC)),
		"\x13\x00\x00\x00\ttest\x00p\xbd\xc3\x8d0\x01\x00\x00\x00",
	},

	{
		stStringSlice{[]string{}},
		testMap([]interface{}{}),
		testMap([]interface{}{}),
		"\x10\x00\x00\x00\x04test\x00\x05\x00\x00\x00\x00\x00",
	},

	{
		stStringSlice{[]string{"hello"}},
		testMap([]interface{}{"hello"}),
		testMap([]interface{}{"hello"}),
		"\x1d\x00\x00\x00\x04test\x00\x12\x00\x00\x00\x020\x00\x06\x00\x00\x00hello\x00\x00\x00",
	},

	{
		stStringArray{[2]string{"hello", "world"}},
		testMap([]interface{}{"hello", "world"}),
		testMap([]interface{}{"hello", "world"}),
		"\x2a\x00\x00\x00\x04test\x00\x1f\x00\x00\x00\x020\x00\x06\x00\x00\x00hello\x00\x021\x00\x06\x00\x00\x00world\x00\x00\x00",
	},

	{BSONData{Kind: kindDocument, Data: []byte("\x15\x00\x00\x00\x02test\x00\x06\x00\x00\x00world\x00\x00")},
		testMap("world"),
		testMap("world"),
		"\x15\x00\x00\x00\x02test\x00\x06\x00\x00\x00world\x00\x00",
	},

	{
		stId{Test: 2, Id: 1},
		map[string]interface{}{"test": 2, "_id": 1},
		map[string]interface{}{"test": 2, "_id": 1},
		"\x18\x00\x00\x00\x10_id\x00\x01\x00\x00\x00\x10test\x00\x02\x00\x00\x00\x00",
	},

	{
		stEmbed{stInt32: stInt32{2}, Id: 1},
		map[string]interface{}{"test": 2, "_id": 1},
		map[string]interface{}{"test": 2, "_id": 1},
		"\x18\x00\x00\x00\x10_id\x00\x01\x00\x00\x00\x10test\x00\x02\x00\x00\x00\x00",
	},
}

var decodeConversionTests = []struct {
	v  interface{}
	sv interface{}
}{
	{int32(10), stInt8{10}},
	{int32(10), stInt16{10}},
	{int32(10), stInt32{10}},
	{int32(10), stInt64{10}},
	{int32(10), stInt{10}},
	{int32(10), stUint8{10}},
	{int32(10), stUint16{10}},
	{int32(10), stUint32{10}},
	{int32(10), stUint64{10}},
	{int32(10), stUint{10}},
	{int32(10), stFloat32{10}},
	{int32(10), stFloat64{10}},
	{int32(10), stBool{true}},
	{int32(0), stBool{false}},

	{int64(10), stInt8{10}},
	{int64(10), stInt16{10}},
	{int64(10), stInt32{10}},
	{int64(10), stInt64{10}},
	{int64(10), stInt{10}},
	{int64(10), stUint8{10}},
	{int64(10), stUint16{10}},
	{int64(10), stUint32{10}},
	{int64(10), stUint64{10}},
	{int64(10), stUint{10}},
	{int64(10), stFloat32{10}},
	{int64(10), stFloat64{10}},
	{int64(10), stBool{true}},
	{int64(0), stBool{false}},

	{float64(10), stInt8{10}},
	{float64(10), stInt16{10}},
	{float64(10), stInt32{10}},
	{float64(10), stInt64{10}},
	{float64(10), stInt{10}},
	{float64(10), stUint8{10}},
	{float64(10), stUint16{10}},
	{float64(10), stUint32{10}},
	{float64(10), stUint64{10}},
	{float64(10), stUint{10}},
	{float64(10), stFloat32{10}},
	{float64(10), stFloat64{10}},
	{float64(10), stBool{true}},
	{float64(0), stBool{false}},

	{time.Unix(0, int64(time.Millisecond*333)), stInt64{333}},

	{Symbol("hello"), stSymbol{"hello"}},
	{Symbol("hello"), stString{"hello"}},
}

func TestEncodeMap(t *testing.T) {
	for _, bt := range bsonTests {
		if bt.mv == nil {
			continue
		}
		var data []byte
		data, err := Encode(data, bt.mv)
		if err != nil {
			t.Errorf("Encode(%v) returned error %v", bt.mv, err)
		} else if string(data) != bt.data {
			t.Errorf("Encode(%v) = %q, want %q", bt.mv, string(data), bt.data)
		}
	}
}

func TestEncodeStruct(t *testing.T) {
	for _, bt := range bsonTests {
		var data []byte
		data, err := Encode(data, bt.sv)
		if err != nil {
			t.Errorf("Encode(%v%v) returned error %v", reflect.TypeOf(bt.sv), bt.sv, err)
		} else if string(data) != bt.data {
			t.Errorf("Encode(%v%v) = %q, want %q", reflect.TypeOf(bt.sv), bt.sv, string(data), bt.data)
		}
	}
}

func TestDecodeMap(t *testing.T) {
	for _, bt := range bsonTests {
		if bt.dmv == nil {
			continue
		}
		m := map[string]interface{}{}
		err := Decode([]byte(bt.data), m)
		if err != nil {
			t.Errorf("Decode(%q) returned error %v", bt.data, err)
		} else if !reflect.DeepEqual(bt.dmv, m) {
			t.Errorf("Decode(%q) = %q, want %q", bt.data, m, bt.dmv)
		}
	}
}

func TestDecodeMapPtr(t *testing.T) {
	for _, bt := range bsonTests {
		if bt.dmv == nil {
			continue
		}
		var m map[string]interface{}
		err := Decode([]byte(bt.data), &m)
		if err != nil {
			t.Errorf("Decode(%q) returned error %v", bt.data, err)
		} else if !reflect.DeepEqual(bt.dmv, m) {
			t.Errorf("Decode(%q) = %q, want %q", bt.data, m, bt.dmv)
		}
	}
}

func TestDecodeStruct(t *testing.T) {
	for _, bt := range bsonTests {
		if bt.dmv == nil {
			continue
		}
		psv := reflect.New(reflect.ValueOf(bt.sv).Type())
		err := Decode([]byte(bt.data), psv.Interface())
		sv := psv.Elem().Interface()
		if err != nil {
			t.Errorf("Decode(%q, &%v) returned error %v", bt.data, reflect.TypeOf(bt.sv), err)
		} else if !reflect.DeepEqual(sv, bt.sv) {
			t.Errorf("Decode(%q, &%v) = %q, want %q", bt.data, reflect.TypeOf(bt.sv), sv, bt.sv)
		}
	}
}

func TestDecodeConversions(t *testing.T) {
	for _, dt := range decodeConversionTests {
		var data []byte
		data, err := Encode(data, testMap(dt.v))
		if err != nil {
			t.Errorf("Encode(map[test:%v]) returned error %v", dt.v, err)
			continue
		}
		psv := reflect.New(reflect.ValueOf(dt.sv).Type())
		err = Decode(data, psv.Interface())
		sv := psv.Elem().Interface()
		if err != nil {
			t.Errorf("Decode(Encode(map[test:%v]), &%t) returned error %v", dt.v, reflect.TypeOf(dt.v), err)
		} else if !reflect.DeepEqual(sv, dt.sv) {
			t.Errorf("Decode(Encode(map[test:(%q)]), &%v) = %v, want %v", dt.v, reflect.TypeOf(dt.v), sv, dt.sv)
		}
	}
}

func TestEncodeOrderedMap(t *testing.T) {
	m := D{{"test", "hello world"}}
	expected := []byte("\x1b\x00\x00\x00\x02test\x00\f\x00\x00\x00hello world\x00\x00")
	var actual []byte
	actual, err := Encode(actual, m)
	if err != nil {
		t.Error("error encoding map %s", err)
	} else if !bytes.Equal(expected, actual) {
		t.Errorf("  expected %q\n  actual   %q", expected, actual)
	}
}

func TestObjectId(t *testing.T) {
	t1 := time.Now()
	min := MinObjectIdForTime(t1)
	id := NewObjectId()
	max := MaxObjectIdForTime(time.Now())
	if id < min {
		t.Errorf("%q < %q", id, min)
	}
	if id > max {
		t.Errorf("%q > %q", id, max)
	}
	if min.CreationTime().Unix() != t1.Unix() {
		t.Errorf("min.CreationTime() = %v, want %v", min.CreationTime(), t1)
	}
	id2, err := NewObjectIdHex(id.String())
	if err != nil {
		t.Errorf("NewObjectIdString returned %q", err)
	}
	if id2 != id {
		t.Errorf("%q != %q", id2, id)
	}
	t2 := ObjectId("").CreationTime()
	if !t2.IsZero() {
		t.Error("creation time for invalid id = %d, want 0", t1)
	}
}

func TestObjectIdMarshalJSON(t *testing.T) {
	jsonId := "\"4c9b8fb4a382aafe17c86e63\""
	id, _ := NewObjectIdHex(jsonId[1:25])

	jsonOutput, err := id.MarshalJSON()
	if err != nil {
		t.Errorf("MarshalJSON returned %q", err)
	}
	if !bytes.Equal(jsonOutput, []byte(jsonId)) {
		t.Errorf("id.MarshalJSON() = %v, want %v", string(jsonOutput), jsonId)
	}
}

func TestObjectIdUnmarshalJSON(t *testing.T) {
	jsonId := "\"4c9b8fb4a382aafe17c86e63\""
	id, _ := NewObjectIdHex(jsonId[1:25])

	var outputId ObjectId
	err := outputId.UnmarshalJSON([]byte(jsonId))
	if err != nil {
		t.Errorf("UnmarshalJSON returned %q", err)
	}
	if outputId != id {
		t.Errorf("id.UnmarshalJSON() = %v, want %v", outputId, id)
	}
}

func TestBadDecodeResults(t *testing.T) {
	empty := []byte("\x05\x00\x00\x00\x00")

	var m M
	err := Decode(empty, m)
	if err == nil {
		t.Error("Decode nil map did not return an error.")
	}

	err = Decode(empty, struct{}{})
	if err == nil {
		t.Error("Decode struct value did not return an error.")
	}

	var p *struct{}
	err = Decode(empty, p)
	if err == nil {
		t.Error("Decode nil pointer did not return an error.")
	}

	err = Decode(empty, 1)
	if err == nil {
		t.Error("Decode int did not return an error.")
	}

	err = Decode(empty, new(int))
	if err == nil {
		t.Error("Decode *int did not return an error.")
	}
}

var structFieldsTests = []struct {
	v interface{}
	m M
}{
	{
		struct{}{},
		M{"_id": 0},
	},
	{
		struct {
			Id   int `bson:"_id"`
			Test int
		}{},
		M{"Test": 1},
	},
}

func TestStructFields(t *testing.T) {
	for _, tt := range structFieldsTests {
		fields := StructFields(reflect.ValueOf(tt.v).Type())
		m := make(M)
		for _, di := range fields.(D) {
			m[di.Key] = di.Value
		}
		if !reflect.DeepEqual(m, tt.m) {
			t.Errorf("%+v fields=%v, want %v\n", tt.v, m, tt.m)
		}
	}
}
