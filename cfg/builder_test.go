package cfg

import (
	"encoding/hex"
	"fmt"
	"testing"

	"fadingrose/iris-ether-v3/provider"
)

// Sum_D and Fix_D test
// 任何一个 D 类型节点；其 dest 解析的结果应当只有两种情况：
// 1. 依赖于 D 的 S_in，说明是 D
// 2. 完全不依赖于 D 的 S_in, 那么要经过 any stack test
//   - 如果通过，说明为 fixed_D，即它不是一个 context-aware 的块
//   - 如果不通过，这是未定义行为，这个错误应该被捕捉
func TestDTypeBlock(t *testing.T) {
	// 0x003e523dfb5d91e508745256603d222891e65828 stuck at here
	path := "../../instrument_migrate.db"
	prov, err := provider.NewLocalBenchmarkDB(path)
	if err != nil {
		t.Fatal(err)
	}
	// addresses, err := prov.GeetAddresses()
	// if err != nil {
	// 	t.Fatal(err)
	// }
	addresses := []string{"0x003e523dfb5d91e508745256603d222891e65828"}
	total := len(addresses)
	for i, address := range addresses {
		if i%100 == 0 {
			fmt.Printf("Processed %d/%d addresses\n", i, total)
		}
		fmt.Println(address)
		hexcode, err := prov.GetRuntimeBinary(address)
		if err != nil {
			t.Fatal(err)
		}
		bytecode, err := hex.DecodeString(string(hexcode))
		if err != nil {
			// t.Fatal(err)
			fmt.Println(err)
			continue
		}

		c := NewCFG(bytecode)

		// Step 1: LiteralConnectSuccessors
		c.LiteralConnectSuccessors()

		// Step 2: init D set and fixed D set
		// DFS from the entry basic block
		var (
			SumD []*Path
			// sigma_D []*Path
		)

		visited := make(map[*BasicBlock]bool)
		var dfs func(bb *BasicBlock, currentPath *Path)
		dfs = func(bb *BasicBlock, currentPath *Path) {
			if visited[bb] {
				return
			}
			visited[bb] = true

			// If this block is in D, we've found a path for sum_D
			if bb.isContextAware() {
				fmt.Printf("Path: %s\nPath length: %d, Visiting: %d\n", currentPath.String(), len(currentPath.data), bb.index)
				SumD = append(SumD, currentPath.copy())
			}

			for _, succ := range bb.succs {
				newPath := currentPath.copy()
				if succ.Dst == nil {
					continue
				}
				newPath.push(succ.Dst)

				// fmt.Printf("Path length: %d, Visiting: %d\n", len(newPath.data), succ.Dst.index)
				dfs(succ.Dst, newPath)
			}
			visited[bb] = false
		}

		entry := c.BasicBlocks[0]
		dfs(entry, NewPath(entry))
		fmt.Println()

		// Step 3: loop until D set is empty
		// 	for len(cb.D) > 0 {
		// 	}
		for _, path := range SumD {
			i := NewInterpreter(path, 1024)
			last, _, snaps, err := i.Run()
			if err != nil {
				fmt.Println("err", err)
				continue
			}
			if last == nil {
				fmt.Println("last is nil")
				continue
			}

			_, flag := snaps[path.last()].Live(last.params[0])
			// fmt.Println(res)
			// 如果 last 的 dest 解析的值不依赖于 S_in，那么它应当直接来源于最后一个块
			if !flag {
				path = NewPath(path.data[len(path.data)-1])
				i := NewInterpreter(path, 1024)
				st := NewAnyStack(c)
				i.SetStack(st)
				slast, _, _, err := i.Run()
				if err != nil {
					fmt.Println("any test failed, err: ", err)
					continue
				}
				if last.params[0].value().Uint64() != slast.params[0].value().Uint64() {
					fmt.Println(path.String())
					fmt.Println("any test failed, last is not last")
					fmt.Println(last.params[0].value())
					fmt.Println(slast.params[0].value())
					fmt.Println(last.Expand())
					fmt.Println(slast.Expand())
					continue
				}
				continue
			}

			// fmt.Println(last.Expand())
		}
		// cb.cfg.PrintReadableCFG()
	}
}
