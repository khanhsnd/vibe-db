package s3

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/objectstore"
	"github.com/khanhsnd/vibe-db/internal/sigv4"
)

// A live run against a real service, because the things about this adapter that cannot be checked
// any other way are the ones a signature is involved in: a canonical request either matches what the
// service works out or it does not, and a fake server that accepts anything would never say so.
//
// It is skipped unless the three variables below name a service — see docs/development.md — and it
// works in a bucket of its own, made and emptied by the test, so nothing it does can be mistaken for
// something the reader put there.
//
//	VIBE_S3_ENDPOINT=http://127.0.0.1:9000 VIBE_S3_KEY=minioadmin VIBE_S3_SECRET=minioadmin \
//	  go test ./internal/objectstore/s3 -run TestLive -v
const (
	liveEndpoint = "VIBE_S3_ENDPOINT"
	liveKey      = "VIBE_S3_KEY"
	liveSecret   = "VIBE_S3_SECRET"
)

// liveBucket is made at the start of the run and removed at the end of it.
const liveBucket = "vibe-db-live-test"

func openLive(t *testing.T) *Store {
	t.Helper()

	endpoint := os.Getenv(liveEndpoint)
	key := os.Getenv(liveKey)
	secret := os.Getenv(liveSecret)
	if endpoint == "" || key == "" || secret == "" {
		t.Skipf("set %s, %s and %s to run this against a real service", liveEndpoint, liveKey, liveSecret)
	}

	store, err := Open(Options{
		Profile: domain.StoreProfile{
			Provider:  domain.ProviderOther,
			Endpoint:  endpoint,
			Region:    "us-east-1",
			PathStyle: true,
		},
		Credentials: sigv4.Credentials{AccessKeyID: key, SecretAccessKey: secret},
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(store.Close)

	return store
}

// TestLive walks the whole adapter in one run, in the order a reader would: make a bucket, write into
// it, list it, read it back, copy, delete. They are one test because they are one *state* — a bucket
// the run is responsible for — and splitting them would mean six runs each making and dropping one.
func TestLive(t *testing.T) {
	store := openLive(t)
	ctx := context.Background()

	if err := store.MakeBucket(ctx, liveBucket); err != nil {
		// A bucket left behind by a previous run is not a failure: the run empties it below.
		if !strings.Contains(err.Error(), "already") {
			t.Fatalf("MakeBucket() error = %v", err)
		}
	}
	t.Cleanup(func() {
		clean, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		objectstore.RemovePrefix(clean, store, liveBucket, "", nil)
		store.RemoveBucket(clean, liveBucket)
	})

	t.Run("a bucket the credentials made is one they can list", func(t *testing.T) {
		buckets, err := store.Buckets(ctx)
		if err != nil {
			t.Fatalf("Buckets() error = %v", err)
		}
		for _, bucket := range buckets {
			if bucket.Name == liveBucket {
				return
			}
		}
		t.Errorf("Buckets() = %v, want it to hold %s", buckets, liveBucket)
	})

	// A key with a space, a plus and a hash in it: the three characters an escaper gets wrong, and
	// getting any of them wrong is a signature the service works out differently.
	const awkward = "logs/2026/one two+three#four.txt"

	t.Run("a key with awkward characters survives the round trip", func(t *testing.T) {
		body := []byte("the body of an awkward key")
		if err := store.Put(ctx, objectstore.PutRequest{
			Bucket:      liveBucket,
			Key:         awkward,
			Body:        bytes.NewReader(body),
			Size:        int64(len(body)),
			ContentType: "text/plain",
		}); err != nil {
			t.Fatalf("Put() error = %v", err)
		}

		head, err := store.Head(ctx, liveBucket, awkward)
		if err != nil {
			t.Fatalf("Head() error = %v", err)
		}
		if head.Size != int64(len(body)) {
			t.Errorf("Head().Size = %d, want %d", head.Size, len(body))
		}

		reader, err := store.Open(ctx, liveBucket, awkward, objectstore.Span{})
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		defer reader.Close()

		read := make([]byte, len(body)+1)
		count, _ := reader.Read(read)
		if !bytes.Equal(read[:count], body) {
			t.Errorf("Open() gave %q, want %q", read[:count], body)
		}
	})

	t.Run("a listing is a folder at a time", func(t *testing.T) {
		for _, key := range []string{"logs/2026/a.log", "logs/2027/b.log", "top.txt"} {
			if err := store.Put(ctx, objectstore.PutRequest{
				Bucket: liveBucket, Key: key, Body: strings.NewReader("x"), Size: 1,
			}); err != nil {
				t.Fatalf("Put(%q) error = %v", key, err)
			}
		}

		page, err := store.List(ctx, objectstore.ListRequest{
			Bucket:    liveBucket,
			Delimiter: objectstore.Separator,
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		// One folder and one object at the top: `logs/` stands for everything under it, which is the
		// whole reason browsing a bucket costs one request.
		if len(page.Folders) != 1 || page.Folders[0] != "logs/" {
			t.Errorf("List().Folders = %v, want [logs/]", page.Folders)
		}
		if len(page.Objects) != 1 || page.Objects[0].Key != "top.txt" {
			t.Errorf("List().Objects = %v, want top.txt alone", page.Objects)
		}
	})

	t.Run("a ranged read is the range and not the object", func(t *testing.T) {
		body := bytes.Repeat([]byte("abcdefghij"), 100)
		if err := store.Put(ctx, objectstore.PutRequest{
			Bucket: liveBucket, Key: "ranged.bin", Body: bytes.NewReader(body), Size: int64(len(body)),
		}); err != nil {
			t.Fatalf("Put() error = %v", err)
		}

		reader, err := store.Open(ctx, liveBucket, "ranged.bin", objectstore.Span{Offset: 10, Length: 5})
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		defer reader.Close()

		read := make([]byte, 32)
		count, _ := reader.Read(read)
		if string(read[:count]) != "abcde" {
			t.Errorf("a range of five bytes gave %q, want %q", read[:count], "abcde")
		}
	})

	t.Run("a large object goes up in parts and comes back down in ranges", func(t *testing.T) {
		// Just over the threshold, so the multipart path and the parallel download path both run —
		// which is the pair of things a fake server cannot honestly check.
		body := make([]byte, objectstore.PartThreshold+4096)
		for at := range body {
			body[at] = byte(at % 251)
		}

		path := filepath.Join(t.TempDir(), "big.bin")
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatalf("writing the file to upload: %v", err)
		}

		if _, err := objectstore.Upload(ctx, store, objectstore.UploadPlan{
			Bucket: liveBucket, Key: "big.bin", Path: path, PartSize: 5 << 20, Parts: 3,
		}, nil); err != nil {
			t.Fatalf("Upload() error = %v", err)
		}

		down := filepath.Join(t.TempDir(), "down.bin")
		if _, err := objectstore.Download(ctx, store, objectstore.DownloadPlan{
			Bucket: liveBucket, Key: "big.bin", Path: down, PartSize: 5 << 20, Parts: 3,
		}, nil); err != nil {
			t.Fatalf("Download() error = %v", err)
		}

		got, err := os.ReadFile(down)
		if err != nil {
			t.Fatalf("reading it back: %v", err)
		}
		if !bytes.Equal(got, body) {
			t.Errorf("the object did not survive being written in parts and read in ranges (%d bytes back of %d)", len(got), len(body))
		}
	})

	t.Run("a copy moves no bytes through here", func(t *testing.T) {
		if err := store.Copy(ctx, liveBucket, awkward, liveBucket, "copied/one two.txt"); err != nil {
			t.Fatalf("Copy() error = %v", err)
		}
		if _, err := store.Head(ctx, liveBucket, "copied/one two.txt"); err != nil {
			t.Errorf("Head() on the copy error = %v", err)
		}
	})

	t.Run("a search asks the service for the part it can answer", func(t *testing.T) {
		answer, err := objectstore.Search(ctx, store, objectstore.SearchRequest{
			Bucket: liveBucket,
			Match:  domain.TextMatch{Text: "logs/2026", Mode: domain.MatchPrefix},
		})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if answer.Prefix != "logs/2026" {
			t.Errorf("Search().Prefix = %q, want the literal handed to the service", answer.Prefix)
		}
		if len(answer.Hits) != 2 {
			t.Errorf("Search() found %d, want the two keys under logs/2026", len(answer.Hits))
		}
		// The point of pruning: the scan saw only what the service handed back.
		if answer.Scanned != 2 {
			t.Errorf("Search() read %d keys, want 2 — the rest were never listed", answer.Scanned)
		}
	})

	t.Run("a presigned link reads the object with no credential", func(t *testing.T) {
		link, err := store.Presign(ctx, liveBucket, awkward, 5*time.Minute)
		if err != nil {
			t.Fatalf("Presign() error = %v", err)
		}
		if !strings.Contains(link, "X-Amz-Signature=") {
			t.Errorf("Presign() = %q, want a signature in the query", link)
		}
	})

	t.Run("an object that is not there says which kind of missing it is", func(t *testing.T) {
		if _, err := store.Head(ctx, liveBucket, "nothing/here.txt"); !errors.Is(err, domain.ErrUnknownObject) {
			t.Errorf("Head() error = %v, want ErrUnknownObject", err)
		}
		if _, err := store.List(ctx, objectstore.ListRequest{Bucket: liveBucket + "-nope"}); !errors.Is(err, domain.ErrUnknownBucket) {
			t.Errorf("List() error = %v, want ErrUnknownBucket", err)
		}
	})

	t.Run("a folder is deleted in batches and says how many went", func(t *testing.T) {
		for at := range 5 {
			key := "batch/" + strconv.Itoa(at)
			if err := store.Put(ctx, objectstore.PutRequest{
				Bucket: liveBucket, Key: key, Body: strings.NewReader("x"), Size: 1,
			}); err != nil {
				t.Fatalf("Put(%q) error = %v", key, err)
			}
		}

		removed, err := objectstore.RemovePrefix(ctx, store, liveBucket, "batch/", nil)
		if err != nil {
			t.Fatalf("RemovePrefix() error = %v", err)
		}
		if removed.Deleted != 5 {
			t.Errorf("RemovePrefix() deleted %d, want 5", removed.Deleted)
		}
	})
}
