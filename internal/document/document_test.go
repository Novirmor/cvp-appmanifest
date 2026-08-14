package document

import (
	"strings"
	"testing"
)

func TestDecodeValid(t *testing.T) {
	doc, err := Decode([]byte(`
apiVersion: appmanifest.mgconsulting.io/v1alpha1
name: app
port: 80
`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc["name"] != "app" {
		t.Fatalf("name = %v", doc["name"])
	}
}

func TestDecodeMultiDocumentRejected(t *testing.T) {
	_, err := Decode([]byte("a: 1\n---\nb: 2\n"))
	if err == nil {
		t.Fatal("expected error for multi-document input")
	}
}

func TestDecodeDuplicateKeysRejected(t *testing.T) {
	_, err := Decode([]byte("a: 1\na: 2\n"))
	if err == nil || !IsDuplicateKeyError(err) {
		t.Fatalf("expected duplicate-key error, got %v", err)
	}
}

func TestDecodeNonStringKeyRejected(t *testing.T) {
	_, err := Decode([]byte("1: value\n"))
	if err == nil {
		t.Fatal("expected error for non-string key")
	}
}

func TestDecodeAliasRejected(t *testing.T) {
	_, err := Decode([]byte("a: &x 1\nb: *x\n"))
	if err == nil {
		t.Fatal("expected error for alias")
	}
}

func TestDecodeMergeKeyRejected(t *testing.T) {
	_, err := Decode([]byte("base: &b {x: 1}\nchild:\n  <<: *b\n"))
	if err == nil {
		t.Fatal("expected error for merge key")
	}
}

func TestDecodeTimestampRejected(t *testing.T) {
	_, err := Decode([]byte("when: 2026-08-14T10:00:00Z\n"))
	if err == nil || !strings.Contains(err.Error(), "timestamp") {
		t.Fatalf("expected timestamp rejection, got %v", err)
	}
}

func TestDecodeCustomTagRejected(t *testing.T) {
	_, err := Decode([]byte("a: !custom value\n"))
	if err == nil {
		t.Fatal("expected error for custom tag")
	}
}

func TestDecodeNumericScalarKeptAsNumber(t *testing.T) {
	doc, err := Decode([]byte("port: 8080\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc["port"] != int64(8080) {
		t.Fatalf("port = %v", doc["port"])
	}
}

func TestDecodePathInError(t *testing.T) {
	_, err := Decode([]byte("a:\n  b:\n    c: !x 1\n"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "$.a.b.c") {
		t.Fatalf("error should carry path, got: %v", err)
	}
}
