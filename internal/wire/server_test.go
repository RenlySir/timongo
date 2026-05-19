package wire

import (
	"bufio"
	"bytes"
	"context"
	"testing"

	ferretwire "github.com/FerretDB/wire"
	"github.com/FerretDB/wire/wirebson"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/RenlySir/timongo/internal/handler"
	"github.com/RenlySir/timongo/internal/storage"
)

func TestHandleOpMsgPing(t *testing.T) {
	h := handler.New(storage.NewMemoryStore())
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
	h := handler.New(storage.NewMemoryStore())
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

func TestHandleOpMsgInsertWithDocumentSequence(t *testing.T) {
	h := handler.New(storage.NewMemoryStore())
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
