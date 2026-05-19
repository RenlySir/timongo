package wire

import (
	"bufio"
	"bytes"
	"context"
	"testing"

	ferretwire "github.com/FerretDB/wire"
	"github.com/FerretDB/wire/wirebson"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/RenlySir/timongo/internal/backend/memory"
	"github.com/RenlySir/timongo/internal/handler"
)

func TestHandleOpMsgPing(t *testing.T) {
	h := handler.New(memory.NewStore())
	req := ferretwire.MustOpMsg("ping", int32(1), "$db", "admin")

	resp, err := HandleOpMsg(context.Background(), h, req)
	if err != nil {
		t.Fatalf("HandleOpMsg returned error: %v", err)
	}

	doc, err := resp.Document()
	if err != nil {
		t.Fatalf("Document returned error: %v", err)
	}
	if doc.Get("ok") != float64(1) {
		t.Fatalf("ok = %v, want 1", doc.Get("ok"))
	}
}

func TestHandleOpQueryHello(t *testing.T) {
	h := handler.New(memory.NewStore())
	req := ferretwire.MustOpQuery("isMaster", int32(1), "$db", "admin")

	resp, err := HandleOpQuery(context.Background(), h, req)
	if err != nil {
		t.Fatalf("HandleOpQuery returned error: %v", err)
	}

	doc, err := resp.Document()
	if err != nil {
		t.Fatalf("Document returned error: %v", err)
	}
	if doc.Get("ok") != float64(1) {
		t.Fatalf("ok = %v, want 1", doc.Get("ok"))
	}
}

func TestWriteResponseSetsMessageLength(t *testing.T) {
	resp := ferretwire.MustOpMsg("ok", float64(1))
	header := responseHeader(10, ferretwire.OpCodeMsg, resp)

	var buf bytes.Buffer
	err := ferretwire.WriteMessage(bufio.NewWriter(&buf), header, resp)
	if err != nil {
		t.Fatalf("WriteMessage returned error: %v", err)
	}

	if header.MessageLength == 0 {
		t.Fatal("MessageLength was not set")
	}
}

func TestWireDocumentRoundTripToBSONMap(t *testing.T) {
	doc := wirebson.MustDocument("find", "users", "$db", "app", "filter", wirebson.MustDocument("_id", int32(1)))

	got, err := WireDocToBSONMap(doc)
	if err != nil {
		t.Fatalf("WireDocToBSONMap returned error: %v", err)
	}

	filter, ok := got["filter"].(bson.M)
	if !ok {
		t.Fatalf("filter has type %T, want bson.M", got["filter"])
	}
	if filter["_id"] != int32(1) {
		t.Fatalf("filter _id = %v, want 1", filter["_id"])
	}
}

func TestWireDocumentRoundTripWithArray(t *testing.T) {
	doc := wirebson.MustDocument("hello", int32(1), "compression", wirebson.MustArray("snappy", "zstd"))

	got, err := WireDocToBSONMap(doc)
	if err != nil {
		t.Fatalf("WireDocToBSONMap returned error: %v", err)
	}

	compression, ok := got["compression"].(bson.A)
	if !ok {
		t.Fatalf("compression has type %T, want bson.A", got["compression"])
	}
	if len(compression) != 2 {
		t.Fatalf("len(compression) = %d, want 2", len(compression))
	}
}

func TestWireRawDocumentSupportsMinAndMaxKey(t *testing.T) {
	raw, err := bson.Marshal(bson.M{
		"insert": "types",
		"$db":    "app",
		"documents": bson.A{
			bson.M{"_id": int32(1), "kind": "min", "value": bson.MinKey{}},
			bson.M{"_id": int32(2), "kind": "max", "value": bson.MaxKey{}},
		},
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	got, err := rawDocumentToBSONMap(wirebson.RawDocument(raw))
	if err != nil {
		t.Fatalf("rawDocumentToBSONMap returned error: %v", err)
	}
	docs := got["documents"].(bson.A)
	if _, ok := docs[0].(bson.M)["value"].(bson.MinKey); !ok {
		t.Fatalf("first value has type %T, want bson.MinKey", docs[0].(bson.M)["value"])
	}
	if _, ok := docs[1].(bson.M)["value"].(bson.MaxKey); !ok {
		t.Fatalf("second value has type %T, want bson.MaxKey", docs[1].(bson.M)["value"])
	}
}

func TestHandleOpMsgInsertWithDocumentSequence(t *testing.T) {
	h := handler.New(memory.NewStore())
	req := ferretwire.MustOpMsg("insert", "users", "$db", "app", "documents", wirebson.MustArray(
		wirebson.MustDocument("_id", int32(1), "name", "Ada"),
	))

	resp, err := HandleOpMsg(context.Background(), h, req)
	if err != nil {
		t.Fatalf("HandleOpMsg returned error: %v", err)
	}

	doc, err := resp.Document()
	if err != nil {
		t.Fatalf("Document returned error: %v", err)
	}
	if doc.Get("ok") != float64(1) {
		t.Fatalf("ok = %v, want 1", doc.Get("ok"))
	}
}

func TestSequenceFieldNameUsesCommandSpecificField(t *testing.T) {
	tests := []struct {
		name string
		cmd  bson.M
		want string
	}{
		{name: "insert", cmd: bson.M{"insert": "users"}, want: "documents"},
		{name: "update", cmd: bson.M{"update": "users"}, want: "updates"},
		{name: "delete", cmd: bson.M{"delete": "users"}, want: "deletes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sequenceFieldName(tt.cmd); got != tt.want {
				t.Fatalf("sequenceFieldName() = %q, want %q", got, tt.want)
			}
		})
	}
}
