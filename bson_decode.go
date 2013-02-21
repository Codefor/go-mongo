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
	"time"
)

var ErrEOD = errors.New("bson: unexpected end of data when parsing BSON")

// DecodeConvertError is returned when decoder cannot convert BSON value to the
// target type.
type DecodeConvertError struct {
	kind int
	t    reflect.Type
}

func (e *DecodeConvertError) Error() string {
	return "bson: could not decode " + kindName(e.kind) + " to " + e.t.String()
}

// DecodeTypeError is returned when the decoder encounters an unknown type in
// the input.
type DecodeTypeError struct {
	kind int
}

func (e *DecodeTypeError) Error() string {
	return "bson: could not decode " + kindName(e.kind)
}

// Deocde decodes BSON data to value v.
//
// Decode traverses the value v recursively. Decode uses the inverse of the
// encodings supported by Encode, allocating maps, slices and pointers as
// needed. The following conversions from BSON types to GO types are supported:
//
//      BSON                -> Go
//      Integer32           -> signed and unsigned integers, floats, bool
//      Integer64           -> signed and unsigned integers, floats, bool
//      Array               -> []interface{}, other slice types
//      Binary              -> []byte
//      Boolean             -> bool
//      Datetime            -> time.Time, int64
//      Document            -> map[string]interface{}, struct types
//      Double              -> signed and unsigned integers, floats, bool
//      MinValue, MaxValue  -> mongo.MinMax
//      ObjectID            -> mongo.ObjectId
//      Symbol              -> mongo.Symbol, string
//      Timestamp           -> mongo.Timestamp, int64
//      string              -> string
//
// If a number overflows the target type or the BSON value cannot be converted
// to the target type, then the decoding completes the best it can and an error
// is returned.
//
// To decode a BSON value into a nil interface value, the first type listed in
// the right hand column of the table above is used.
func Decode(data []byte, v interface{}) (err error) {
	return decodeInternal(kindDocument, data, v)
}

// decodeInternal decodes BSON data with given kind to v.
func decodeInternal(kind int, data []byte, v interface{}) (err error) {
	defer handleAbort(&err)
	value, ok := v.(reflect.Value)
	if !ok {
		value = reflect.ValueOf(v)
		switch value.Kind() {
		case reflect.Map:
			if value.IsNil() {
				return errors.New("bson: Decode map arg must not be nil.")
			}
		case reflect.Ptr:
			if value.IsNil() {
				return errors.New("bson: Decode pointer arg must not be nil.")
			}
			value = value.Elem()
		default:
			return errors.New("bson: Decode arg must be pointer or map.")
		}
	}

	d := decodeState{data: data}
	d.decodeValue(kind, value)
	return d.savedError
}

// decodeState represents the state while decoding a JSON value.
type decodeState struct {
	data       []byte
	offset     int // read offset in data
	savedError error
}

// saveError saves the first err it is called with, for reporting at the end of
// Decode.
func (d *decodeState) saveError(err error) {
	if d.savedError == nil {
		d.savedError = err
	}
}

// saveErrorAndSkip skips the value and saves a conversion error.
func (d *decodeState) saveErrorAndSkip(kind int, t reflect.Type) {
	d.skipValue(kind)
	if d.savedError == nil {
		d.savedError = &DecodeConvertError{kind, t}
	}
}

func (d *decodeState) beginDoc() int {
	offset := d.offset
	offset += int(wire.Uint32(d.scanSlice(4)))
	return offset
}

func (d *decodeState) endDoc(offset int) {
	if d.offset != offset {
		abort(errors.New("bson: doc length wrong"))
	}
}

func (d *decodeState) scanByte() byte {
	if d.offset >= len(d.data) {
		abort(ErrEOD)
	}
	b := d.data[d.offset]
	d.offset += 1
	return b
}

func (d *decodeState) scanSlice(n int) []byte {
	offset := d.offset + n
	if offset > len(d.data) {
		abort(ErrEOD)
	}
	p := d.data[d.offset:offset]
	d.offset = offset
	return p
}

func (d *decodeState) scanKindName() (int, []byte) {
	kind := int(d.scanByte())
	if kind == 0 {
		return 0, nil
	}
	for i, b := range d.data[d.offset:] {
		if b == 0 {
			name := d.data[d.offset : d.offset+i]
			d.offset += i + 1
			return kind, name
		}
	}
	abort(ErrEOD)
	panic("unreachable")
}

func (d *decodeState) scanFloat() float64 {
	return math.Float64frombits(wire.Uint64(d.scanSlice(8)))
}

