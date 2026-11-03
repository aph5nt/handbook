// Replays the microkernel lesson: a DEX decoder with a small core,
// plug-in adapters and a registry file. A new exchange is one plug-in and
// one row. A registry that names a missing adapter version stops the core,
// and a pre-roll check catches it before the roll.
package main

import (
	"fmt"
	"sort"
)

// Adapter is the plug-in contract: decode one swap log into a swap row.
type Adapter interface {
	Decode(log Log) Swap
}

type Log struct{ Pool, Factory string }
type Swap struct{ Pool, Exchange string }

// RegistryRow names which adapter, at which version, decodes a factory's pools.
type RegistryRow struct {
	Factory, Adapter string
	Version          int
}

// exchange is one plug-in. All four in this demo share one shape.
type exchange struct{ name string }

func (e exchange) Decode(l Log) Swap { return Swap{l.Pool, e.name} }

// Binary is what one release carries: adapter name@version -> plug-in.
type Binary map[string]Adapter

func id(name string, v int) string { return fmt.Sprintf("%s@%d", name, v) }

// Core walks swap logs. It holds no exchange code.
type Core struct{ route map[string]Adapter }

// Start reads the registry and finds each row's adapter in the binary.
func Start(reg []RegistryRow, bin Binary) (*Core, error) {
	c := &Core{route: map[string]Adapter{}}
	for _, r := range reg {
		a, ok := bin[id(r.Adapter, r.Version)]
		if !ok {
			return nil, fmt.Errorf("unknown adapter %s", id(r.Adapter, r.Version))
		}
		c.route[r.Factory] = a
	}
	return c, nil
}

// Walk hands each log to the plug-in of its pool's factory.
func (c *Core) Walk(logs []Log) []Swap {
	var out []Swap
	for _, l := range logs {
		if a, ok := c.route[l.Factory]; ok {
			out = append(out, a.Decode(l))
		}
	}
	return out
}

// PreRoll compares the registry with the adapters inside the new binary.
func PreRoll(reg []RegistryRow, bin Binary) (string, []string) {
	var missing []string
	for _, r := range reg {
		if _, ok := bin[id(r.Adapter, r.Version)]; !ok {
			missing = append(missing, id(r.Adapter, r.Version))
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return "no-go", missing
	}
	return "go", nil
}

func main() {
	bin := Binary{
		"uniswap-v2@1": exchange{"Uniswap v2"}, "uniswap-v3@1": exchange{"Uniswap v3"},
		"aerodrome@1": exchange{"Aerodrome"}, "balancer-v2@1": exchange{"Balancer v2"},
	}
	reg := []RegistryRow{
		{"factory-u2", "uniswap-v2", 1}, {"factory-u3", "uniswap-v3", 1},
		{"factory-ae", "aerodrome", 1}, {"factory-ba", "balancer-v2", 1},
	}
	core, err := Start(reg, bin)
	fmt.Printf("start: %d registry rows, %d routes, err %v\n", len(reg), len(core.route), err)
	sw := core.Walk([]Log{{"pool-7", "factory-u3"}})
	fmt.Printf("swap log from pool-7 (factory-u3) -> decoded by %s\n", sw[0].Exchange)

	// A new exchange: one plug-in in the binary, one row in the registry.
	bin["velodrome@1"] = exchange{"Velodrome"}
	reg = append(reg, RegistryRow{"factory-ve", "velodrome", 1})
	core, err = Start(reg, bin)
	sw = core.Walk([]Log{{"pool-9", "factory-ve"}})
	fmt.Printf("add Velodrome: %d rows, %d routes, err %v, pool-9 -> %s\n",
		len(reg), len(core.route), err, sw[0].Exchange)

	// The registry update names velodrome@2. The running binary has only @1.
	reg[4].Version = 2
	for try := 1; try <= 3; try++ {
		_, err = Start(reg, bin)
		fmt.Printf("  start try %d: %v\n", try, err)
	}
	verdict, missing := PreRoll(reg, bin)
	fmt.Printf("pre-roll check, old binary: %s, missing %v\n", verdict, missing)

	bin["velodrome@2"] = exchange{"Velodrome"} // the binary that carries v2
	verdict, missing = PreRoll(reg, bin)
	core, err = Start(reg, bin)
	fmt.Printf("pre-roll check, new binary: %s, missing %d, start err %v, routes %d\n", verdict, len(missing), err, len(core.route))
}
