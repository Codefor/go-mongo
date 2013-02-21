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
	"bytes"
	"fmt"
	"log"
)

// NewLoggingConn returns a logging wrapper around a connection.
func NewLoggingConn(conn Conn, log *log.Logger, prefix string) Conn {
	if prefix != "" {
		prefix = prefix + "."
	}
	return &loggingConn{conn, log, prefix, 0}
}

type loggingConn struct {
	Conn
	log      *log.Logger
	prefix   string
	cursorId int
}

func (c *loggingConn) Close() error {
	err := c.Conn.Close()
	c.log.Printf("%sClose() (err: %v)", c.prefix, err)
	return err
}

func (c *loggingConn) Update(namespace string, selector, update interface{}, options *UpdateOptions) error {
	err := c.Conn.Update(namespace, selector, update, options)
	var buf bytes.Buffer
	if options != nil {
		if options.Upsert {
			buf.WriteString(", upsert=true")
		}
		if options.Multi {
			buf.WriteString(", multi=true")
		}
	}
	c.log.Printf("%sUpdate(%+v, %+v, %+v%s) (%v)", c.prefix, namespace, selector, update, buf.String(), err)
	return err
}

func (c *loggingConn) Insert(namespace string, options *InsertOptions, documents ...interface{}) error {
	err := c.Conn.Insert(namespace, options, documents...)
	var buf bytes.Buffer
	if options != nil {
		if options.ContinueOnError {
			buf.WriteString(", continue=true")
		}
	}
	c.log.Printf("%sInsert(%s%s, %+v) (%v)", c.prefix, namespace, buf.String(), documents, err)
	return err
}

func (c *loggingConn) Remove(namespace string, selector interface{}, options *RemoveOptions) error {
	err := c.Conn.Remove(namespace, selector, options)
	var buf bytes.Buffer
	if options != nil {
		if options.Single {
			buf.WriteString(", single=true")
		}
	}
	c.log.Printf("%sRemove(%s, %+v%s) (%v)", c.prefix, namespace, selector, buf.String(), err)
	return err
}

func (c *loggingConn) Find(namespace string, query interface{}, options *FindOptions) (Cursor, error) {
	r, err := c.Conn.Find(namespace, query, options)
	prefix := ""
	if r != nil {
		c.cursorId += 1
		prefix = fmt.Sprintf("%s%d.", c.prefix, c.cursorId)
		r = &logCursor{r, c.log, prefix}
	}
	var buf bytes.Buffer
	if options != nil {
		if options.Fields != nil {
			buf.WriteString(", fields:")
			fmt.Fprintf(&buf, "%+v", options.Fields)
		}
		if options.Tailable {
			buf.WriteString(", tailable:true")
		}
		if options.SlaveOk {
			buf.WriteString(", slaveOK:true")
		}
		if options.NoCursorTimeout {
			buf.WriteString(", noCursorTimeout:true")
		}
		if options.AwaitData {
			buf.WriteString(", awaitData:true")
		}
		if options.Exhaust {
			buf.WriteString(", exhaust:true")
		}
		if options.PartialResults {
			buf.WriteString(", partialResults:true")
		}
		if options.Skip != 0 {
			fmt.Fprintf(&buf, ", skip:%d", options.Skip)
		}
		if options.Limit != 0 {
			fmt.Fprintf(&buf, ", limit:%d", options.Limit)
		}
		if options.BatchSize != 0 {
			fmt.Fprintf(&buf, ", batchSize:%d", options.BatchSize)
		}
	}
	c.log.Printf("%sFind(%s, %+v%s) (%s, %v)", c.prefix, namespace, query, buf.String(), prefix[:len(prefix)-1], err)
	return r, err
}

type logCursor struct {
	Cursor
	log    *log.Logger
	prefix string
}

func (r *logCursor) Close() error {
	err := r.Cursor.Close()
	r.log.Printf("%sClose() (%v)", r.prefix, err)
	return err
}

func (r *logCursor) Next(value interface{}) error {
	var bd BSONData
	err := r.Cursor.Next(&bd)
	var m M
	if err == nil {
		err = Decode(bd.Data, value)
		bd.Decode(&m)
	}
	r.log.Printf("%sNext() (%v, %v)", r.prefix, m, err)
	return err
}
