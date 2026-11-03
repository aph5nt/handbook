// Claim check: send a ticket for the blob, not the blob.
//
// A map stands in for object storage and a slice for the topic. Time is in
// minutes and only added up, so nothing sleeps. Blob size is this example's.
// Deterministic.
package main

import (
	"errors"
	"fmt"
)

type Ticket struct {
	Path     string
	Size     int
	Gas      int // a reader that needs only this never fetches the blob
	LinkEnds int // minute a presigned link expires, 0 = no link
}

type Store struct {
	blobs map[string]int // path -> size
	puts  map[string]int // path -> times uploaded
}

func (s *Store) Put(path string, size int) {
	s.blobs[path] = size
	s.puts[path]++
}

var ErrNotFound = errors.New("404 not found")
var ErrExpired = errors.New("403 access denied: link expired")

func (s *Store) Get(t Ticket, now int) (int, error) {
	if t.LinkEnds != 0 && now > t.LinkEnds {
		return 0, ErrExpired
	}
	size, ok := s.blobs[t.Path]
	if !ok {
		return 0, ErrNotFound
	}
	return size, nil
}

func main() {
	const path, size = "receipts/9506.bin", 5_000_000
	fmt.Println("== Tab 1: upload, then send the ticket ==")
	s := &Store{map[string]int{}, map[string]int{}}
	s.Put(path, size)
	t := Ticket{Path: path, Size: size, Gas: 21000}
	fmt.Printf("ticket: path %q, %d bytes of blob stay in storage\n", t.Path, t.Size)
	n, err := s.Get(t, 0)
	fmt.Printf("graph sync fetched %d bytes, err=%v\n", n, err)
	fmt.Printf("alert rule read gas %d from the ticket, fetched nothing\n", t.Gas)

	fmt.Println("== Tab 2: publish before the upload is done ==")
	s = &Store{map[string]int{}, map[string]int{}}
	_, err = s.Get(t, 0) // the ticket arrived first
	fmt.Printf("fast reader, upload not finished: %v\n", err)
	s.Put(path, size)
	n, err = s.Get(t, 0)
	fmt.Printf("fix, publish after the upload returned: %d bytes, err=%v\n", n, err)
	s.Put(path, size) // a retry after a timeout: same name
	fmt.Printf("retry with a name from the block number: %d uploads, %d file\n", s.puts[path], len(s.blobs))

	fmt.Println("== Tab 3: expiry ==")
	link := t
	link.LinkEnds = 15 // valid for 15 minutes
	_, err = s.Get(link, 60)
	fmt.Printf("price service back after 60 minutes, link valid 15: %v\n", err)
	n, err = s.Get(t, 60)
	fmt.Printf("fix, a path and own credentials: %d bytes, err=%v\n", n, err)
	topicDays, blobDays := 7, 8
	safe := blobDays > topicDays // 8 > 7
	fmt.Printf("lifecycle: blob deleted after %d days, topic keeps messages %d days: safe=%v\n", blobDays, topicDays, safe)
}
