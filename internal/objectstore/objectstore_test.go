package objectstore

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// fake is a service in a map. It is here rather than an httptest server because everything in this
// package is written *over* the port: what these tests are about is the walking, the searching, the
// ranges and the resuming, and none of that is protocol.
type fake struct {
	mu      sync.Mutex
	objects map[string][]byte
	// pageSize cuts the listings small so pagination is exercised by a handful of keys.
	pageSize int
	// opened counts the ranged reads, which is how a resume is checked: a resumed download asks for
	// fewer of them.
	opened int
	// failFirst makes the first `failFirst` reads drop half way, which is what a retry is for.
	failFirst int
	// failAfter lets that many reads through and drops every one after them, which is what an
	// interrupted download looks like: some ranges landed and the rest did not.
	failAfter int
	deleted   []string
	// metadata is what the last WriteMetadata replaced it all with.
	metadata Metadata
}

func newFake(pageSize int) *fake {
	return &fake{objects: map[string][]byte{}, pageSize: pageSize}
}

func (f *fake) put(key string, body []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = body
}

func (f *fake) Buckets(context.Context) ([]Bucket, error) {
	return []Bucket{{Name: "shop"}}, nil
}

func (f *fake) List(_ context.Context, request ListRequest) (Listing, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	keys := make([]string, 0, len(f.objects))
	for key := range f.objects {
		if strings.HasPrefix(key, request.Prefix) {
			keys = append(keys, key)
		}
	}
	sortStrings(keys)

	from := 0
	if request.Token != "" {
		from, _ = strconv.Atoi(request.Token)
	}

	size := f.pageSize
	if size <= 0 {
		size = PageSize
	}

	listing := Listing{}
	folders := map[string]bool{}

	at := from
	for ; at < len(keys) && len(listing.Objects)+len(listing.Folders) < size; at++ {
		key := keys[at]
		rest := strings.TrimPrefix(key, request.Prefix)

		if request.Delimiter != "" && strings.Contains(rest, request.Delimiter) {
			folder := request.Prefix + rest[:strings.Index(rest, request.Delimiter)+1]
			if !folders[folder] {
				folders[folder] = true
				listing.Folders = append(listing.Folders, folder)
			}
			continue
		}

		body := f.objects[key]
		listing.Objects = append(listing.Objects, Object{
			Key:      key,
			Size:     int64(len(body)),
			Modified: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			ETag:     sumOf(body),
		})
	}

	if at < len(keys) {
		listing.Next = strconv.Itoa(at)
	}
	return listing, nil
}

func (f *fake) Head(_ context.Context, _, key string) (Object, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	body, ok := f.objects[key]
	if !ok {
		return Object{}, domain.ErrUnknownObject
	}
	return Object{Key: key, Size: int64(len(body)), ETag: sumOf(body)}, nil
}

func (f *fake) Open(_ context.Context, _, key string, span Span) (io.ReadCloser, error) {
	f.mu.Lock()
	body, ok := f.objects[key]
	f.opened++
	drop := f.failFirst > 0
	if drop {
		f.failFirst--
	}
	if f.failAfter > 0 && f.opened > f.failAfter {
		drop = true
	}
	f.mu.Unlock()

	if !ok {
		return nil, domain.ErrUnknownObject
	}

	if !span.Whole() {
		end := span.Offset + span.Length
		if end > int64(len(body)) {
			end = int64(len(body))
		}
		body = body[span.Offset:end]
	}
	if drop {
		// Half a body and then nothing, which is what a dropped connection looks like from here.
		return io.NopCloser(io.MultiReader(
			bytes.NewReader(body[:len(body)/2]),
			errorReader{},
		)), nil
	}
	return io.NopCloser(bytes.NewReader(body)), nil
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("connection reset") }

func (f *fake) Put(_ context.Context, request PutRequest) error {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return err
	}
	f.put(request.Key, body)
	return nil
}

func (f *fake) CreateUpload(context.Context, string, string, string) (string, error) {
	return "upload", nil
}

func (f *fake) UploadPart(_ context.Context, _, key, _ string, number int, body io.Reader, _ int64) (Part, error) {
	contents, err := io.ReadAll(body)
	if err != nil {
		return Part{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key+".part"+strconv.Itoa(number)] = contents
	return Part{Number: number, ETag: sumOf(contents)}, nil
}

func (f *fake) CompleteUpload(_ context.Context, _, key, _ string, parts []Part) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	whole := []byte{}
	for at := 1; at <= len(parts); at++ {
		name := key + ".part" + strconv.Itoa(at)
		whole = append(whole, f.objects[name]...)
		delete(f.objects, name)
	}
	f.objects[key] = whole
	return nil
}

