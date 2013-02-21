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
	"bufio"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
)

const (
	insertContinueOnError = 1 << 0
	updateUpsert          = 1 << 0
	updateMulti           = 1 << 1
	removeSingle          = 1 << 0
	queryTailable         = 1 << 1
	querySlaveOk          = 1 << 2
	queryNoCursorTimeout  = 1 << 4
	queryAwaitData        = 1 << 5
	queryExhaust          = 1 << 6
	queryPartialResults   = 1 << 7
	cursorNotFound        = 1 << 0
	queryFailure          = 1 << 1
)

type connection struct {
	conn          net.Conn
	addr          string
	requestId     uint32
	cursors       map[uint32]*cursor
	err           error
	buf           [1024]byte
	responseLen   int
	responseCount int
	cursor        *cursor
	br            *bufio.Reader
}

type cursor struct {
	conn      *connection
	namespace string
	requestId uint32
	cursorId  uint64
	limit     int
	batchSize int
	count     int
	docs      [][]byte
	flags     int
	err       error
}

// Dial connects to server at addr.
func Dial(addr string) (Conn, error) {
	if strings.LastIndex(addr, ":") <= strings.LastIndex(addr, "]") {
		addr = addr + ":27017"
	}
	c := connection{
		addr:    addr,
		cursors: make(map[uint32]*cursor),
	}
	return &c, c.connect()
}

func (c *connection) connect() error {
	conn, err := net.Dial("tcp", c.addr)
	if err != nil {
		return err
	}
	if c.conn != nil {
		c.conn.Close()
	}
	c.conn = conn
	c.br = bufio.NewReader(conn)
	return nil
}

func (c *connection) nextId() uint32 {
	c.requestId += 1
	return c.requestId
}

func (c *connection) fatal(err error) error {
	if c.err == nil {
		c.Close()
		c.err = err
	}
	return err
}

// Close closes the connection to the server.
func (c *connection) Close() (err error) {
	if c.conn != nil {
		err = c.conn.Close()
		c.conn = nil
	}
	c.cursors = nil
	c.cursor = nil
	c.responseCount = 0
	c.responseLen = 0
	c.err = errors.New("mongo: connection closed")
	return err
}

func (c *connection) Err() error {
	return c.err
}

// send sets the message length and writes the message to the socket.
func (c *connection) send(msg []byte) error {
	if c.err != nil {
		return c.err
	}
	wire.PutUint32(msg[0:4], uint32(len(msg)))
	_, err := c.conn.Write(msg)
	if err != nil {
		return c.fatal(err)
	}
	return nil
}

func (c *connection) Update(namespace string, selector, update interface{}, options *UpdateOptions) (err error) {
	if selector == nil {
		selector = emptyDoc
	}
	flags := 0
	if options != nil {
		if options.Upsert {
			flags |= updateUpsert
		}
		if options.Multi {
			flags |= updateMulti
		}
	}

	b := buffer(c.buf[:0])
	b.Next(4)                    // placeholder for message length
	b.WriteUint32(c.nextId())    // requestId
	b.WriteUint32(0)             // responseTo
	b.WriteUint32(2001)          // opCode
	b.WriteUint32(0)             // reserved
	b.WriteCString(namespace)    // namespace
	b.WriteUint32(uint32(flags)) // flags
	b, err = Encode(b, selector)
	if err != nil {
		return err
	}
	b, err = Encode(b, update)
	if err != nil {
		return err
	}
	return c.send(b)
}

func (c *connection) Insert(namespace string, options *InsertOptions, documents ...interface{}) (err error) {
	if len(documents) == 0 {
		return errors.New("mongo: insert with no documents")
	}
	flags := 0
	if options != nil {
		if options.ContinueOnError {
			flags |= insertContinueOnError
		}
	}
	b := buffer(c.buf[:0])
	b.Next(4)                    // placeholder for message length
	b.WriteUint32(c.nextId())    // requestId
	b.WriteUint32(0)             // responseTo
	b.WriteUint32(2002)          // opCode
	b.WriteUint32(uint32(flags)) // flags
	b.WriteCString(namespace)    // namespace
	for _, document := range documents {
		b, err = Encode(b, document)
		if err != nil {
			return err
		}
	}
	return c.send(b)
}

func (c *connection) Remove(namespace string, selector interface{}, options *RemoveOptions) (err error) {
	if selector == nil {
		selector = emptyDoc
	}
	flags := 0
	if options != nil {
		if options.Single {
			flags |= removeSingle
		}
	}
	b := buffer(c.buf[:0])
	b.Next(4)                    // placeholder for message length
	b.WriteUint32(c.nextId())    // requestId
	b.WriteUint32(0)             // responseTo
	b.WriteUint32(2006)          // opCode
	b.WriteUint32(0)             // reserved
	b.WriteCString(namespace)    // namespace
	b.WriteUint32(uint32(flags)) // flags
	b, err = Encode(b, selector)
	if err != nil {
		return err
	}
	return c.send(b)
}

