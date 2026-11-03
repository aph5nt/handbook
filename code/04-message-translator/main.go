// Message translator: a CSV price line in, the oracle's JSON out.
//
// A slice stands in for the raw-prices topic. The prices and amounts are this
// example's. math/big.Rat holds an exact decimal, and float64 shows the bug.
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

type oraclePrice struct {
	Token string `json:"token"`
	TsMs  int64  `json:"ts"`
	USD   string `json:"usd"` // exact decimal, carried as text
}

var rawPrices []string // the raw-prices topic: every line, unchanged

// translate parses, converts and returns the target message.
func translate(line string) (oraclePrice, error) {
	rawPrices = append(rawPrices, line)
	f, err := csv.NewReader(strings.NewReader(line)).Read()
	if err != nil {
		return oraclePrice{}, err
	}
	sec, err := strconv.ParseInt(f[1], 10, 64)
	if err != nil {
		return oraclePrice{}, err
	}
	usd, ok := new(big.Rat).SetString(f[2])
	if !ok {
		return oraclePrice{}, fmt.Errorf("bad price %q", f[2])
	}
	return oraclePrice{f[0], sec * 1000, strings.TrimRight(usd.FloatString(18), "0")}, nil
}

func main() {
	fmt.Println("== Tab 1: translate ==")
	p, err := translate(`WETH,1730550000,"3012.45"`)
	if err != nil {
		panic(err)
	}
	out, _ := json.Marshal(p)
	fmt.Println(string(out))
	p, err = translate(`WETH,1730550000,"3012.45",new-column`) // the source adds a column
	if err != nil {
		panic(err)
	}
	out, _ = json.Marshal(p)
	fmt.Println("source adds a column:", string(out))

	fmt.Println("== Tab 2: the float ==")
	const price = "0.000000012345678901"
	const amount = "123456789012345678901" // in the token's smallest unit

	exactAmount, _ := new(big.Int).SetString(amount, 10)
	floatAmount, _ := strconv.ParseFloat(amount, 64)
	viaFloat, _ := big.NewFloat(floatAmount).Int(nil)
	fmt.Println("amount exact:     ", exactAmount)
	fmt.Println("amount via float: ", viaFloat)
	fmt.Println("off by:           ", new(big.Int).Sub(viaFloat, exactAmount))

	exactPrice, _ := new(big.Rat).SetString(price)
	floatPrice, _ := strconv.ParseFloat(price, 64)
	fmt.Println("price exact:      ", exactPrice.FloatString(30))
	fmt.Println("price via float:  ", new(big.Rat).SetFloat64(floatPrice).FloatString(30))

	golden := amount
	fmt.Println("golden case, float:", viaFloat.String() == golden, " exact:", exactAmount.String() == golden)

	fmt.Printf("replay: %d raw lines on raw-prices, translate them again\n", len(rawPrices))
}