func (f *fake) AbortUpload(context.Context, string, string, string) error { return nil }

func (f *fake) Delete(_ context.Context, _ string, keys []string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, key := range keys {
		delete(f.objects, key)
		f.deleted = append(f.deleted, key)
	}
	return nil, nil
}

func (f *fake) Copy(_ context.Context, _, from, _, to string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[to] = f.objects[from]
	return nil
}

// The three panes' reads. The fake answers plainly: what they are *for* is one request each and the
// shapes are the adapter's to parse, which `s3`'s own tests and its live run cover.
func (f *fake) ObjectPermissions(_ context.Context, _, key string) (Permissions, error) {
	return Permissions{
		Owner:  "owner",
		Grants: []Permission{{Grantee: "owner", Kind: "CanonicalUser", Allow: "FULL_CONTROL"}},
	}, nil
}

func (f *fake) ObjectTags(context.Context, string, string) ([]Tag, error) {
	return []Tag{{Name: "env", Value: "test"}}, nil
}

func (f *fake) WriteMetadata(_ context.Context, _, key string, meta Metadata) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.objects[key]; !ok {
		return domain.ErrUnknownObject
	}
	f.metadata = meta
	return nil
}

func (f *fake) ObjectVersions(_ context.Context, _, key string, _ int) ([]Version, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	body, ok := f.objects[key]
	if !ok {
		return nil, domain.ErrUnknownObject
	}
	return []Version{{VersionID: "1", Latest: true, Size: int64(len(body)), ETag: sumOf(body)}}, nil
}

func (f *fake) MakeBucket(context.Context, string) error   { return nil }
func (f *fake) RemoveBucket(context.Context, string) error { return nil }
func (f *fake) Presign(context.Context, string, string, time.Duration) (string, error) {
	return "https://example/signed", nil
}
func (f *fake) Close() {}

func sumOf(body []byte) string {
	digest := md5.Sum(body)
	return hex.EncodeToString(digest[:])
}

func sortStrings(values []string) {
	for at := 1; at < len(values); at++ {
		for back := at; back > 0 && values[back] < values[back-1]; back-- {
			values[back], values[back-1] = values[back-1], values[back]
		}
	}
}

// ---------------------------------------------------------------- paths

// A key is a string a stranger chose, so this is the test that matters most in the package: nothing
// a bucket's owner can write may put a file outside the folder the reader picked.
func TestLocalPathRefusesToLeaveTheFolder(t *testing.T) {
	root := t.TempDir()

	for _, key := range []string{
		"../escape.txt",
		"../../escape.txt",
		"a/../../escape.txt",
		"./../escape.txt",
		"..",
		"/",
		"",
	} {
		path, ok := LocalPath(root, "", key)
		if !ok {
			continue
		}
		relative, err := filepath.Rel(root, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			t.Errorf("LocalPath(%q) = %q, which is outside the root", key, path)
		}
	}
}

func TestLocalPathTakesTheBasePrefixOff(t *testing.T) {
	root := t.TempDir()

	path, ok := LocalPath(root, "logs/", "logs/2024/03/a.log")
	if !ok {
		t.Fatal("LocalPath() refused an ordinary key")
	}
	want := filepath.Join(root, "2024", "03", "a.log")
	if path != want {
		t.Errorf("LocalPath() = %q, want %q", path, want)
	}
}

func TestLocalPathMakesAKeyIntoAFileNameThisMachineWillTake(t *testing.T) {
	root := t.TempDir()

	tests := map[string]string{
		`a<b>c.txt`:  "a_b_c.txt",
		`with:colon`: "with_colon",
		"con.log":    "con_.log",
		"trailing. ": "trailing",
	}
	for key, want := range tests {
		path, ok := LocalPath(root, "", key)
		if !ok {
			t.Errorf("LocalPath(%q) refused it", key)
			continue
		}
		if got := filepath.Base(path); got != want {
			t.Errorf("LocalPath(%q) = %q, want %q", key, got, want)
		}
	}
}

// ---------------------------------------------------------------- pruning