func (c *connection) Find(namespace string, query interface{}, options *FindOptions) (Cursor, error) {
	r := cursor{
		conn:      c,
		namespace: namespace,
		requestId: c.nextId(),
	}

	if query == nil {
		query = emptyDoc
	}

	var fields interface{}
	var skip int
	if options != nil {
		skip = options.Skip
		fields = options.Fields
		r.limit = options.Limit
		r.batchSize = options.BatchSize
		if r.batchSize == 1 {
			// Server handles numberToReturn == 1 as hard limit. Change value
			// to two to avoid having batch size set hard limit.
			r.batchSize = 2
		}
		if options.Tailable {
			r.flags |= queryTailable
			r.limit = 0
		}
		if options.SlaveOk {
			r.flags |= querySlaveOk
		}
		if options.NoCursorTimeout {
			r.flags |= queryNoCursorTimeout
		}
		if options.AwaitData {
			r.flags |= queryAwaitData
		}
		if options.Exhaust {
			r.flags |= queryExhaust
		}
		if options.PartialResults {
			r.flags |= queryPartialResults
		}
	}

	b := buffer(c.buf[:0])
	b.Next(4)                         // placeholder for message length
	b.WriteUint32(r.requestId)        // requestId
	b.WriteUint32(0)                  // responseTo
	b.WriteUint32(2004)               // opCode
	b.WriteUint32(uint32(r.flags))    // flags
	b.WriteCString(namespace)         // namespace
	b.WriteUint32(uint32(skip))       // numberToSkip
	b.WriteUint32(r.numberToReturn()) // numberToReturn
	b, err := Encode(b, query)
	if err != nil {
		return nil, err
	}
	if fields != nil {
		b, err = Encode(b, fields)
		if err != nil {
			return nil, err
		}
	}
	err = c.send(b)
	if err != nil {
		return nil, err
	}

	c.cursors[r.requestId] = &r
	return &r, nil
}

func (c *connection) getMore(r *cursor) error {
	requestId := c.nextId()
	b := buffer(c.buf[:0])
	b.Next(4)                   // placeholder for message length
	b.WriteUint32(requestId)    // requestId
	b.WriteUint32(0)            // responseTo
	b.WriteUint32(2005)         // opCode
	b.WriteUint32(0)            // reserved
	b.WriteCString(r.namespace) // namespace
	b.WriteUint32(r.numberToReturn())
	b.WriteUint64(r.cursorId)
	if err := c.send(b); err != nil {
		return err
	}
	r.requestId = requestId
	c.cursors[requestId] = r
	return nil
}

func (c *connection) killCursors(cursorIds ...uint64) error {
	b := buffer(c.buf[:0])
	b.Next(4)                             // placeholder for message length
	b.WriteUint32(c.nextId())             // requestId
	b.WriteUint32(0)                      // responseTo
	b.WriteUint32(2007)                   // opCode
	b.WriteUint32(0)                      // zero
	b.WriteUint32(uint32(len(cursorIds))) // number of cursor ids.
	for _, cursorId := range cursorIds {
		b.WriteUint64(cursorId)
	}
	return c.send(b)
}

// readDoc reads a single document from the connection.
func (c *connection) readDoc(alloc bool) ([]byte, error) {
	if c.responseLen < 4 {
		return nil, c.fatal(errors.New("mongo: incomplete document in message"))
	}
	b, err := c.br.Peek(4)
	if err != nil {
		return nil, c.fatal(err)
	}
	n := int(wire.Uint32(b))
	if c.responseLen < n {
		return nil, c.fatal(errors.New("mongo: incomplete document in message"))
	}
	var p []byte
	if n > len(c.buf) || alloc {
		p = make([]byte, n)
	} else {
		p = c.buf[:n]
	}
	_, err = io.ReadFull(c.br, p)
	if err != nil {
		return nil, c.fatal(err)
	}
	c.responseLen -= n
	c.responseCount -= 1
	if c.responseCount == 0 {
		c.cursor = nil
		if c.responseLen != 0 {
			return nil, c.fatal(errors.New("mongo: unexpected data in message."))
		}
	}
	return p, nil
}

// skipDocs skips over unread documents in the current batch.
func (c *connection) skipDocs() error {
	for c.responseLen > 0 {
		n := c.responseLen
		if n > len(c.buf) {
			n = len(c.buf)
		}
		var err error
		n, err = c.br.Read(c.buf[0:n])
		if err != nil {
			return c.fatal(err)
		}
		c.responseLen -= n
	}
	c.responseLen = 0
	c.responseCount = 0
	c.cursor = nil
	return nil
}

