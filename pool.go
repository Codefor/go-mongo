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

// Pool maintains a pool of database connections.
//
// The following example shows how to use a pool in a web application. The
// application creates a pool at application startup and makes it available to
// request handlers, possibly using a global variable:
//
//      var server string           // host:port of server
//      var name, password string   // authentication credentials
//
//      ...
//
//      pool = mongo.NewPool(func () (c mongo.Conn, err os.Error) {
//          c, err = mongo.Dial(server)
//            if err != nil {
//                return
//            }
//            err = mongo.Database{c, "admin", nil}.Authenticate(name, password)
//            if err != nil {
//                c.Close()
//                c = nil
//            }
//            return
//        }, 3)
//
// This pool has a maximum of three connections to the server specified by the
// variable "server". Each connection is logged into the "admin" database using
// the credentials specified by the variables "name" and "password".
//
// A request handler gets a connection from the pool and closes the connection
// when the handler is done:
//
//  conn, err := pool.Get()
//  if err != nil {
//      // handle the error
//  }
//  defer conn.Close()
//  // do something with the connection
//
// Close() returns the connection to the pool if there's room in the pool and
// the connection does not have a permanent error. Otherwise, Close() releases
// the resources used by the connection.
type Pool struct {
	newFn func() (Conn, error)
	conns chan Conn
}

type pooledConnection struct {
	Conn
	pool *Pool
}

// NewDialPool returns a new connection pool. The pool uses mongo.Dial to
// create new connections and maintains a maximum of maxIdle connections.
func NewDialPool(addr string, maxIdle int) *Pool {
	return NewPool(func() (Conn, error) { return Dial(addr) }, maxIdle)
}

// NewPool returns a new connection pool. The pool uses newFn to create
// connections as needed and maintains a maximum of maxIdle idle connections.
func NewPool(newFn func() (Conn, error), maxIdle int) *Pool {
	return &Pool{newFn: newFn, conns: make(chan Conn, maxIdle)}
}

// Get returns an idle connection from the pool if available or creates a new
// connection. The caller should Close() the connection to return the
// connection to the pool.
func (p *Pool) Get() (Conn, error) {
	var c Conn
	select {
	case c = <-p.conns:
	default:
		var err error
		c, err = p.newFn()
		if err != nil {
			return nil, err
		}
	}
	return &pooledConnection{Conn: c, pool: p}, nil
}

func (c *pooledConnection) Close() error {
	if c.Conn == nil {
		return nil
	}
	if c.Err() != nil {
		return nil
	}
	select {
	case c.pool.conns <- c.Conn:
	default:
		c.Conn.Close()
	}
	c.Conn = nil
	return nil
}