func (d *decodeState) scanString() string {
	n := int(wire.Uint32(d.scanSlice(4)))
	s := string(d.scanSlice(n - 1))
	d.offset += 1 // skip null terminator
	return s
}

func (d *decodeState) scanBinary() ([]byte, int) {
	n := int(wire.Uint32(d.scanSlice(4)))
	subtype := int(d.scanByte())
	return d.scanSlice(n), subtype
}

func (d *decodeState) scanBool() bool {
	b := d.scanByte()
	if b == 0 {
		return false
	}
	return true
}

func (d *decodeState) scanInt32() int32 {
	return int32(wire.Uint32(d.scanSlice(4)))
}

func (d *decodeState) scanInt64() int64 {
	return int64(wire.Uint64(d.scanSlice(8)))
}

func (d *decodeState) decodeValue(kind int, v reflect.Value) {
	v = d.indirect(v)
	t := v.Type()
	decoder, ok := typeDecoder[t]
	if !ok {
		decoder, ok = kindDecoder[t.Kind()]
		if !ok {
			d.saveErrorAndSkip(kind, v.Type())
			return
		}
	}
	decoder(d, kind, v)
}

// indirect walks down v allocating pointers as needed, until it gets to a
// non-pointer.
func (d *decodeState) indirect(v reflect.Value) reflect.Value {
	for {
		if v.Kind() == reflect.Interface && !v.IsNil() {
			v = v.Elem()
			continue
		}
		if v.Kind() != reflect.Ptr {
			break
		}
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	return v
}

func decodeFloat(d *decodeState, kind int, v reflect.Value) {
	var f float64
	switch kind {
	default:
		d.saveErrorAndSkip(kind, v.Type())
		return
	case kindFloat:
		f = d.scanFloat()
	case kindInt64:
		f = float64(d.scanInt64())
	case kindInt32:
		f = float64(d.scanInt32())
	}
	if v.OverflowFloat(f) {
		d.saveError(&DecodeConvertError{kind, v.Type()})
		return
	}
	v.SetFloat(f)
}

func decodeInt(d *decodeState, kind int, v reflect.Value) {
	var n int64
	switch kind {
	default:
		d.saveErrorAndSkip(kind, v.Type())
		return
	case kindInt64, kindTimestamp, kindDateTime:
		n = d.scanInt64()
	case kindInt32:
		n = int64(d.scanInt32())
	case kindFloat:
		n = int64(d.scanFloat())
	}
	if v.OverflowInt(n) {
		d.saveError(&DecodeConvertError{kind, v.Type()})
		return
	}
	v.SetInt(n)
}

func decodeUint(d *decodeState, kind int, v reflect.Value) {
	var n uint64
	switch kind {
	default:
		d.saveErrorAndSkip(kind, v.Type())
		return
	case kindInt64, kindTimestamp, kindDateTime:
		n = uint64(d.scanInt64())
	case kindInt32:
		n = uint64(d.scanInt32())
	case kindFloat:
		n = uint64(d.scanFloat())
	}
	if v.OverflowUint(n) {
		d.saveError(&DecodeConvertError{kind, v.Type()})
		return
	}
	v.SetUint(n)
}

func decodeTimestamp(d *decodeState, kind int, v reflect.Value) {
	switch kind {
	default:
		d.saveErrorAndSkip(kind, v.Type())
		return
	case kindInt64, kindTimestamp:
		decodeInt(d, kindInt64, v)
	}
}

func decodeTime(d *decodeState, kind int, v reflect.Value) {
	switch kind {
	default:
		d.saveErrorAndSkip(kind, v.Type())
		return
	case kindDateTime:
		v.Set(reflect.ValueOf(timeFromMS(d.scanInt64())))
	}
}

func decodeString(d *decodeState, kind int, v reflect.Value) {
	var s string
	switch kind {
	default:
		d.saveErrorAndSkip(kind, v.Type())
		return
	case kindString, kindSymbol, kindCode:
		s = d.scanString()
	}
	v.SetString(s)
}

func decodeObjectId(d *decodeState, kind int, v reflect.Value) {
	var p []byte
	switch kind {
	default:
		d.saveErrorAndSkip(kind, v.Type())
		return
	case kindObjectId:
		p = d.scanSlice(12)
	}
	v.SetString(string(p))
}

func decodeBSONData(d *decodeState, kind int, v reflect.Value) {
	start := d.offset
	d.skipValue(kind)
	bd := BSONData{Kind: kind, Data: make([]byte, d.offset-start)}
	copy(bd.Data, d.data[start:d.offset])
	v.Set(reflect.ValueOf(bd))
}

func decodeByteSlice(d *decodeState, kind int, v reflect.Value) {
	var p []byte
	switch kind {
	default:
		d.saveErrorAndSkip(kind, v.Type())
		return
	case kindBinary:
		p, _ = d.scanBinary()
	}
	if v.IsNil() || v.Cap() < len(p) {
		v.Set(reflect.MakeSlice(v.Type(), len(p), len(p)))
	} else {
		v.SetLen(len(p))
	}
	reflect.Copy(v, reflect.ValueOf(p))
}

func decodeBool(d *decodeState, kind int, v reflect.Value) {
	var b bool
	switch kind {
	default:
		d.saveErrorAndSkip(kind, v.Type())
		return
	case kindBool:
		b = d.scanBool()
	case kindInt32:
		b = d.scanInt32() != 0
	case kindInt64:
		b = d.scanInt64() != 0
	case kindFloat:
		b = d.scanFloat() != 0
	}
	v.SetBool(b)
}

func decodeMinMax(d *decodeState, kind int, v reflect.Value) {
	var n int64
	switch kind {
	default:
		d.saveError(&DecodeConvertError{kind, v.Type()})
		return
	case kindMaxValue:
		n = 1
	case kindMinValue:
		n = -1
	}
	v.SetInt(n)
}

func decodeMapStringInterface(d *decodeState, kind int, v reflect.Value) {
	if kind != kindDocument {
		d.saveErrorAndSkip(kind, v.Type())
	}
	if v.IsNil() {
		v.Set(reflect.MakeMap(v.Type()))
	}

	var m map[string]interface{}
	switch mm := v.Interface().(type) {
	case map[string]interface{}:
		m = mm
	case M:
		m = (map[string]interface{})(mm)
	}

	offset := d.beginDoc()
	for {
		kind, name := d.scanKindName()
		if kind == 0 {
			break
		}
		if kind == kindNull {
			continue
		}
		m[string(name)] = d.decodeValueInterface(kind)
	}
	d.endDoc(offset)
}

func decodeMap(d *decodeState, kind int, v reflect.Value) {
	t := v.Type()
	if t.Key().Kind() != reflect.String || kind != kindDocument {
		d.saveErrorAndSkip(kind, t)
		return
	}
	if v.IsNil() {
		v.Set(reflect.MakeMap(t))
	}
	subv := reflect.New(t.Elem()).Elem()
	offset := d.beginDoc()
	for {
		kind, name := d.scanKindName()
		if kind == 0 {
			break
		}
		if kind == kindNull {
			continue
		}
		subv.Set(reflect.Zero(t.Elem()))
		d.decodeValue(kind, subv)
		v.SetMapIndex(reflect.ValueOf(string(name)), subv)
	}
	d.endDoc(offset)
}

func decodeSlice(d *decodeState, kind int, v reflect.Value) {
	t := v.Type()
	if kind == kindBinary && t.Elem().Kind() == reflect.Uint8 {
		decodeByteSlice(d, kind, v)
		return
	}
	if kind != kindArray {
		d.saveErrorAndSkip(kind, v.Type())
		return
	}
	offset := d.beginDoc()
	i := 0
	for {
		kind, _ := d.scanKindName()
		if kind == 0 {
			break
		}
		if i >= v.Cap() {
			newcap := v.Cap() + v.Cap()/2
			if newcap < 4 {
				newcap = 4
			}
			newv := reflect.MakeSlice(t, v.Len(), newcap)
			reflect.Copy(newv, v)
			v.Set(newv)
		}
		if i >= v.Len() {
			v.SetLen(i + 1)
		}
		d.decodeValue(kind, v.Index(i))
		i += 1
	}
	if v.IsNil() {
		v.Set(reflect.MakeSlice(t, 0, 0))
	}
	d.endDoc(offset)
}

func decodeArray(d *decodeState, kind int, v reflect.Value) {
	if kind != kindArray {
		d.saveErrorAndSkip(kind, v.Type())
		return
	}
	offset := d.beginDoc()
	i := 0
	for {
		kind, _ := d.scanKindName()
		if kind == 0 {
			break
		}
		if i < v.Len() {
			d.decodeValue(kind, v.Index(i))
		} else {
			d.skipValue(kind)
		}
		i += 1
	}
	d.endDoc(offset)
}

func decodeStruct(d *decodeState, kind int, v reflect.Value) {
	t := v.Type()
	ss := structSpecForType(t)
	offset := d.beginDoc()
	for {
		kind, name := d.scanKindName()
		if kind == 0 {
			break
		}
		if kind == kindNull {
			continue
		}
		if fs := ss.fieldSpec(name); fs != nil {
			d.decodeValue(kind, v.FieldByIndex(fs.index))
		} else {
			d.skipValue(kind)
		}
	}
	d.endDoc(offset)
}

func decodeInterface(d *decodeState, kind int, v reflect.Value) {
	v.Set(reflect.ValueOf(d.decodeValueInterface(kind)))
}

func (d *decodeState) decodeValueInterface(kind int) interface{} {
	switch kind {
	case kindFloat:
		return d.scanFloat()
	case kindString:
		return d.scanString()
	case kindDocument:
		m := make(map[string]interface{})
		offset := d.beginDoc()
		for {
			kind, name := d.scanKindName()
			if kind == 0 {
				break
			}
			m[string(name)] = d.decodeValueInterface(kind)
		}
		d.endDoc(offset)
		return m
	case kindArray:
		a := make([]interface{}, 0)
		offset := d.beginDoc()
		for {
			kind, _ := d.scanKindName()
			if kind == 0 {
				break
			}
			a = append(a, d.decodeValueInterface(kind))
		}
		d.endDoc(offset)
		return a
	case kindBinary:
		p, _ := d.scanBinary()
		newp := make([]byte, len(p))
		copy(newp, p)
		return newp
	case kindObjectId:
		return ObjectId(string(d.scanSlice(12)))
	case kindBool:
		return d.scanBool()
	case kindDateTime:
		return timeFromMS(d.scanInt64())
	case kindNull:
		return nil
	case kindSymbol:
		return Symbol(d.scanString())
	case kindInt32:
		return int(d.scanInt32())
	case kindTimestamp:
		return Timestamp(d.scanInt64())
	case kindInt64:
		return d.scanInt64()
	case kindMinValue:
		return MinValue
	case kindMaxValue:
		return MaxValue
	default:
		abort(&DecodeTypeError{kind})
	}
	return nil
}

func (d *decodeState) skipValue(kind int) {
	switch kind {
	case kindString, kindSymbol:
		n := int(d.scanInt32())
		d.offset += n
	case kindDocument, kindArray:
		n := int(d.scanInt32())
		d.offset += n - 4
	case kindBinary:
		n := int(d.scanInt32())
		d.offset += n + 1
	case kindObjectId:
		d.offset += 12
	case kindBool:
		d.offset += 1
	case kindDateTime, kindTimestamp, kindInt64, kindFloat:
		d.offset += 8
	case kindInt32:
		d.offset += 4
	case kindMinValue, kindMaxValue, kindNull:
		d.offset += 0
	default:
		abort(&DecodeTypeError{kind})
	}
}

type decoderFunc func(e *decodeState, kind int, v reflect.Value)

var kindDecoder map[reflect.Kind]decoderFunc
var typeDecoder map[reflect.Type]decoderFunc

func init() {
	kindDecoder = map[reflect.Kind]decoderFunc{
		reflect.Bool:    decodeBool,
		reflect.Float32: decodeFloat,
		reflect.Float64: decodeFloat,
		reflect.Int8:    decodeInt,
		reflect.Int16:   decodeInt,
		reflect.Int32:   decodeInt,
		reflect.Int64:   decodeInt,
		reflect.Int:     decodeInt,
		reflect.Uint8:   decodeUint,
		reflect.Uint16:  decodeUint,
		reflect.Uint32:  decodeUint,
		reflect.Uint64:  decodeUint,
		reflect.Uint:    decodeUint,
		reflect.Map:     decodeMap,
		reflect.String:  decodeString,
		reflect.Struct:  decodeStruct,
		reflect.Slice:   decodeSlice,
		reflect.Array:   decodeArray,
	}
	typeDecoder = map[reflect.Type]decoderFunc{
		reflect.TypeOf(BSONData{}):                   decodeBSONData,
		reflect.TypeOf(time.Time{}):                  decodeTime,
		reflect.TypeOf(MinMax(0)):                    decodeMinMax,
		reflect.TypeOf(ObjectId("")):                 decodeObjectId,
		reflect.TypeOf(Symbol("")):                   decodeString,
		reflect.TypeOf(Timestamp(0)):                 decodeTimestamp,
		reflect.TypeOf(make(map[string]interface{})): decodeMapStringInterface,
		reflect.TypeOf(M{}):                          decodeMapStringInterface,
		reflect.TypeOf(new(interface{})).Elem():      decodeInterface,
	}
}
