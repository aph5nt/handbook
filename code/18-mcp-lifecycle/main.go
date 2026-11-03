// One MCP connection from start to end over stdio, and a local proxy with a
// cache and a packaged fallback. No network: io.Pipe is the stdio pipe. Run: go run .
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type Msg struct {
	ID     *int            `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
}

// server is the graph server: one line in, zero or more lines out.
func server(in io.Reader, out io.Writer, debugOnStdout bool) {
	initialized := false
	reply := func(id *int, result string) {
		fmt.Fprintf(out, `{"jsonrpc":"2.0","id":%d,"result":%s}`+"\n", *id, result)
	}
	sc := bufio.NewScanner(in)
	for sc.Scan() { // ends when the client closes stdin: that is shutdown
		var m Msg
		json.Unmarshal(sc.Bytes(), &m)
		switch {
		case m.Method == "initialize":
			if debugOnStdout {
				fmt.Fprintln(out, "DEBUG connected to db") // (bug) stdout is the wire
			}
			reply(m.ID, `{"protocolVersion":"2025-06-18","capabilities":{"tools":{"listChanged":true},"resources":{}},"serverInfo":{"name":"graph-server"},"instructions":"Anchor every search on one address."}`)
		case m.Method == "notifications/initialized": // no id, no reply
			initialized = true
		case !initialized:
			fmt.Fprintf(out, `{"jsonrpc":"2.0","id":%d,"error":{"code":-32002,"message":"not initialized"}}`+"\n", *m.ID)
		case m.Method == "tools/list":
			reply(m.ID, `{"tools":[{"name":"graph_query","description":"Read the graph. Anchor on one address. Write every address in full.","inputSchema":{"type":"object","properties":{"query":{"type":"string"},"network":{"type":"string"}},"required":["query","network"]},"annotations":{"readOnlyHint":true}}]}`)
		case m.Method == "tools/call":
			fmt.Fprintln(out, `{"jsonrpc":"2.0","method":"notifications/progress","params":{"progressToken":"p3","progress":3,"total":10}}`)
			reply(m.ID, `{"content":[{"type":"text","text":"[{\"to\":\"0x9f...\",\"amount\":2.1}]"},{"type":"text","text":"data is 40 blocks behind the head"}],"isError":false}`)
		case m.Method == "resources/read":
			reply(m.ID, `{"contents":[{"uri":"schema://robinhood","text":"labels: Address"}]}`)
			fmt.Fprintln(out, `{"jsonrpc":"2.0","method":"notifications/tools/list_changed"}`)
		}
	}
	fmt.Println("  server: stdin closed, cleaning up, exit")
}

// client sends each line, prints what comes back and stops on bad JSON.
func client(lines []string, debug bool) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan struct{})
	go func() { server(inR, outW, debug); outW.Close(); close(done) }()
	got := make(chan string, 16)
	go func() {
		rd := bufio.NewScanner(outR)
		for rd.Scan() {
			if !json.Valid(rd.Bytes()) {
				got <- "parse error: " + rd.Text() + " -> connection dropped, no tools"
				continue
			}
			got <- "<- " + rd.Text()
		}
		close(got)
	}()
	for _, l := range lines {
		fmt.Println("  ->", l)
		fmt.Fprintln(inW, l)
		for i := 0; i < expected(l); i++ {
			fmt.Println("  ", <-got)
		}
	}
	inW.Close() // shutdown: there is no shutdown message
	<-done
}

// expected counts the lines the server writes for one client line.
func expected(line string) int {
	var m Msg
	json.Unmarshal([]byte(line), &m)
	switch {
	case m.ID == nil:
		return 0 // a notification gets no reply
	case m.Method == "tools/call", m.Method == "resources/read":
		return 2 // progress or a list change, plus the reply
	}
	return 1
}

// Proxy: a cache, a run-time fetch and a packaged fallback.
const packaged = `{"Aerodrome":"router 0x8f..2c11, 11 pools"}`

type Proxy struct {
	cache map[string]string
	fetch func() ([]byte, error)
}

func (p *Proxy) dexLookup(name string) string {
	src := "cache hit"
	if p.cache == nil {
		b, err := p.fetch()
		src = "fetched"
		if err != nil { // the repository is down
			b, src = []byte(packaged), "packaged copy"
		}
		json.Unmarshal(b, &p.cache)
	}
	return fmt.Sprintf("%s: %s (%s)", name, p.cache[name], src)
}

// graphQuery forwards to the lane. A refusal stays a result the model can read.
func (p *Proxy) graphQuery(address string) string {
	if address == "" {
		return `{"content":[{"type":"text","text":"anchor_missing: anchor the search on one address"}],"isError":true}`
	}
	return `{"content":[{"type":"text","text":"rows from the lane"}],"isError":false}`
}

func main() {
	fmt.Println("one connection:")
	client([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{"roots":{},"elicitation":{}},"clientInfo":{"name":"claude-code"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"graph_query","arguments":{"network":"robinhood","query":"USE topology MATCH ..."},"_meta":{"progressToken":"p3"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"schema://robinhood"}}`,
	}, false)
	fmt.Println("a log line on stdout:")
	client([]string{`{"jsonrpc":"2.0","id":1,"method":"initialize"}`}, true)

	fmt.Println("proxy:")
	p := &Proxy{fetch: func() ([]byte, error) { return []byte(`{"Aerodrome":"router 0x8f..2c11, 12 pools"}`), nil }}
	fmt.Println(" ", p.dexLookup("Aerodrome"))
	fmt.Println(" ", p.dexLookup("Aerodrome"))
	fmt.Println(" ", p.graphQuery("0x18..cb24"))
	fmt.Println(" ", p.graphQuery(""))
	down := &Proxy{fetch: func() ([]byte, error) { return nil, errors.New("GitHub does not answer") }}
	fmt.Println(" ", down.dexLookup("Aerodrome"))
}
