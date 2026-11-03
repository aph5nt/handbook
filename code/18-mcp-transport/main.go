// Command mcptransport replays the transport lesson: JSON-RPC 2.0 messages
// over stdio (a pipe pair, as a host and its child process use) and over
// Streamable HTTP (one POST endpoint, plain JSON or an SSE stream).
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
)

// Msg is one JSON-RPC 2.0 message. A request has ID and Method, a response
// has ID and Result or Error, a notification has Method and no ID.
type Msg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// handle answers one message. It returns nil for a notification.
func handle(m Msg) *Msg {
	if m.ID == nil { // a notification: nobody replies
		return nil
	}
	switch m.Method {
	case "initialize":
		return &Msg{JSONRPC: "2.0", ID: m.ID, Result: map[string]any{
			"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{}}}}
	case "tools/list":
		return &Msg{JSONRPC: "2.0", ID: m.ID, Result: map[string]any{"tools": []string{"graph_query"}}}
	case "tools/call":
		var p struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(m.Params, &p) != nil || p.Name == "" {
			return &Msg{JSONRPC: "2.0", ID: m.ID, Error: &RPCError{-32602, "invalid params"}}
		}
		return &Msg{JSONRPC: "2.0", ID: m.ID, Result: map[string]any{"content": "rows"}}
	}
	return &Msg{JSONRPC: "2.0", ID: m.ID, Error: &RPCError{-32601, "method not found"}}
}

// serveStdio is the server's main loop: one JSON line in, one JSON line out.
// Logs go to logw (stderr), never to out (stdout), unless debugOnStdout is set.
func serveStdio(in io.Reader, out, logw io.Writer, debugOnStdout bool) {
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		var m Msg
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			continue
		}
		if debugOnStdout {
			fmt.Fprintln(out, "DEBUG loading graph") // the bug: a log line on the wire
		} else {
			fmt.Fprintln(logw, "DEBUG loading graph") // the fix: stderr
		}
		if r := handle(m); r != nil {
			b, _ := json.Marshal(r)
			fmt.Fprintln(out, string(b))
		}
	}
}

// host plays the client: it writes request lines and reads answer lines.
// A line that is not JSON breaks the connection, as a real host does.
func host(lines []string, debugOnStdout bool) {
	inR, inW := io.Pipe()   // the child's stdin
	outR, outW := io.Pipe() // the child's stdout
	var stderr strings.Builder
	go func() { serveStdio(inR, outW, &stderr, debugOnStdout); outW.Close() }()
	go func() {
		for _, l := range lines {
			fmt.Println("  host -> stdin :", l)
			fmt.Fprintln(inW, l)
		}
		inW.Close()
	}()
	sc := bufio.NewScanner(outR)
	for sc.Scan() {
		var m Msg
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			fmt.Printf("  host <- stdout: %q -> parse error, connection closed\n", sc.Text())
			inR.Close()
			return
		}
		fmt.Println("  host <- stdout:", sc.Text())
	}
	if stderr.Len() > 0 {
		fmt.Printf("  stderr log    : %d lines kept out of the wire\n", strings.Count(stderr.String(), "\n"))
	}
}

// httpHandler is the Streamable HTTP side: one endpoint, POST /mcp.
func httpHandler(legacy bool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		var m Msg
		json.NewDecoder(r.Body).Decode(&m)
		if m.ID == nil { // notification: 202, no body
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if m.Method == "tools/call" && strings.Contains(string(m.Params), "slow") {
			w.Header().Set("Content-Type", "text/event-stream") // several messages
			for _, p := range []string{"40%", "80%"} {
				fmt.Fprintf(w, "data: {\"method\":\"notifications/progress\",\"params\":{\"progress\":%q}}\n\n", p)
			}
			b, _ := json.Marshal(handle(m))
			fmt.Fprintf(w, "data: %s\n\n", b)
			return
		}
		w.Header().Set("Content-Type", "application/json") // one answer
		json.NewEncoder(w).Encode(handle(m))
	})
	if legacy { // the old two-endpoint form, kept for old clients
		mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "event: endpoint\ndata: /messages\n\n")
		})
	}
	return mux
}

func post(h http.Handler, method, path, body string) {
	rec := httptest.NewRecorder() // no network: the handler is called directly
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	h.ServeHTTP(rec, req)
	fmt.Printf("  %s %s %s\n    -> %d %s %s\n", method, path, body, rec.Code,
		rec.Header().Get("Content-Type"), strings.TrimSpace(strings.ReplaceAll(rec.Body.String(), "\n\n", " | ")))
}

func main() {
	msgs := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":42}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`,
	}
	fmt.Println("stdio, logs on stderr (the fix):")
	host(msgs, false)
	fmt.Println("stdio, a debug print on stdout (the failure):")
	host(msgs, true)

	fmt.Println("Streamable HTTP:")
	h := httpHandler(false)
	post(h, "POST", "/mcp", `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"graph_query"}}`)
	post(h, "POST", "/mcp", `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"graph_query","arguments":"slow"}}`)
	post(h, "POST", "/mcp", `{"jsonrpc":"2.0","method":"notifications/cancelled"}`)
	post(h, "GET", "/sse", ``)
	fmt.Println("Legacy endpoint kept:")
	post(httpHandler(true), "GET", "/sse", ``)
}
