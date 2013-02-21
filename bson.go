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
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

var emptyDoc = M{}

// Timestamp represents a BSON timesamp.
type Timestamp int64

// CodeWithScope represents javascript in BSON.
type CodeWithScope struct {
	Code  string
	Scope map[string]interface{}
}

// Regexp represents a BSON regular expression.
type Regexp struct {
	Pattern string
	// The valid options are:
	//	i	Case insensitive matching
	//	l	Make \w, \W, etc. locale-dependent
	//	m	Multi-line matching
	//	s	Dotall mode
	//	u	Make \w, \W, etc. match Unicode
	//	x	Verbose mode
	// Options must be specified in alphabetical order.
	Options string
}

// ObjectId represents a BSON object identifier.
type ObjectId string

// String returns the hexadecimal encoding of id. Use the function
// NewObjectIdHex to convert the string back to an object id.
func (id ObjectId) String() string {
	return hex.EncodeToString([]byte(string(id)))
}

// MarshalJSON returns the JSON encoding of id.
func (id ObjectId) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.String())
}

// UnmarshalJSON decodes id from JSON to ObjectId.
func (id *ObjectId) UnmarshalJSON(data []byte) error {
	if len(data) != 26 || data[0] != '"' || data[25] != '"' {
		return fmt.Errorf("mongo: invalid ObjectId in JSON: %q", data)
	}
	var err error
	*id, err = NewObjectIdHex(string(data[1:25]))
	return err
}

func newObjectId(t time.Time, c uint64) ObjectId {
	u := t.Unix()
	b := [12]byte{
		byte(u >> 24),
		byte(u >> 16),
		byte(u >> 8),
		byte(u),
		byte(c >> 56),
		byte(c >> 48),
		byte(c >> 40),
		byte(c >> 32),
		byte(c >> 24),
		byte(c >> 16),
		byte(c >> 8),
		byte(c)}
	return ObjectId(b[:])
}

// NewObjectId returns a new object id. This function uses the following format
// for object ids:
//
//  [0:4]  Big endian time since epoch in seconds. This is compatible
//         with other drivers.
//
//  [4:12] Incrementing counter initialized with cryptographic random number.
//          This ensures that object ids are unique, but is simpler than
//          the format used by other drivers.
func NewObjectId() ObjectId {
	return newObjectId(time.Now(), nextOidCounter())
}

// NewObjectIdHex returns an object id initialized from the hexadecimal
// encoding of the object id.
func NewObjectIdHex(hexString string) (ObjectId, error) {
	p, err := hex.DecodeString(hexString)
	if err != nil {
		return "", err
	}
	if len(p) != 12 {
		return "", errors.New("mongo: bad object id string len")
	}
	return ObjectId(p), nil
}

// MaxObjectIdForTime returns the maximum object id for time t in seconds from
// the epoch.
func MaxObjectIdForTime(t time.Time) ObjectId {
	return newObjectId(t, 0xffffffffffffffff)
}

// MinObjectIdForTime returns the minimum object id for time t in seconds from
// the epoch.
func MinObjectIdForTime(t time.Time) ObjectId {
	return newObjectId(t, 0)
}

// CreationTime extracts the time the object id was created in seconds since the epoch.
func (id ObjectId) CreationTime() time.Time {
	if len(id) != 12 {
		return time.Time{}
	}
	return time.Unix(int64(id[0])<<24+int64(id[1])<<16+int64(id[2])<<8+int64(id[3]), 0)
}

var (
	oidLock    sync.Mutex
	oidCounter uint64
)

func nextOidCounter() uint64 {
	oidLock.Lock()
	defer oidLock.Unlock()
	if oidCounter == 0 {
		if err := binary.Read(rand.Reader, binary.BigEndian, &oidCounter); err != nil {
			panic(err)
		}
	}
	oidCounter += 1
	return oidCounter
}

// BSONData represents a chunk of uninterpreted BSON data. Use this type to
// copy raw data into or out of a BSON encoding.
type BSONData struct {
	Kind int
	Data []byte
}

// Deocde decodes bd to v. See the Decode function for more information about
// BSON decoding.
func (bd BSONData) Decode(v interface{}) error {
	return decodeInternal(bd.Kind, bd.Data, v)
}

// Symbol represents a BSON symbol.
type Symbol string

// Code represents Javascript code in BSON.
type Code string

type DocItem struct {
	Key   string
	Value interface{}
}

// D represents an ordered BSON document. Use D for commands, index
// specifications and other situations where the order of the key-value pairs
// in a document is important.
type D []DocItem

// Append adds an item to the document..
func (d *D) Append(name string, value interface{}) {
	*d = append(*d, DocItem{name, value})
}

// M is a shortcut for writing map[string]interface{} in BSON literal
// expressions. The type M is encoded the same as the type
// map[string]interface{}.
type M map[string]interface{}

// A is a shortcut for writing []interface{} in BSON literal expressions. The
// type A is encoded the same as the type []interface{}.
type A []interface{}

// MinMax represents either a minimum or maximum BSON value.
type MinMax int

const (
	// MaxValue is the maximum BSON value.
	MaxValue MinMax = 1
	// MinValue is the minimum BSON value.
	MinValue MinMax = -1
)

