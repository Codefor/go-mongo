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
	"errors"
	"math"
	"reflect"
	"strconv"
	"time"
)

var (
	typeD        = reflect.TypeOf(D{})
	typeBSONData = reflect.TypeOf(BSONData{})
	idKey        = reflect.ValueOf("_id")
	itoas        = [...]string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}
)

// EncodeTypeError is the error indicating that Encode could not encode an input type.
type EncodeTypeError struct {
	Type reflect.Type
}

func (e *EncodeTypeError) Error() string {
	return "bson: unsupported type: " + e.Type.String()
}

type encodeState struct {
	buffer
}

// Encode appends the BSON encoding of doc to buf and returns the new slice.
//
// Encode traverses the value doc recursively using the following
// type-dependent encodings:
//
// Struct values encode as BSON documents. Each exported struct field is
// written as a document element subject to comma separated options in the
// "bson" struct field tag. The first option specifies the name of the document
// element. If the name is "-", then the field is ignored. If the name is "",
// then the name of the struct field is used as the name of the document
// element. The remaining options are:
//
//  omitempty   If the field is the zero value, then the field is not
//              written to the encoding.
//
// Anonymous struct fields are encoded in-line with the containing struct.
//
// Array and slice values encode as BSON arrays.
//
// Map values encode as BSON documents. The map's key type must be string; the
// object keys are used directly as map keys.
//
// Pointer values encode as the value pointed to.
//
// Interface values encode as the value contained in the interface.
//
// Other types are encoded as follows
//
//      Go                  -> BSON
//      bool                -> Boolean
//      float32             -> Double
//      float64             -> Double
//      int, uint, uint32   -> Integer32 if value fits in int32, else Integer64
//      int8, int16, int32  -> Integer32
//      uint8, uint16       -> Integer32
//      int64, uint64       -> Integer64
//      string              -> String
//      []byte              -> Binary data
//      time.Time           -> UTC Datetime
//      mongo.Code          -> Javascript code
//      mongo.CodeWithScope -> Javascript code with scope
//      mongo.D             -> Document. Use when element order is important.
//      mongo.MinMax        -> Minimum / Maximum value
//      mongo.ObjectId      -> ObjectId
//      mongo.Regexp        -> Regular expression
//      mongo.Symbol        -> Symbol
//      mongo.Timestamp     -> Timestamp
//
// Other types including channels, complex and function values cannot be encoded.
//
// BSON cannot represent cyclic data structure and Encode does not handle them.
// Passing cyclic structures to Encode will result in an infinite recursion.
func Encode(buf []byte, doc interface{}) (result []byte, err error) {
	defer handleAbort(&err)

	v := reflect.ValueOf(doc)
	if kind := v.Kind(); kind == reflect.Interface || kind == reflect.Ptr {
		v = v.Elem()
	}

	e := encodeState{buffer: buf}
	switch v.Type() {
	case typeD:
		e.writeD(v.Interface().(D))
	case typeBSONData:
		bd := v.Interface().(BSONData)
		if bd.Kind != kindDocument {
			return nil, &EncodeTypeError{v.Type()}
		}
		e.Write(bd.Data)
	default:
		switch v.Kind() {
		case reflect.Struct:
			e.writeStruct(v)
		case reflect.Map:
			e.writeMap(v, true)
		default:
			return nil, &EncodeTypeError{v.Type()}
		}
	}
	return e.buffer, nil
}

func (e *encodeState) beginDoc() (offset int) {
	offset = len(e.buffer)
	e.buffer.Next(4)
	return
}

func (e *encodeState) endDoc(offset int) {
	n := len(e.buffer) - offset
	wire.PutUint32(e.buffer[offset:offset+4], uint32(n))
}

func (e *encodeState) writeKindName(kind int, name string) {
	e.WriteByte(byte(kind))
	e.WriteCString(name)
}

func (e *encodeState) writeStruct(v reflect.Value) {
	offset := e.beginDoc()
	ss := structSpecForType(v.Type())
	for _, fs := range ss.l {
		e.encodeValue(fs.name, fs, v.FieldByIndex(fs.index))
	}
	e.WriteByte(0)
	e.endDoc(offset)
}

func (e *encodeState) writeMap(v reflect.Value, topLevel bool) {
	if v.IsNil() {
		return
	}
	if v.Type().Key().Kind() != reflect.String {
		abort(&EncodeTypeError{v.Type()})
	}
	offset := e.beginDoc()
	skipId := false
	if topLevel {
		idValue := v.MapIndex(idKey)
		if idValue.IsValid() {
			skipId = true
			e.encodeValue("_id", defaultFieldSpec, idValue)
		}
	}
	for _, k := range v.MapKeys() {
		sk := k.String()
		if !skipId || sk != "_id" {
			e.encodeValue(sk, defaultFieldSpec, v.MapIndex(k))
		}
	}
	e.WriteByte(0)
	e.endDoc(offset)
}

