// JSON-RPC 2.0: several requests in flight, answered out of order, matched by
// id through a map of waiting channels. Run: go run .
package main

import (
	"encoding/json"
	"fmt"
)

type Msg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Client keeps one waiting channel per id still in flight.
type Client struct {
	pending map[string]chan Msg
}

func key(id any) string { return fmt.Sprint(id) }

func (c *Client) send(id any, method string) (chan Msg, string) {
	ch := make(chan Msg, 1)
	c.pending[key(id)] = ch
	b, _ := json.Marshal(Msg{JSONRPC: "2.0", ID: id, Method: method})
	return ch, string(b)
}

// receive routes one incoming line by its shape.
func (c *Client) receive(line string, answer func(Msg) string) string {
	var m Msg
	json.Unmarshal([]byte(line), &m)
	switch {
	case m.Method != "" && m.ID != nil: // the server asks too
		return "request from the server, reply: " + answer(m)
	case m.Method != "": // no id: a notification, nobody replies
		return "notification, no reply"
	}
	ch, ok := c.pending[key(m.ID)]
	if !ok {
		return "unknown id " + key(m.ID)
	}
	delete(c.pending, key(m.ID))
	ch <- m
	return "matched id " + key(m.ID)
}

// handle is the server side of one method.
func handle(id any, method string) string {
	var r Msg
	switch method {
	case "tools/call":
		r = Msg{"2.0", id, "", json.RawMessage(`{"content":"rows"}`), nil}
	case "tools/list":
		r = Msg{"2.0", id, "", json.RawMessage(`{"tools":["graph_query"]}`), nil}
	case "prompts/list":
		r = Msg{"2.0", id, "", json.RawMessage(`{"prompts":["trace-funds"]}`), nil}
	default:
		r = Msg{JSONRPC: "2.0", ID: id, Error: &RPCError{-32601, "method not found"}}
	}
	b, _ := json.Marshal(r)
	return string(b)
}

func main() {
	c := &Client{pending: map[string]chan Msg{}}
	ch7, l7 := c.send(7, "tools/call")
	ch8, l8 := c.send(8, "tools/list")
	ch9, l9 := c.send(9, "prompts/list")
	fmt.Println("->", l7, "\n->", l8, "\n->", l9)
	answer := func(m Msg) string { return fmt.Sprintf(`{"id":%q,"result":{...}}`, m.ID) }

	for _, id := range []int{9, 8, 7} { // the fast ones finish first
		method := map[int]string{7: "tools/call", 8: "tools/list", 9: "prompts/list"}[id]
		l := handle(id, method)
		fmt.Println("<-", l, "=>", c.receive(l, answer))
	}
	fmt.Println("id 7 caller got:", string((<-ch7).Result))
	fmt.Println("id 8 caller got:", string((<-ch8).Result))
	fmt.Println("id 9 caller got:", string((<-ch9).Result))

	ch10, _ := c.send(10, "tools/run")
	l := handle(10, "tools/run")
	fmt.Println("<-", l, "=>", c.receive(l, answer), "code", (<-ch10).Error.Code)
	fmt.Println("->", c.receive(`{"jsonrpc":"2.0","method":"notifications/cancelled"}`, answer))
	fmt.Println("<-", c.receive(`{"jsonrpc":"2.0","id":"s1","method":"sampling/createMessage"}`, answer))

	// (bug) trusting the order: the first answer goes to the first question.
	first := handle(9, "prompts/list")
	fmt.Println("buggy client gives id 7's caller:", first)
}