const (
	kindFloat         = 0x1
	kindString        = 0x2
	kindDocument      = 0x3
	kindArray         = 0x4
	kindBinary        = 0x5
	kindObjectId      = 0x7
	kindBool          = 0x8
	kindDateTime      = 0x9
	kindNull          = 0xA
	kindRegexp        = 0xB
	kindCode          = 0xD
	kindSymbol        = 0xE
	kindCodeWithScope = 0xF
	kindInt32         = 0x10
	kindTimestamp     = 0x11
	kindInt64         = 0x12
	kindMinValue      = 0xff
	kindMaxValue      = 0x7f
)

var kindNames = map[int]string{
	kindFloat:         "float",
	kindString:        "string",
	kindDocument:      "document",
	kindArray:         "array",
	kindBinary:        "binary",
	kindObjectId:      "objectId",
	kindBool:          "bool",
	kindDateTime:      "dateTime",
	kindNull:          "null",
	kindRegexp:        "regexp",
	kindCode:          "code",
	kindSymbol:        "symbol",
	kindCodeWithScope: "codeWithScope",
	kindInt32:         "int32",
	kindTimestamp:     "timestamp",
	kindInt64:         "int64",
	kindMinValue:      "minValue",
	kindMaxValue:      "maxValue",
}

func kindName(kind int) string {
	name, ok := kindNames[kind]
	if !ok {
		name = strconv.Itoa(kind)
	}
	return name
}

type fieldSpec struct {
	name      string
	index     []int
	omitEmpty bool
}

type structSpec struct {
	m      map[string]*fieldSpec
	l      []*fieldSpec
	fields D
}

func (ss *structSpec) fieldSpec(name []byte) *fieldSpec {
	return ss.m[string(name)]
}

func compileStructSpec(t reflect.Type, depth map[string]int, index []int, ss *structSpec) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		switch {
		case f.PkgPath != "":
			// Ignore unexported fields.
		case f.Anonymous:
			// TODO: Handle pointers. Requires change to decoder and
			// protection against infinite recursion.
			if f.Type.Kind() == reflect.Struct {
				compileStructSpec(f.Type, depth, append(index, i), ss)
			}
		default:
			fs := &fieldSpec{name: f.Name}
			tag := f.Tag.Get("bson")
			if strings.Contains(tag, "/c") {
				panic("use ,omitempty instead of /c in bson field tag")
			}
			p := strings.Split(tag, ",")
			if len(p) > 0 && p[0] != "-" {
				if len(p[0]) > 0 {
					fs.name = p[0]
				}
				for _, s := range p[1:] {
					switch s {
					case "omitempty":
						fs.omitEmpty = true
					default:
						panic(errors.New("bson: unknown field flag " + s + " for type " + t.Name()))
					}
				}
			}
			d, found := depth[fs.name]
			if !found {
				d = 1 << 30
			}
			switch {
			case len(index) == d:
				// At same depth, remove from result.
				delete(ss.m, fs.name)
				j := 0
				for i := 0; i < len(ss.l); i++ {
					if fs.name != ss.l[i].name {
						ss.l[j] = ss.l[i]
						j += 1
					}
				}
				ss.l = ss.l[:j]
			case len(index) < d:
				fs.index = make([]int, len(index)+1)
				copy(fs.index, index)
				fs.index[len(index)] = i
				depth[fs.name] = len(index)
				ss.m[fs.name] = fs
				ss.l = append(ss.l, fs)
			}
		}
	}
}

var (
	structSpecMutex  sync.RWMutex
	structSpecCache  = make(map[reflect.Type]*structSpec)
	defaultFieldSpec = &fieldSpec{}
)

func structSpecForType(t reflect.Type) *structSpec {

	structSpecMutex.RLock()
	ss, found := structSpecCache[t]
	structSpecMutex.RUnlock()
	if found {
		return ss
	}

	structSpecMutex.Lock()
	defer structSpecMutex.Unlock()
	ss, found = structSpecCache[t]
	if found {
		return ss
	}

	ss = &structSpec{m: make(map[string]*fieldSpec)}
	compileStructSpec(t, make(map[string]int), nil, ss)

	hasId := false
	for _, fs := range ss.l {
		if fs.name == "_id" {
			hasId = true
		} else {
			ss.fields.Append(fs.name, 1)
		}
	}
	if !hasId {
		// Explicitly exclude _id because it's included by default.
		ss.fields.Append("_id", 0)
	}

	structSpecCache[t] = ss
	return ss
}

// StructFields returns a MongoDB field specification for the given struct
// type.
func StructFields(t reflect.Type) interface{} {
	return structSpecForType(t).fields
}

type aborted struct{ err error }

func abort(err error) { panic(aborted{err}) }

func handleAbort(err *error) {
	if r := recover(); r != nil {
		if a, ok := r.(aborted); ok {
			*err = a.err
		} else {
			panic(r)
		}
	}
}

func msFromTime(t time.Time) int64 {
	return int64(t.Unix())*1e3 + int64(t.Nanosecond())/1e6
}

func timeFromMS(ms int64) time.Time {
	return time.Unix(ms/1e3, (ms%1e3)*1e6).In(time.UTC)
}
