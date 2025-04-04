// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package fswalker contains functionality to walk a file system and compare the differences.
package fswalker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

// Generating Go representations for the proto buf libraries.

//go:generate protoc -I=. -I=$GOPATH/src --go_out=paths=source_relative:. proto/fswalker/fswalker.proto

const (
	// tsFileFormat is the time format used in file names.
	tsFileFormat = "20060102-150405"
)

// WalkFilename returns the appropriate filename for a Walk for the given host and time.
// If time is not provided, it returns a file pattern to glob by.
func WalkFilename(hostname string, t time.Time) string {
	hn := "*"
	if hostname != "" {
		hn = hostname
	}
	ts := "*"
	if !t.IsZero() {
		ts = t.Format(tsFileFormat)
	}
	return fmt.Sprintf("%s-%s-fswalker-state.pb", hn, ts)
}

// NormalizePath returns a cleaned up path with a path separator at the end if it's a directory.
// It should always be used when printing or comparing paths.
func NormalizePath(path string, isDir bool) string {
	p := filepath.Clean(path)
	if isDir && p[len(p)-1] != filepath.Separator {
		p += string(filepath.Separator)
	}
	return p
}

// helper for deferred calls
func callAndLogOnError(f func() error) {
	if err := f(); err != nil {
		log.Println("error when calling (probably deferred) function:", err)
	}
}

// sha256sum reads the given file path and builds a SHA-256 sum over its content.
func sha256sum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer callAndLogOnError(f.Close)

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// readTextProto reads a text format proto buf and unmarshals it into the provided proto message.
func readTextProto(ctx context.Context, path string, pb proto.Message) error {
	b, err := ReadFile(ctx, path)
	if err != nil {
		return err
	}
	return prototext.Unmarshal(b, pb)
}

// writeTextProto writes a text format proto buf for the provided proto message.
func writeTextProto(ctx context.Context, path string, pb proto.Message) error {
	blob, err := prototext.Marshal(pb)
	if err != nil {
		return err
	}
	// replace message boundary characters as curly braces look nicer (both is fine to parse)
	blobStr := strings.ReplaceAll(strings.ReplaceAll(string(blob), "<", "{"), ">", "}")
	return WriteFile(ctx, path, []byte(blobStr), 0644)
}

// ReadFile reads the file named by filename and returns the contents.
func ReadFile(_ context.Context, filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

// WriteFile writes data to a file named by filename.
func WriteFile(_ context.Context, filename string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

// Glob returns the names of all files matching pattern or nil if there is no matching file.
func Glob(_ context.Context, pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}