func (e *encodeState) writeD(v D) {
	offset := e.beginDoc()
	for _, kv := range v {
		e.encodeValue(kv.Key, defaultFieldSpec, reflect.ValueOf(kv.Value))
	}
	e.WriteByte(0)
	e.endDoc(offset)
}

func (e *encodeState) encodeValue(name string, fs *fieldSpec, v reflect.Value) {
	if !v.IsValid() {
		return
	}
	t := v.Type()
	encoder, found := typeEncoder[t]
	if !found {
		encoder, found = kindEncoder[t.Kind()]
		if !found {
			abort(&EncodeTypeError{t})
		}
	}
	encoder(e, name, fs, v)
}

func encodeBool(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	b := v.Bool()
	if b == false && fs.omitEmpty {
		return
	}
	e.writeKindName(kindBool, name)
	if b {
		e.WriteByte(1)
	} else {
		e.WriteByte(0)
	}
}

func encodeInt(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	i := v.Int()
	if i == 0 && fs.omitEmpty {
		return
	}
	if i >= math.MinInt32 && i <= math.MaxInt32 {
		e.writeKindName(kindInt32, name)
		e.WriteUint32(uint32(i))
	} else {
		e.writeKindName(kindInt64, name)
		e.WriteUint64(uint64(i))
	}
}

func encodeUint16(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	u := v.Uint()
	if u == 0 && fs.omitEmpty {
		return
	}
	e.writeKindName(kindInt32, name)
	e.WriteUint32(uint32(u))
}

func encodeUint(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	u := v.Uint()
	if u == 0 && fs.omitEmpty {
		return
	}
	if int64(u) < 0 {
		abort(errors.New("bson: uint value does not fit in int64"))
	}
	if u <= math.MaxInt32 {
		e.writeKindName(kindInt32, name)
		e.WriteUint32(uint32(u))
	} else {
		e.writeKindName(kindInt64, name)
		e.WriteUint64(uint64(u))
	}
}

func encodeInt32(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	i := v.Int()
	if i == 0 && fs.omitEmpty {
		return
	}
	e.writeKindName(kindInt32, name)
	e.WriteUint32(uint32(i))
}

func encodeInt64(e *encodeState, kind int, name string, fs *fieldSpec, v reflect.Value) {
	i := v.Int()
	if i == 0 && fs.omitEmpty {
		return
	}
	e.writeKindName(kind, name)
	e.WriteUint64(uint64(i))
}

func encodeUint64(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	u := v.Uint()
	if u == 0 && fs.omitEmpty {
		return
	}
	if int64(u) < 0 {
		abort(errors.New("bson: uint64 value does not fit in int64"))
	}
	e.writeKindName(kindInt64, name)
	e.WriteUint64(u)
}

func encodeFloat(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	f := v.Float()
	if f == 0 && fs.omitEmpty {
		return
	}
	e.writeKindName(kindFloat, name)
	e.WriteUint64(math.Float64bits(f))
}

func encodeString(e *encodeState, kind int, name string, fs *fieldSpec, v reflect.Value) {
	s := v.String()
	if s == "" && fs.omitEmpty {
		return
	}
	e.writeKindName(kind, name)
	e.WriteUint32(uint32(len(s) + 1))
	e.WriteCString(s)
}

func encodeRegexp(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	r := v.Interface().(Regexp)
	if r.Pattern == "" && fs.omitEmpty {
		return
	}
	e.writeKindName(kindRegexp, name)
	e.WriteCString(r.Pattern)
	e.WriteCString(r.Options)
}

func encodeObjectId(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	oid := v.Interface().(ObjectId)
	if oid == "" {
		return
	}
	if len(oid) != 12 {
		abort(errors.New("bson: object id length != 12"))
	}
	e.writeKindName(kindObjectId, name)
	copy(e.Next(12), oid)
}

func encodeBSONData(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	bd := v.Interface().(BSONData)
	if bd.Kind == 0 {
		return
	}
	e.writeKindName(bd.Kind, name)
	e.Write(bd.Data)
}

func encodeCodeWithScope(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	c := v.Interface().(CodeWithScope)
	if c.Code == "" && c.Scope == nil && fs.omitEmpty {
		return
	}
	e.writeKindName(kindCodeWithScope, name)
	offset := e.beginDoc()
	e.WriteUint32(uint32(len(c.Code) + 1))
	e.WriteCString(c.Code)
	scopeOffset := e.beginDoc()
	for k, v := range c.Scope {
		e.encodeValue(k, defaultFieldSpec, reflect.ValueOf(v))
	}
	e.WriteByte(0)
	e.endDoc(scopeOffset)
	e.endDoc(offset)
}

