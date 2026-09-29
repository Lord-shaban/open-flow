package storage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPrivateUploadRetriesSameObjectAndBounds(t *testing.T) {
	var calls atomic.Int32
	key := "open-flow/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000002/00000000-0000-4000-8000-000000000003"
	body := []byte("bounded-fixture")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, key) || r.Header.Get("x-amz-acl") != "" || !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256") {
			t.Error("private signed object request required")
		}
		got, _ := io.ReadAll(r.Body)
		if !bytes.Equal(got, body) {
			t.Error("retried upload lost original bytes")
		}
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`<Error><Code>SlowDown</Code></Error>`))
			return
		}
		w.Header().Set("ETag", `"fixture"`)
	}))
	defer server.Close()
	s, err := New(Config{Endpoint: server.URL, Region: "us-east-1", Bucket: "private", AccessKey: "test-access", SecretKey: "test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Put(context.Background(), key, "image/png", int64(len(body)), bytes.NewReader(body)); err != nil || calls.Load() != 2 {
		t.Fatal("bounded safe upload retry", err, calls.Load())
	}
	for _, bad := range []struct {
		key, mime string
		size      int64
	}{{"https://external/file", "image/png", 10}, {key, "text/html", 10}, {key, "image/png", MaxImageBytes + 1}, {"open-flow/../../outside", "image/png", 10}} {
		if err = s.Put(context.Background(), bad.key, bad.mime, bad.size, bytes.NewReader(body)); err == nil {
			t.Fatal("unsafe upload accepted")
		}
	}
	if calls.Load() != 2 {
		t.Fatal("invalid uploads reached storage")
	}
}
func TestRemoteStorageRequiresHTTPS(t *testing.T) {
	for _, endpoint := range []string{"http://example.com", "https://user:secret@example.com", "https://example.com?token=secret"} {
		if _, err := New(Config{Endpoint: endpoint, Region: "us-east-1", Bucket: "private", AccessKey: "access", SecretKey: "secret"}); err == nil {
			t.Fatal("unsafe endpoint accepted")
		}
	}
}
