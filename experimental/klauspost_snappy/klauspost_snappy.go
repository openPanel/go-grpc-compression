/*
 *
 * Copyright 2017 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// Package experimental/klauspost_snappy is a wrapper for using
// github.com/klauspost/compress/s2 in snappy compatibility mode with gRPC.
// It might be more efficient than the top-level snappy package which makes
// use of github.com/golang/snappy.
//
// Note that this is registered under the same "snappy" name with gRPC, so
// only one of the two packages should be used at a time.
package klauspost_snappy

// This code is based upon the gzip wrapper in github.com/grpc/grpc-go:
// https://github.com/grpc/grpc-go/blob/master/encoding/gzip/gzip.go

import (
	"io"
	"io/ioutil"
	"sync"

	snappylib "github.com/klauspost/compress/s2"
	"google.golang.org/grpc/encoding"
)

const Name = "snappy"

type compressor struct {
	poolCompressor   sync.Pool
	poolDecompressor sync.Pool
}

type writer struct {
	*snappylib.Writer
	pool *sync.Pool
}

type reader struct {
	*snappylib.Reader
	pool *sync.Pool
}

func init() {
	c := &compressor{}
	c.poolCompressor.New = func() interface{} {
		w := snappylib.NewWriter(ioutil.Discard, snappylib.WriterSnappyCompat())
		return &writer{Writer: w, pool: &c.poolCompressor}
	}
	encoding.RegisterCompressor(c)
}

func (c *compressor) Compress(w io.Writer) (io.WriteCloser, error) {
	z := c.poolCompressor.Get().(*writer)
	z.Writer.Reset(w)
	return z, nil
}

func (c *compressor) Decompress(r io.Reader) (io.Reader, error) {
	z, inPool := c.poolDecompressor.Get().(*reader)
	if !inPool {
		newR := snappylib.NewReader(r, snappylib.ReaderAllocBlock(64 << 10))
		return &reader{Reader: newR, pool: &c.poolDecompressor}, nil
	}
	z.Reset(r)
	return z, nil
}

func (c *compressor) Name() string {
	return Name
}

func (z *writer) Close() error {
	err := z.Writer.Close()
	z.pool.Put(z)
	return err
}

func (z *reader) Read(p []byte) (n int, err error) {
	n, err = z.Reader.Read(p)
	if err == io.EOF {
		z.pool.Put(z)
	}
	return n, err
}