func TestPruneToHandsTheServiceWhatItCanAnswer(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		match  domain.TextMatch
		want   string
	}{
		{
			"a prefix match is the whole question",
			"",
			domain.TextMatch{Text: "logs/2024-01", Mode: domain.MatchPrefix},
			"logs/2024-01",
		},
		{
			"a prefix match under a folder",
			"logs/",
			domain.TextMatch{Text: "2024", Mode: domain.MatchPrefix},
			"logs/2024",
		},
		{
			"a wildcard ends the literal",
			"",
			domain.TextMatch{Text: "logs/2024%03", Mode: domain.MatchPrefix},
			"logs/2024",
		},
		{
			"a fragment could be anywhere",
			"logs/",
			domain.TextMatch{Text: "invoice", Mode: domain.MatchContains},
			"logs/",
		},
		{
			"an anchored expression has a literal head",
			"",
			domain.TextMatch{Text: `^logs/2024-0[12]`, Mode: domain.MatchRegex},
			"logs/2024-0",
		},
		{
			"an unanchored expression has none",
			"logs/",
			domain.TextMatch{Text: `[0-9]+\.log`, Mode: domain.MatchRegex},
			"logs/",
		},
		{
			"ignoring case gives up the literal, keys being case sensitive",
			"logs/",
			domain.TextMatch{Text: "2024", Mode: domain.MatchPrefix, CaseInsensitive: true},
			"logs/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PruneTo(tt.prefix, tt.match); got != tt.want {
				t.Errorf("PruneTo() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------- walking and searching

func TestWalkReadsEveryPageAndSkipsFolderMarkers(t *testing.T) {
	store := newFake(2)
	for _, key := range []string{"a/1", "a/2", "a/3", "a/", "b/1"} {
		store.put(key, []byte("x"))
	}
	store.put("a/", nil)

	var keys []string
	walked, err := Walk(context.Background(), store, WalkRequest{Bucket: "shop", Prefix: "a/"}, func(object Object) error {
		keys = append(keys, object.Key)
		return nil
	})
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	if strings.Join(keys, ",") != "a/1,a/2,a/3" {
		t.Errorf("Walk() = %v, want the three objects and not the folder marker", keys)
	}
	if !walked.Whole {
		t.Error("Walk() says it did not see the whole prefix, and it did")
	}
}

func TestWalkStopsAtItsCapAndSaysSo(t *testing.T) {
	store := newFake(2)
	for at := range 10 {
		store.put("a/"+strconv.Itoa(at), []byte("x"))
	}

	seen := 0
	walked, err := Walk(context.Background(), store, WalkRequest{Bucket: "shop", Prefix: "a/", Cap: 5}, func(Object) error {
		seen++
		return nil
	})
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}
	if seen != 5 {
		t.Errorf("Walk() looked at %d keys, want 5", seen)
	}
	if walked.Whole {
		t.Error("Walk() claims to have seen the whole prefix after stopping at its cap")
	}
}

func TestSearchAnswersWithTheMatchesAndWhatItCost(t *testing.T) {
	store := newFake(3)
	for _, key := range []string{
		"logs/2024/one.log", "logs/2024/two.txt", "logs/2025/three.log", "other/four.log",
	} {
		store.put(key, []byte("x"))
	}

	answer, err := Search(context.Background(), store, SearchRequest{
		Bucket: "shop",
		Prefix: "logs/",
		Match:  domain.TextMatch{Text: ".log", Mode: domain.MatchContains},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(answer.Hits) != 2 {
		t.Fatalf("Search() found %d, want 2", len(answer.Hits))
	}
	if answer.Scanned != 3 {
		t.Errorf("Scanned = %d, want the three keys under the prefix", answer.Scanned)
	}
	if !answer.Whole || answer.Full {
		t.Errorf("Whole = %v, Full = %v, want true and false", answer.Whole, answer.Full)
	}
	if answer.Prefix != "logs/" {
		t.Errorf("Prefix = %q, want the folder it was run in", answer.Prefix)
	}
}

// The bounds beside the text are facts the listing already carries, so they cost no request — and
// they have to actually narrow, or they would be three boxes that do nothing.
func TestSearchNarrowsBySizeAndTime(t *testing.T) {
	store := newFake(10)
	store.put("a/small.log", []byte("x"))
	store.put("a/large.log", bytes.Repeat([]byte("x"), 100))

	answer, err := Search(context.Background(), store, SearchRequest{
		Bucket:  "shop",
		Prefix:  "a/",
		Match:   domain.TextMatch{Text: ".log", Mode: domain.MatchContains},
		MinSize: 50,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(answer.Hits) != 1 || answer.Hits[0].Key != "a/large.log" {
		t.Errorf("Search() = %v, want the large one alone", answer.Hits)
	}

	answer, err = Search(context.Background(), store, SearchRequest{
		Bucket: "shop",
		Prefix: "a/",
		Match:  domain.TextMatch{Text: ".log", Mode: domain.MatchContains},
		Since:  time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(answer.Hits) != 0 {
		t.Errorf("Search() = %v, want nothing written after September", answer.Hits)
	}
}

// The case that showed the default was wrong. A key is a **path**, so a file name is at the end of
// it — and the mode a service can narrow is matched from the *start*. Both halves are true and the
// pair of them meant a reader pasting a file name was told their object was not there.
//
// Both are asserted, because the fix was to change which one a box opens in and not to change either:
// the quick mode still cannot find a name, and the pane now says so and offers the other.
func TestAFileNameIsFoundByContainsAndNotByPrefix(t *testing.T) {
	store := newFake(10)
	const key = "private/77/b5867e28-a10f-4db6-b4ff-455ae3cb1e57.docx"
	store.put(key, []byte("a document"))
	store.put("private/77/other.docx", []byte("another"))

	found := func(mode domain.SearchMode, text string) int {
		answer, err := Search(context.Background(), store, SearchRequest{
			Bucket: "shop",
			Match:  domain.TextMatch{Text: text, Mode: mode},
		})
		if err != nil {
			t.Fatalf("Search(%q, %q) error = %v", mode, text, err)
		}
		return len(answer.Hits)
	}

	// The whole key finds it either way, which is what a reader who pastes one gets.
	if got := found(domain.MatchPrefix, key); got != 1 {
		t.Errorf("the whole key by prefix found %d, want 1", got)
	}
	if got := found(domain.MatchContains, key); got != 1 {
		t.Errorf("the whole key by contains found %d, want 1", got)
	}

	// The **name alone** is the one that parted company, and it is why Contains is the default.
	for _, text := range []string{
		"b5867e28-a10f-4db6-b4ff-455ae3cb1e57.docx",
		"b5867e28-a10f-4db6-b4ff-455ae3cb1e57",
	} {
		if got := found(domain.MatchContains, text); got != 1 {
			t.Errorf("%q by contains found %d, want 1", text, got)
		}
		if got := found(domain.MatchPrefix, text); got != 0 {
			t.Errorf("%q by prefix found %d, want 0 — a key beginning `private/` begins with no file name", text, got)
		}
	}
}

// The other half of that trade, said as a number: the quick mode reads only what the service handed
// back, and the one that finds a name reads the folder. It is the whole reason both exist.
func TestWhatEachModeCostsToRead(t *testing.T) {
	store := newFake(10)
	for at := range 20 {
		store.put("private/77/file-"+strconv.Itoa(at)+".docx", []byte("x"))
	}

	quick, err := Search(context.Background(), store, SearchRequest{
		Bucket: "shop",
		Match:  domain.TextMatch{Text: "private/77/file-1.docx", Mode: domain.MatchPrefix},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if quick.Scanned != 1 || quick.Prefix != "private/77/file-1.docx" {
		t.Errorf("the prefix search read %d keys under %q, want 1 under the literal itself", quick.Scanned, quick.Prefix)
	}

	whole, err := Search(context.Background(), store, SearchRequest{
		Bucket: "shop",
		Match:  domain.TextMatch{Text: "file-1.docx", Mode: domain.MatchContains},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if whole.Scanned != 20 || whole.Prefix != "" {
		t.Errorf("the contains search read %d keys under %q, want all 20 and no pruning", whole.Scanned, whole.Prefix)
	}
	if len(whole.Hits) != 1 {
		t.Errorf("the contains search found %d, want the one file", len(whole.Hits))
	}
}

func TestSearchSaysWhenThereAreMoreOfThem(t *testing.T) {
	store := newFake(10)
	for at := range 10 {
		store.put("a/"+strconv.Itoa(at)+".log", []byte("x"))
	}

	answer, err := Search(context.Background(), store, SearchRequest{
		Bucket: "shop",
		Match:  domain.TextMatch{Text: ".log", Mode: domain.MatchContains},
		Hits:   3,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(answer.Hits) != 3 || !answer.Full || answer.Whole {
		t.Errorf("Search() = %d hits, Full = %v, Whole = %v; want 3, true, false",
			len(answer.Hits), answer.Full, answer.Whole)
	}
}

// ---------------------------------------------------------------- downloading

func downloaded(t *testing.T, store Store, plan DownloadPlan) (Downloaded, []byte) {
	t.Helper()

	done, err := Download(context.Background(), store, plan, nil)
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	contents, err := os.ReadFile(done.Path)
	if err != nil {
		t.Fatalf("reading what was written: %v", err)
	}
	return done, contents
}

func TestDownloadStreamsASmallObjectAndVerifiesIt(t *testing.T) {
	store := newFake(10)
	body := []byte("a small object")
	store.put("a.txt", body)

	path := filepath.Join(t.TempDir(), "a.txt")
	done, contents := downloaded(t, store, DownloadPlan{
		Bucket: "shop", Key: "a.txt", Path: path,
		Size: int64(len(body)), ETag: sumOf(body),
	})

	if !bytes.Equal(contents, body) {
		t.Errorf("the file holds %q, want %q", contents, body)
	}
	if !done.Verified {
		t.Error("Verified = false, and the tag was a plain MD5 this download could check")
	}
	if _, err := os.Stat(path + partialSuffix); !errors.Is(err, os.ErrNotExist) {
		t.Error("the partial file was left behind")
	}
}

// An object the service wrote in parts has a tag that is not a checksum, so a download of it says
// `Verified` false rather than claiming a check it did not make.
func TestDownloadDoesNotClaimToHaveVerifiedAMultipartTag(t *testing.T) {
	store := newFake(10)
	body := []byte("written in parts")
	store.put("a.bin", body)

	done, _ := downloaded(t, store, DownloadPlan{
		Bucket: "shop", Key: "a.bin", Path: filepath.Join(t.TempDir(), "a.bin"),
		Size: int64(len(body)), ETag: sumOf(body) + "-3",
	})
	if done.Verified {
		t.Error("Verified = true for a multipart tag, which is not a checksum of anything")
	}
}

func TestDownloadRefusesBytesThatDoNotMatchTheChecksum(t *testing.T) {
	store := newFake(10)
	store.put("a.txt", []byte("the real bytes"))

	path := filepath.Join(t.TempDir(), "a.txt")
	_, err := Download(context.Background(), store, DownloadPlan{
		Bucket: "shop", Key: "a.txt", Path: path,
		Size: 14, ETag: sumOf([]byte("something else")),
	}, nil)

	if !errors.Is(err, domain.ErrFileUnreadable) {
		t.Fatalf("Download() error = %v, want it to refuse the bytes", err)
	}
	// A file that failed its checksum must not be left where something else would open it.
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Error("the file that failed its checksum was left in place")
	}
}

// The parallel path, checked the only way that matters: the ranges have to be reassembled in the
// right order, or a large download is quietly corrupt.
func TestDownloadReadsALargeObjectInRangesAndPutsItBackTogether(t *testing.T) {
	store := newFake(10)
	body := make([]byte, PartThreshold+1000)
	for at := range body {
		body[at] = byte(at % 251)
	}
	store.put("big.bin", body)

	path := filepath.Join(t.TempDir(), "big.bin")
	moved := int64(0)
	var guard sync.Mutex

	done, err := Download(context.Background(), store, DownloadPlan{
		Bucket: "shop", Key: "big.bin", Path: path,
		Size: int64(len(body)), ETag: sumOf(body),
		PartSize: 4 << 20, Parts: 4,
	}, func(delta int64) {
		guard.Lock()
		moved += delta
		guard.Unlock()
	})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading it back: %v", err)
	}
	if !bytes.Equal(contents, body) {
		t.Error("the ranges were not put back together in order")
	}
	if done.Bytes != int64(len(body)) || moved != int64(len(body)) {
		t.Errorf("Bytes = %d and progress = %d, want %d for both", done.Bytes, moved, len(body))
	}
	if store.opened < 2 {
		t.Errorf("it made %d ranged reads, want several", store.opened)
	}
}

// The whole point of the sidecar: a download interrupted part way through asks the service for what
// is left and not for the object again.
func TestDownloadCarriesOnFromWhereItStopped(t *testing.T) {
	store := newFake(10)
	body := make([]byte, PartThreshold+1000)
	for at := range body {
		body[at] = byte(at % 251)
	}
	store.put("big.bin", body)

	path := filepath.Join(t.TempDir(), "big.bin")
	plan := DownloadPlan{
		Bucket: "shop", Key: "big.bin", Path: path,
		Size: int64(len(body)), ETag: sumOf(body),
		PartSize: 4 << 20, Parts: 1,
	}

	// The first attempt gets two of the five ranges down and then the connection stops answering,
	// which is exactly what a lid closed on a laptop looks like from here.
	store.failAfter = 2
	if _, err := Download(context.Background(), store, plan, nil); err == nil {
		t.Fatal("Download() succeeded against a service that stopped answering")
	}
	if _, err := os.Stat(path + planSuffix); err != nil {
		t.Fatalf("no resume plan was left beside the partial file: %v", err)
	}

	store.failAfter = 0
	store.opened = 0
	done, err := Download(context.Background(), store, plan, nil)
	if err != nil {
		t.Fatalf("Download() error on the second attempt = %v", err)
	}
	if !done.Resumed {
		t.Error("Resumed = false, and two of the five ranges were already down")
	}
	// The whole point: the ranges that landed are not asked for again.
	if store.opened != 3 {
		t.Errorf("the second attempt made %d ranged reads, want the 3 that were left", store.opened)
	}

	contents, err := os.ReadFile(done.Path)
	if err != nil {
		t.Fatalf("reading it back: %v", err)
	}
	if !bytes.Equal(contents, body) {
		t.Error("a resumed download did not produce the object")
	}
	if _, err := os.Stat(path + planSuffix); !errors.Is(err, os.ErrNotExist) {
		t.Error("the resume plan was left behind after the download finished")
	}
}

// An object rewritten under the same key is a different object, so a plan whose tag no longer
// matches is thrown away rather than resumed into a file that would be half of each.
func TestAResumePlanForAnotherObjectIsThrownAway(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "big.bin")

	store := newFake(10)
	body := make([]byte, PartThreshold+10)
	store.put("big.bin", body)

	writePlan(path+planSuffix, "a-tag-from-another-object", int64(len(body)), 4<<20, []bool{true, true, true})

	done, contents := downloaded(t, store, DownloadPlan{
		Bucket: "shop", Key: "big.bin", Path: path,
		Size: int64(len(body)), ETag: sumOf(body), PartSize: 4 << 20,
	})
	if done.Resumed {
		t.Error("Resumed = true, and the plan was for a different object")
	}
	if !bytes.Equal(contents, body) {
		t.Error("the file is not the object")
	}
}

func TestCollisionIsTheReadersRule(t *testing.T) {
	store := newFake(10)
	store.put("a.txt", []byte("from the bucket"))

	t.Run("the default leaves the file alone", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "a.txt")
		os.WriteFile(path, []byte("mine"), 0o600)

		done, contents := downloaded(t, store, DownloadPlan{Bucket: "shop", Key: "a.txt", Path: path, Size: 15})
		if !done.Skipped || string(contents) != "mine" {
			t.Errorf("Skipped = %v and the file holds %q, want true and the file untouched", done.Skipped, contents)
		}
	})

	t.Run("overwrite replaces it", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "a.txt")
		os.WriteFile(path, []byte("mine"), 0o600)

		_, contents := downloaded(t, store, DownloadPlan{
			Bucket: "shop", Key: "a.txt", Path: path, Size: 15, Collision: CollisionOverwrite,
		})
		if string(contents) != "from the bucket" {
			t.Errorf("the file holds %q, want the object", contents)
		}
	})

	t.Run("rename writes beside it", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "a.txt")
		os.WriteFile(path, []byte("mine"), 0o600)

		done, contents := downloaded(t, store, DownloadPlan{
			Bucket: "shop", Key: "a.txt", Path: path, Size: 15, Collision: CollisionRename,
		})
		if done.Path == path {
			t.Error("it wrote over the file it was told to write beside")
		}
		if filepath.Base(done.Path) != "a (2).txt" {
			t.Errorf("it wrote %q, want %q", filepath.Base(done.Path), "a (2).txt")
		}
		if string(contents) != "from the bucket" {
			t.Errorf("the copy holds %q, want the object", contents)
		}
	})

	t.Run("newer is what makes a second run a sync", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "a.txt")
		os.WriteFile(path, []byte("mine"), 0o600)

		// The file is from now, so an object written last month is not newer than it.
		done, _ := downloaded(t, store, DownloadPlan{
			Bucket: "shop", Key: "a.txt", Path: path, Size: 15,
			Modified: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC).Unix(), Collision: CollisionNewer,
		})
		if !done.Skipped {
			t.Error("Skipped = false for an object older than the file")
		}

		done, contents := downloaded(t, store, DownloadPlan{
			Bucket: "shop", Key: "a.txt", Path: path, Size: 15,
			Modified: time.Now().Add(time.Hour).Unix(), Collision: CollisionNewer,
		})
		if done.Skipped || string(contents) != "from the bucket" {
			t.Errorf("Skipped = %v and the file holds %q, want false and the object", done.Skipped, contents)
		}
	})
}

func TestDownloadMakesTheFolderItWritesInto(t *testing.T) {
	store := newFake(10)
	store.put("logs/2024/a.log", []byte("x"))

	root := t.TempDir()
	path := filepath.Join(root, "2024", "a.log")
	if _, contents := downloaded(t, store, DownloadPlan{
		Bucket: "shop", Key: "logs/2024/a.log", Path: path, Size: 1,
	}); string(contents) != "x" {
		t.Errorf("the file holds %q, want the object", contents)
	}
}

// ---------------------------------------------------------------- uploading

func TestUploadWritesASmallFileInOneRequest(t *testing.T) {
	store := newFake(10)
	path := filepath.Join(t.TempDir(), "a.csv")
	os.WriteFile(path, []byte("one,two\n"), 0o600)

	done, err := Upload(context.Background(), store, UploadPlan{Bucket: "shop", Key: "a.csv", Path: path}, nil)
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if done.Multipart {
		t.Error("Multipart = true for eight bytes")
	}
	if string(store.objects["a.csv"]) != "one,two\n" {
		t.Errorf("the object holds %q", store.objects["a.csv"])
	}
}

func TestUploadWritesALargeFileInParts(t *testing.T) {
	store := newFake(10)
	body := make([]byte, PartThreshold+500)
	for at := range body {
		body[at] = byte(at % 251)
	}
	path := filepath.Join(t.TempDir(), "big.bin")
	os.WriteFile(path, body, 0o600)

	done, err := Upload(context.Background(), store, UploadPlan{
		Bucket: "shop", Key: "big.bin", Path: path, PartSize: 4 << 20, Parts: 4,
	}, nil)
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if !done.Multipart {
		t.Error("Multipart = false for a file over the threshold")
	}
	if !bytes.Equal(store.objects["big.bin"], body) {
		t.Error("the parts were not put back together in order")
	}
}

func TestContentTypeIsReadOffTheName(t *testing.T) {
	// The machine's own table answers most of these, so what is checked is that a type comes back at
	// all and that the fallback is the one a service serves as a download.
	if got := ContentTypeOf("a/b/report.csv"); !strings.HasPrefix(got, "text/csv") {
		t.Errorf("ContentTypeOf(.csv) = %q, want text/csv", got)
	}
	if got := ContentTypeOf("dump.sql"); got == "" {
		t.Error("ContentTypeOf(.sql) is empty")
	}
	if got := ContentTypeOf("no-extension"); got != "application/octet-stream" {
		t.Errorf("ContentTypeOf() = %q, want application/octet-stream", got)
	}
}

// ---------------------------------------------------------------- folders

func TestRemovePrefixDeletesEverythingUnderIt(t *testing.T) {
	store := newFake(2)
	for _, key := range []string{"a/1", "a/2", "a/3", "b/1"} {
		store.put(key, []byte("x"))
	}

	removed, err := RemovePrefix(context.Background(), store, "shop", "a/", nil)
	if err != nil {
		t.Fatalf("RemovePrefix() error = %v", err)
	}
	if removed.Deleted != 3 {
		t.Errorf("Deleted = %d, want 3", removed.Deleted)
	}
	if _, still := store.objects["b/1"]; !still {
		t.Error("it deleted something outside the prefix")
	}
}

// A folder copied into itself would find its own output and go round for ever, and a walk cannot
// notice — so it is refused before it starts.
func TestCopyPrefixRefusesToCopyIntoItself(t *testing.T) {
	store := newFake(10)
	store.put("a/1", []byte("x"))

	if _, err := CopyPrefix(context.Background(), store, "shop", "a/", "shop", "a/deeper/", nil); !errors.Is(err, domain.ErrUnsupported) {
		t.Errorf("CopyPrefix() error = %v, want ErrUnsupported", err)
	}
}

func TestCopyPrefixMovesTheKeysWithoutTheBytesComingHere(t *testing.T) {
	store := newFake(2)
	store.put("a/1", []byte("x"))
	store.put("a/deep/2", []byte("y"))

	copied, err := CopyPrefix(context.Background(), store, "shop", "a/", "shop", "b/", nil)
	if err != nil {
		t.Fatalf("CopyPrefix() error = %v", err)
	}
	if copied.Objects != 2 {
		t.Errorf("Objects = %d, want 2", copied.Objects)
	}
	if _, ok := store.objects["b/deep/2"]; !ok {
		t.Error("the shape under the prefix was not kept")
	}
}

// ---------------------------------------------------------------- the queue

func TestQueueRunsAJobAndReportsWhatItDid(t *testing.T) {
	store := newFake(10)
	for at := range 5 {
		store.put("a/"+strconv.Itoa(at), []byte("body"))
	}

	root := t.TempDir()
	queue := NewQueue(2)
	defer queue.Close()

	queue.Submit(context.Background(), store, Job{
		ID:     "job-1",
		Kind:   KindDownload,
		Bucket: "shop",
		Label:  "a/",
		Root:   root,
		Each: func(ctx context.Context, yield func(Item) error) error {
			_, err := Walk(ctx, store, WalkRequest{Bucket: "shop", Prefix: "a/"}, func(object Object) error {
				path, ok := LocalPath(root, "a/", object.Key)
				if !ok {
					return nil
				}
				return yield(Item{Key: object.Key, Path: path, Size: object.Size, ETag: object.ETag})
			})
			return err
		},
	})

	report := until(t, queue, func(r Report) bool {
		return len(r.Jobs) == 1 && r.Jobs[0].State.Finished()
	})

	job := report.Jobs[0]
	if job.State != JobDone {
		t.Fatalf("State = %q with detail %q, want done", job.State, job.Detail)
	}
	if job.Done != 5 || job.Files != 5 {
		t.Errorf("Done = %d of %d, want 5 of 5", job.Done, job.Files)
	}
	if job.Moved != job.Bytes {
		t.Errorf("Moved = %d of %d, want them equal", job.Moved, job.Bytes)
	}
	if job.Counting {
		t.Error("Counting = true on a job that has finished")
	}
}

// A skipped file still counts towards what was moved, or the bar under a second run of the same
// folder would never fill.
func TestASkippedFileStillFillsTheBar(t *testing.T) {
	store := newFake(10)
	store.put("a/1", []byte("body"))

	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "1"), []byte("mine"), 0o600)

	queue := NewQueue(1)
	defer queue.Close()

	queue.Submit(context.Background(), store, Job{
		ID: "job-1", Kind: KindDownload, Bucket: "shop", Root: root,
		Each: func(_ context.Context, yield func(Item) error) error {
			return yield(Item{Key: "a/1", Path: filepath.Join(root, "1"), Size: 4})
		},
	})

	report := until(t, queue, func(r Report) bool { return len(r.Jobs) == 1 && r.Jobs[0].State.Finished() })
	job := report.Jobs[0]

	if job.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", job.Skipped)
	}
	if job.Moved != job.Bytes {
		t.Errorf("Moved = %d of %d, want them equal", job.Moved, job.Bytes)
	}
}

