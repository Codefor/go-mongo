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
	"reflect"
	"strconv"
	"sync"
	"time"
	"strings"
	"os"
)

var emptyDoc = M{}

// DateTime represents a BSON datetime. The value is in milliseconds since the
// Unix epoch.
type DateTime int64

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

func newObjectId(t int64, c uint64) ObjectId {
	b := [12]byte{
		byte(t >> 24),
		byte(t >> 16),
		byte(t >> 8),
		byte(t),
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
	return newObjectId(time.Seconds(), nextOidCounter())
}

// NewObjectIdHex returns an object id initialized from the hexadecimal
// encoding of the object id.
func NewObjectIdHex(hexString string) (ObjectId, os.Error) {
	p, err := hex.DecodeString(hexString)
	if err != nil {
		return "", err
	}
	if len(p) != 12 {
		return "", os.NewError("mongo: bad object id string len")
	}
	return ObjectId(p), nil
}

// MaxObjectIdForTime returns the maximum object id for time t in seconds from
// the epoch.
func MaxObjectIdForTime(t int64) ObjectId {
	return newObjectId(t, 0xffffffffffffffff)
}

// MinObjectIdForTime returns the minimum object id for time t in seconds from
// the epoch.
func MinObjectIdForTime(t int64) ObjectId {
	return newObjectId(t, 0)
}

// CreationTime extracts the time the object id was created in seconds since the epoch.
func (id ObjectId) CreationTime() int64 {
	if len(id) != 12 {
		return 0
	}
	return int64(id[0])<<24 + int64(id[1])<<16 + int64(id[2])<<8 + int64(id[3])
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
func (bd BSONData) Decode(v interface{}) os.Error {
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

type fieldInfo struct {
	name        string
	index       []int
	conditional bool
}

type structInfo struct {
	m      map[string]*fieldInfo
	l      []*fieldInfo
	fields D
}

func (si *structInfo) FieldInfo(name []byte) *fieldInfo {
	return si.m[string(name)]
}

func compileStructInfo(t reflect.Type, depth map[string]int, index []int, si *structInfo) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		switch {
		case f.PkgPath != "":
			// Ignore unexported fields.
		case f.Anonymous:
			// TODO: Handle pointers. Requires change to decoder and 
			// protection against infinite recursion.
			if f.Type.Kind() == reflect.Struct {
				compileStructInfo(f.Type, depth, append(index, i), si)
			}
		default:
			fi := &fieldInfo{name: f.Name}
			p := strings.Split(f.Tag.Get("bson"), "/")
			if len(p) > 0 {
				if len(p[0]) > 0 {
					fi.name = p[0]
				}
				for _, s := range p[1:] {
					switch s {
					case "c":
						fi.conditional = true
					default:
						panic(os.NewError("bson: unknown field flag " + s + " for type " + t.Name()))
					}
				}
			}
			d, found := depth[fi.name]
			if !found {
				d = 1 << 30
			}
			switch {
			case len(index) == d:
				// At same depth, remove from result.
				si.m[fi.name] = nil, false
				j := 0
				for i := 0; i < len(si.l); i++ {
					if fi.name != si.l[i].name {
						si.l[j] = si.l[i]
						j += 1
					}
				}
				si.l = si.l[:j]
			case len(index) < d:
				fi.index = make([]int, len(index)+1)
				copy(fi.index, index)
				fi.index[len(index)] = i
				depth[fi.name] = len(index)
				si.m[fi.name] = fi
				si.l = append(si.l, fi)
			}
		}
	}
}

var (
	structInfoMutex  sync.RWMutex
	structInfoCache  = make(map[reflect.Type]*structInfo)
	defaultFieldInfo = &fieldInfo{}
)

func structInfoForType(t reflect.Type) *structInfo {

	structInfoMutex.RLock()
	si, found := structInfoCache[t]
	structInfoMutex.RUnlock()
	if found {
		return si
	}

	structInfoMutex.Lock()
	defer structInfoMutex.Unlock()
	si, found = structInfoCache[t]
	if found {
		return si
	}

	si = &structInfo{m: make(map[string]*fieldInfo)}
	compileStructInfo(t, make(map[string]int), nil, si)

	hasId := false
	for _, fi := range si.l {
		if fi.name == "_id" {
			hasId = true
		} else {
			si.fields.Append(fi.name, 1)
		}
	}
	if !hasId {
		// Explicitly exclude _id because it's included by default.
		si.fields.Append("_id", 0)
	}

	structInfoCache[t] = si
	return si
}

// StructFields returns a MongoDB field specification for the given struct
// type.
func StructFields(t reflect.Type) interface{} {
	return structInfoForType(t).fields
}

type aborted struct{ err os.Error }

func abort(err os.Error) { panic(aborted{err}) }

func handleAbort(err *os.Error) {
	if r := recover(); r != nil {
		if a, ok := r.(aborted); ok {
			*err = a.err
		} else {
			panic(r)
		}
	}
}
