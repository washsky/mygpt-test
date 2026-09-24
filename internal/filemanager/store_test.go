package filemanager

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestLocalStoreSaveListAssociateRecycleRestoreAndPurge(t *testing.T) {
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
		t.Fatalf("Get after moving to trash error = %v, want ErrNotFound", err)
	}
	files, err = store.List()
	if err != nil || len(files) != 0 { t.Fatalf("active list after delete = %d files, err = %v",len(files),err) }
	trashed, err := store.Trash()
	if err != nil || len(trashed) != 1 || trashed[0].ID != record.ID { t.Fatalf("trash = %+v, err = %v",trashed,err) }
	if _, err := store.Restore(record.ID); err != nil { t.Fatal(err) }
	if _, err := store.Get(record.ID); err != nil { t.Fatalf("Get after restore error = %v",err) }
	if err := store.Delete(record.ID); err != nil { t.Fatal(err) }
	if err := store.Purge(record.ID); err != nil { t.Fatal(err) }
	if trashed, err = store.Trash(); err != nil || len(trashed) != 0 { t.Fatalf("trash after purge = %+v, err = %v",trashed,err) }
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