// receive receives a single response from the server and delivers it to the
// appropriate cursor.
func (c *connection) receive() error {

	if c.err != nil {
		return c.err
	}

	// Slurp up documents for current cursor.
	for c.responseCount > 0 {
		r := c.cursor
		p, err := c.readDoc(true)
		if err != nil {
			return err
		}
		r.docs = append(r.docs, p)
	}

	// Read response message header.
	if _, err := io.ReadFull(c.br, c.buf[:36]); err != nil {
		return c.fatal(err)
	}

	c.responseLen = int(wire.Uint32(c.buf[0:4]))
	requestId := wire.Uint32(c.buf[4:8])
	responseTo := wire.Uint32(c.buf[8:12])
	opCode := int32(wire.Uint32(c.buf[12:16]))
	flags := wire.Uint32(c.buf[16:20])
	cursorId := wire.Uint64(c.buf[20:28])
	//startingFrom := int32(wire.Uint32(c.buf[28:32]))
	c.responseCount = int(wire.Uint32(c.buf[32:36]))
	c.responseLen -= 36

	if opCode != 1 {
		return c.fatal(errors.New("mongo: unknown response opcode " + strconv.Itoa(int(opCode))))
	}

	r := c.cursors[responseTo]
	if r == nil {
		if cursorId != 0 {
			if err := c.killCursors(cursorId); err != nil {
				return err
			}
		}
		if err := c.skipDocs(); err != nil {
			return err
		}
		return c.err
	}

	delete(c.cursors, responseTo)
	r.cursorId = cursorId
	r.requestId = 0
	if r.flags&queryExhaust != 0 && cursorId != 0 {
		r.requestId = requestId
		c.cursors[requestId] = r
	}

	if flags&cursorNotFound != 0 {
		r.fatal(errors.New("mongo: cursor not found"))
		if c.responseCount != 0 || c.responseLen != 0 {
			return c.fatal(errors.New("mongo: unexpected data after cursor not found."))
		}
	}

	if flags&queryFailure != 0 {
		if c.responseCount != 1 {
			return c.fatal(errors.New("mongo: unexpected number of docs for query failure."))
		}
		p, err := c.readDoc(false)
		if err != nil {
			return err
		}
		var m M
		err = Decode(p, &m)
		if err != nil {
			r.fatal(err)
		} else if s, ok := m["$err"].(string); ok {
			r.fatal(errors.New(s))
		} else {
			r.fatal(errors.New("mongo: query failure"))
		}
		return c.err
	}

	if c.responseCount > 0 {
		c.cursor = r
	}

	return c.err
}

func (r *cursor) numberToReturn() uint32 {
	batchSize := r.batchSize
	if batchSize < 0 {
		batchSize *= -1
	}

	remaining := 0
	if r.limit > 0 {
		remaining = r.limit - r.count
	}

	n := 0
	switch {
	case batchSize == 0 && remaining > 0:
		n = remaining
	case batchSize > 0 && remaining == 0:
		n = batchSize
	case remaining < batchSize:
		n = remaining
	default:
		n = batchSize
	}

	if r.batchSize < 0 {
		n *= -1
	}

	if n == 1 {
		n = -1
	}
	return uint32(n)
}

func (r *cursor) Close() error {
	if r.err != nil {
		return nil
	}
	if r.cursorId != 0 {
		r.conn.killCursors(r.cursorId)
	}
	if r.conn.cursor == r {
		r.conn.skipDocs()
	}
	if r.requestId != 0 && r.conn.cursors != nil {
		delete(r.conn.cursors, r.requestId)
	}
	r.err = errors.New("mongo: cursor closed")
	r.conn = nil
	return nil
}

func (r *cursor) fatal(err error) error {
	if r.err == nil {
		r.Close()
		r.err = err
	}
	return err
}

func (r *cursor) Err() error {
	return r.err
}

func (r *cursor) HasNext() bool {
	// If HasNext() dectects an error other than EOF, then HasNext returns true
	// so that the error is returned to the application on a subsequent call to
	// Next().

	if r.err != nil {
		return r.err != Done
	}

	if len(r.docs) > 0 || r.conn.cursor == r {
		return true
	}

	if r.requestId == 0 {
		if r.cursorId == 0 {
			r.fatal(Done)
			return false
		}
		if err := r.conn.getMore(r); err != nil {
			r.fatal(err)
			return true
		}
	}

	requestId := r.requestId
	for r.requestId == requestId {
		if err := r.conn.receive(); err != nil {
			r.fatal(err)
			break
		}
	}

	switch {
	case r.err != nil:
		return r.err != Done
	case r.conn.cursor == r:
		return true
	case r.cursorId == 0:
		r.fatal(Done)
		return false
	}

	// Tailable cursor case
	return false
}

func (r *cursor) Next(value interface{}) error {
	if !r.HasNext() {
		return Done
	}

	if r.err != nil {
		return r.err
	}

	var p []byte
	switch {
	case len(r.docs) > 0:
		p = r.docs[0]
		r.docs[0] = nil
		r.docs = r.docs[1:]
	case r.conn.cursor == r:
		var err error
		p, err = r.conn.readDoc(false)
		if err != nil {
			return r.fatal(err)
		}
	default:
		panic("unexpected state")
	}

	err := Decode(p, value)

	r.count += 1
	if r.limit > 0 && r.count >= r.limit {
		r.fatal(Done)
	}

	return err
}
