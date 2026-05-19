package wire

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"

	ferretwire "github.com/FerretDB/wire"
	"github.com/FerretDB/wire/wirebson"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/RenlySir/timongo/internal/handler"
)

// Server accepts MongoDB wire protocol connections.
type Server struct {
	Addr    string
	Handler *handler.Handler
}

// ListenAndServe starts the server until ctx is canceled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		go s.serveConn(ctx, conn)
	}
}

func (s *Server) serveConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	for {
		header, body, err := ferretwire.ReadMessage(r)
		if err != nil {
			if errors.Is(err, ferretwire.ErrZeroRead) {
				return
			}
			return
		}

		var resp ferretwire.MsgBody
		var opCode ferretwire.OpCode
		switch req := body.(type) {
		case *ferretwire.OpMsg:
			opCode = ferretwire.OpCodeMsg
			resp, err = HandleOpMsg(ctx, s.Handler, req)
			if err != nil {
				resp, _ = responseOpMsg(bson.M{"ok": float64(0), "errmsg": err.Error(), "code": int32(2)})
			}
		case *ferretwire.OpQuery:
			opCode = ferretwire.OpCodeReply
			resp, err = HandleOpQuery(ctx, s.Handler, req)
			if err != nil {
				resp, _ = responseOpReply(bson.M{"ok": float64(0), "errmsg": err.Error(), "code": int32(2)})
			}
		default:
			return
		}

		respHeader := responseHeader(header.RequestID, opCode, resp)
		if err = ferretwire.WriteMessage(w, respHeader, resp); err != nil {
			return
		}
		if err = w.Flush(); err != nil {
			return
		}
	}
}

func responseHeader(requestID int32, opCode ferretwire.OpCode, body ferretwire.MsgBody) *ferretwire.MsgHeader {
	return &ferretwire.MsgHeader{
		MessageLength: int32(body.Size() + ferretwire.MsgHeaderLen),
		RequestID:     requestID + 1,
		ResponseTo:    requestID,
		OpCode:        opCode,
	}
}

// HandleOpMsg dispatches one OP_MSG request to a command handler.
func HandleOpMsg(ctx context.Context, h *handler.Handler, msg *ferretwire.OpMsg) (*ferretwire.OpMsg, error) {
	_, spec, seq, err := msg.Sections()
	if err != nil {
		return nil, err
	}
	doc, err := spec.DecodeDeep()
	if err != nil {
		return nil, err
	}

	cmd, err := WireDocToBSONMap(doc)
	if err != nil {
		return nil, err
	}
	if len(seq) > 0 {
		docs, err := rawSequenceToBSONArray(seq)
		if err != nil {
			return nil, err
		}
		cmd[sequenceFieldName(cmd)] = docs
	}

	res, err := h.Handle(ctx, cmd)
	if err != nil {
		return nil, err
	}

	return responseOpMsg(res)
}

func rawSequenceToBSONArray(seq []byte) (bson.A, error) {
	var docs bson.A
	for len(seq) > 0 {
		size, err := wirebson.FindRaw(seq)
		if err != nil {
			return nil, err
		}
		raw := wirebson.RawDocument(seq[:size])
		doc, err := raw.DecodeDeep()
		if err != nil {
			return nil, err
		}
		m, err := WireDocToBSONMap(doc)
		if err != nil {
			return nil, err
		}
		docs = append(docs, m)
		seq = seq[size:]
	}

	return docs, nil
}

func sequenceFieldName(cmd bson.M) string {
	if _, ok := cmd["insert"]; ok {
		return "documents"
	}
	if _, ok := cmd["update"]; ok {
		return "updates"
	}
	if _, ok := cmd["delete"]; ok {
		return "deletes"
	}
	return "documents"
}

// HandleOpQuery dispatches one legacy OP_QUERY request to a command handler.
func HandleOpQuery(ctx context.Context, h *handler.Handler, query *ferretwire.OpQuery) (*ferretwire.OpReply, error) {
	doc, err := query.QueryDeep()
	if err != nil {
		return nil, err
	}

	cmd, err := WireDocToBSONMap(doc)
	if err != nil {
		return nil, err
	}

	res, err := h.Handle(ctx, cmd)
	if err != nil {
		return nil, err
	}

	return responseOpReply(res)
}

// WireDocToBSONMap converts a wirebson document into a MongoDB driver BSON map.
func WireDocToBSONMap(doc *wirebson.Document) (bson.M, error) {
	converted, err := wirebson.ToDriver(doc)
	if err != nil {
		return nil, err
	}

	return normalizeDriverDoc(converted)
}

func responseOpMsg(doc bson.M) (*ferretwire.OpMsg, error) {
	converted, err := wirebson.FromDriver(toBSOND(doc))
	if err != nil {
		return nil, err
	}

	wireDoc, ok := converted.(*wirebson.Document)
	if !ok {
		return nil, fmt.Errorf("converted response has type %T", converted)
	}

	return ferretwire.NewOpMsg(wireDoc)
}

func responseOpReply(doc bson.M) (*ferretwire.OpReply, error) {
	converted, err := wirebson.FromDriver(toBSOND(doc))
	if err != nil {
		return nil, err
	}

	wireDoc, ok := converted.(*wirebson.Document)
	if !ok {
		return nil, fmt.Errorf("converted response has type %T", converted)
	}

	return ferretwire.NewOpReply(wireDoc)
}

func toBSOND(doc bson.M) bson.D {
	res := make(bson.D, 0, len(doc))
	for _, key := range []string{"ok", "errmsg", "code", "cursor"} {
		if value, ok := doc[key]; ok {
			res = append(res, bson.E{Key: key, Value: toDriverValue(value)})
		}
	}
	for key, value := range doc {
		if key == "ok" || key == "errmsg" || key == "code" || key == "cursor" {
			continue
		}
		res = append(res, bson.E{Key: key, Value: toDriverValue(value)})
	}

	return res
}

func toDriverValue(value any) any {
	switch v := value.(type) {
	case bson.M:
		return toBSOND(v)
	case bson.A:
		res := make(bson.A, 0, len(v))
		for _, item := range v {
			res = append(res, toDriverValue(item))
		}
		return res
	default:
		return value
	}
}

func normalizeDriverDoc(v any) (bson.M, error) {
	switch doc := v.(type) {
	case bson.D:
		res := make(bson.M, len(doc))
		for _, elem := range doc {
			value, err := normalizeDriverValue(elem.Value)
			if err != nil {
				return nil, err
			}
			res[elem.Key] = value
		}
		return res, nil
	case bson.M:
		res := make(bson.M, len(doc))
		for k, value := range doc {
			normalized, err := normalizeDriverValue(value)
			if err != nil {
				return nil, err
			}
			res[k] = normalized
		}
		return res, nil
	default:
		return nil, fmt.Errorf("document has type %T", v)
	}
}

func normalizeDriverValue(v any) (any, error) {
	switch value := v.(type) {
	case bson.D:
		return normalizeDriverDoc(value)
	case bson.A:
		res := make(bson.A, 0, len(value))
		for _, item := range value {
			normalized, err := normalizeDriverValue(item)
			if err != nil {
				return nil, err
			}
			res = append(res, normalized)
		}
		return res, nil
	default:
		return value, nil
	}
}
