package filemanager

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestLocalStoreSaveListAssociateAndDelete(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	record, err := store.Save("hello.txt", "text/plain", Association{Type: "post", ID: "42"}, bytes.NewBufferString("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if record.Size != 5 || record.SHA256 == "" || record.ID == "" {
		t.Fatalf("unexpected file metadata: %+v", record)
	}
	got, body, err := store.Open(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	content, readErr := io.ReadAll(body)
	_ = body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(content) != "hello" || got.Name != "hello.txt" {
		t.Fatalf("unexpected file contents or name: %q, %q", content, got.Name)
	}
	updated, err := store.SetAssociation(record.ID, Association{Type: "reply", ID: "r-1"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Association.Type != "reply" || updated.Association.ID != "r-1" {
		t.Fatalf("association not updated: %+v", updated.Association)
	}
	files, err := store.List()
	if err != nil || len(files) != 1 {
		t.Fatalf("list = %d files, err = %v", len(files), err)
	}
	if err := store.Delete(record.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(record.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete error = %v, want ErrNotFound", err)
	}
}

func TestLocalStoreRejectsOversizedUpload(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := io.LimitReader(zeroReader{}, MaxUploadSize+1)
	if _, err := store.Save("large.bin", "application/octet-stream", Association{}, source); err == nil {
		t.Fatal("expected oversized upload to fail")
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