func encodeMinMax(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	i := v.Interface().(MinMax)
	if i == 0 && fs.omitEmpty {
		return
	}
	switch v.Interface().(MinMax) {
	case 1:
		e.writeKindName(kindMaxValue, name)
	case -1:
		e.writeKindName(kindMinValue, name)
	default:
		abort(errors.New("bson: unknown MinMax value"))
	}
}

func encodeTime(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	t := v.Interface().(time.Time)
	if t.IsZero() && fs.omitEmpty {
		return
	}
	e.writeKindName(kindDateTime, name)
	e.WriteUint64(uint64(msFromTime(t)))
}

func encodeStruct(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	e.writeKindName(kindDocument, name)
	e.writeStruct(v)
}

func encodeMap(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	if v.IsNil() {
		return
	}
	e.writeKindName(kindDocument, name)
	e.writeMap(v, false)
}

func encodeD(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	d := v.Interface().(D)
	if d == nil {
		return
	}
	e.writeKindName(kindDocument, name)
	e.writeD(d)
}

func encodeByteSlice(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	b := v.Bytes()
	if b == nil {
		return
	}
	e.writeKindName(kindBinary, name)
	e.WriteUint32(uint32(len(b)))
	e.WriteByte(0)
	e.Write(b)
}

func encodeSlice(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	if v.IsNil() {
		return
	}
	if v.Type().Elem().Kind() == reflect.Uint8 {
		encodeByteSlice(e, name, fs, v)
		return
	}
	encodeArray(e, name, fs, v)
}

func encodeArray(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	e.writeKindName(kindArray, name)
	offset := e.beginDoc()
	n := v.Len()
	if n < len(itoas) {
		for i, k := range itoas[:n] {
			e.encodeValue(k, defaultFieldSpec, v.Index(i))
		}
	} else {
		for i := 0; i < n; i++ {
			e.encodeValue(strconv.Itoa(i), defaultFieldSpec, v.Index(i))
		}
	}
	e.WriteByte(0)
	e.endDoc(offset)
}

func encodeInterfaceOrPtr(e *encodeState, name string, fs *fieldSpec, v reflect.Value) {
	if v.IsNil() {
		return
	} else {
		e.encodeValue(name, defaultFieldSpec, v.Elem())
	}
}

type encoderFunc func(e *encodeState, name string, fs *fieldSpec, v reflect.Value)

var kindEncoder map[reflect.Kind]encoderFunc
var typeEncoder map[reflect.Type]encoderFunc

func init() {
	kindEncoder = map[reflect.Kind]encoderFunc{
		reflect.Array:   encodeArray,
		reflect.Bool:    encodeBool,
		reflect.Float32: encodeFloat,
		reflect.Float64: encodeFloat,
		reflect.Int8:    encodeInt32,
		reflect.Int16:   encodeInt32,
		reflect.Int32:   encodeInt32,
		reflect.Int:     encodeInt,
		reflect.Uint8:   encodeUint16,
		reflect.Uint16:  encodeUint16,
		reflect.Uint32:  encodeUint,
		reflect.Uint64:  encodeUint64,
		reflect.Uint:    encodeUint,
		reflect.Int64: func(e *encodeState, name string, fs *fieldSpec, value reflect.Value) {
			encodeInt64(e, kindInt64, name, fs, value)
		},
		reflect.Interface: encodeInterfaceOrPtr,
		reflect.Map:       encodeMap,
		reflect.Ptr:       encodeInterfaceOrPtr,
		reflect.Slice:     encodeSlice,
		reflect.String: func(e *encodeState, name string, fs *fieldSpec, value reflect.Value) {
			encodeString(e, kindString, name, fs, value)
		},
		reflect.Struct: encodeStruct,
	}
	typeEncoder = map[reflect.Type]encoderFunc{
		typeD:        encodeD,
		typeBSONData: encodeBSONData,
		reflect.TypeOf(Code("")): func(e *encodeState, name string, fs *fieldSpec, value reflect.Value) {
			encodeString(e, kindCode, name, fs, value)
		},
		reflect.TypeOf(CodeWithScope{}): encodeCodeWithScope,
		reflect.TypeOf(time.Time{}):     encodeTime,
		reflect.TypeOf(MinMax(0)):       encodeMinMax,
		reflect.TypeOf(ObjectId("")):    encodeObjectId,
		reflect.TypeOf(Regexp{}):        encodeRegexp,
		reflect.TypeOf(Symbol("")): func(e *encodeState, name string, fs *fieldSpec, value reflect.Value) {
			encodeString(e, kindSymbol, name, fs, value)
		},
		reflect.TypeOf(Timestamp(0)): func(e *encodeState, name string, fs *fieldSpec, value reflect.Value) {
			encodeInt64(e, kindTimestamp, name, fs, value)
		},
	}
}
