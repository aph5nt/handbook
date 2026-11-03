// The tool result: a failure the model can fix is a result with isError,
// not a JSON-RPC error. Run: go run .
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

type Block struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
type Result struct {
	Content []Block `json:"content"`
	IsError bool    `json:"isError"`
}
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
type Reply struct {
	ID     int       `json:"id"`
	Result *Result   `json:"result,omitempty"`
	Error  *RPCError `json:"error,omitempty"`
}

func text(s string) Block { return Block{"text", s} }

// callTool answers tools/call. wrong=true is the bad server.
func callTool(ctx context.Context, id int, name, address string, wrong bool) Reply {
	if name != "graph_query" { // a protocol fault: the host has a bug
		return Reply{ID: id, Error: &RPCError{-32602, "unknown tool: " + name}}
	}
	if address == "" {
		if wrong { // (bug) the reason never reaches the model
			return Reply{ID: id, Error: &RPCError{-32602, "invalid params"}}
		}
		return Reply{ID: id, Result: &Result{[]Block{text("anchor a search on one address")}, true}}
	}
	if address == "*broad*" {
		select {
		case <-ctx.Done(): // notifications/cancelled reached the statement
			return Reply{ID: id, Result: &Result{[]Block{text("cancelled")}, true}}
		case <-time.After(20 * time.Millisecond): // the database time limit
			return Reply{ID: id, Result: &Result{[]Block{text("timed out, narrow the time window")}, true}}
		}
	}
	rows := text(`[{"to":"0x9f..0071","value":0.8}]`)
	return Reply{ID: id, Result: &Result{[]Block{rows, text("data is 40 blocks behind the head")}, false}}
}

// model reads only the content of a result. An error gives it no reason.
func model(r Reply) string {
	if r.Error != nil {
		return "tool failed (no reason) -> retry the same call"
	}
	if r.Result.IsError {
		return "read \"" + r.Result.Content[0].Text + "\" -> change the call"
	}
	return "answer the user"
}

func show(r Reply) string { b, _ := json.Marshal(r); return string(b) }

// listTools pages with an opaque cursor. The host never parses it.
func listTools(cursor string, all []string) ([]string, string) {
	start := 0
	if cursor != "" {
		b, _ := base64.StdEncoding.DecodeString(cursor)
		fmt.Sscan(string(b), &start)
	}
	end := min(start+2, len(all))
	if end == len(all) {
		return all[start:end], ""
	}
	return all[start:end], base64.StdEncoding.EncodeToString([]byte(fmt.Sprint(end)))
}

func main() {
	ctx := context.Background()
	r := callTool(ctx, 1, "graph_query", "", false)
	fmt.Println("result 2:", show(r), "->", model(r))
	r = callTool(ctx, 2, "graph_query", "0x18..cb24", false)
	fmt.Println("result 5:", show(r), "->", model(r))
	r = callTool(ctx, 3, "graph_query", "*broad*", false)
	fmt.Println("result 6:", show(r), "->", model(r))
	cctx, cancel := context.WithCancel(ctx)
	cancel() // the user pressed stop: notifications/cancelled {requestId: 4}
	fmt.Println("result 7:", show(callTool(cctx, 4, "graph_query", "*broad*", false)))

	r = callTool(ctx, 5, "graph_query", "", true)
	fmt.Println("error 1:", show(r), "->", model(r))
	r = callTool(ctx, 6, "graph_qeury", "0x18..cb24", false)
	fmt.Println("error 4:", show(r))
	all := []string{"graph_query", "graph_query_batch", "meta_schema"}
	page, next := listTools("", all)
	fmt.Println("error 5: page 1", page, "nextCursor", next)
	page, next = listTools(next, all)
	fmt.Printf("error 5: page 2 %v nextCursor %q\n", page, next)
}