func TestAJobThatWasStoppedSaysSo(t *testing.T) {
	store := newFake(10)
	store.put("a/1", []byte("body"))

	queue := NewQueue(1)
	defer queue.Close()

	blocked := make(chan struct{})
	queue.Submit(context.Background(), store, Job{
		ID: "job-1", Kind: KindDownload, Bucket: "shop", Root: t.TempDir(),
		Each: func(ctx context.Context, _ func(Item) error) error {
			close(blocked)
			<-ctx.Done()
			return ctx.Err()
		},
	})

	<-blocked
	queue.Cancel("job-1")

	report := until(t, queue, func(r Report) bool { return len(r.Jobs) == 1 && r.Jobs[0].State.Finished() })
	if report.Jobs[0].State != JobCancelled {
		t.Errorf("State = %q, want cancelled", report.Jobs[0].State)
	}
}

// until waits for the queue to say something, because a transfer happens on its own goroutines and
// there is nothing to join.
func until(t *testing.T, queue *Queue, ready func(Report) bool) Report {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for {
		report := queue.Report()
		if ready(report) {
			return report
		}
		if time.Now().After(deadline) {
			t.Fatalf("the queue never got there: %+v", report)
		}
		runtime.Gosched()
		time.Sleep(5 * time.Millisecond)
	}
}
