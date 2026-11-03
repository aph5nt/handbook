// Canonical data model: one shared Price between sources and consumers.
//
// Each source has one translator into Price. Function calls stand in for the
// broker. The input values and the field counts are this example's.
package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Price is the canonical model: the fields every consumer needs, one meaning each.
type Price struct {
	Token, USD string
	TsMs       int64
	Source     string
}

// fromExchangeA: JSON, time already in milliseconds.
func fromExchangeA(s string) Price {
	var m struct {
		Sym   string `json:"sym"`
		Price string `json:"price"`
		T     int64  `json:"t"`
	}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		panic(err)
	}
	return Price{m.Sym, m.Price, m.T, "exchange-a"}
}

// fromAggregatorB: CSV. msFormat is the one thing its change touches.
func fromAggregatorB(s string, msFormat bool) Price {
	f := strings.Split(s, ",")
	t, err := strconv.ParseInt(f[1], 10, 64)
	if err != nil {
		panic(err)
	}
	if !msFormat {
		t *= 1000 // the source sends seconds
	}
	return Price{f[0], f[2], t, "aggregator-b"}
}

// A consumer reads Price only. No source-specific code.
func oracle(p Price) string {
	return "oracle: " + p.Token + " " + p.USD + " USD at " + strconv.FormatInt(p.TsMs, 10)
}

func alert(p Price) string {
	return "alert: " + p.Token + " from " + p.Source
}

// riskPrice is the risk engine's own model, behind its anti-corruption layer.
type riskPrice struct{ Token, USD string }

// each consumer translates Price into its own model
func toRisk(p Price) riskPrice { return riskPrice{p.Token, p.USD} }

func main() {
	fmt.Println("== Tab 1: point to point ==")
	sources, consumers := 3, 3
	fmt.Printf("%d sources x %d consumers = %d translators\n", sources, consumers, sources*consumers)
	fmt.Printf("source B changes: %d translators change\n", consumers)
	fmt.Printf("a new consumer: %d translators, %d new\n", sources*(consumers+1), sources)

	fmt.Println("== Tab 2: canonical ==")
	fmt.Printf("%d translators + %d readers = %d pieces of code\n", sources, consumers, sources+consumers)

	a := fromExchangeA(`{"sym":"WETH","price":"3012.45","t":1730550000000}`)
	b := fromAggregatorB("WETH,1730550000,3012.46", false)
	fmt.Println(oracle(a))
	fmt.Println(oracle(b))
	b = fromAggregatorB("WETH,1730550000000,3012.46", true)
	fmt.Println("B now sends ms:", oracle(b), "- 1 translator changed, 0 consumers")
	fmt.Println(alert(a), "- a new consumer: 1 reader, 0 translators")
	fmt.Printf("%d translators + %d readers = %d pieces of code\n", sources, consumers+1, sources+consumers+1)

	fmt.Println("== Tab 3: bloat ==")
	type blurred struct {
		Price string // mid price from one source, last trade from another
		Kind  string
	}
	mid, last := blurred{"3012.45", "mid"}, blurred{"3012.46", "last trade"}
	fmt.Printf("same field, two meanings: %s (%s), %s (%s)\n", mid.Price, mid.Kind, last.Price, last.Kind)
	fields := 40
	fmt.Printf("risk engine model: %+v\n", toRisk(Price{"WETH", "3012.45", 0, "exchange-a"}))
	fmt.Printf("in this example: %d fields, 6 teams to ask for one change; cut back to %d\n", fields, 4)
}
