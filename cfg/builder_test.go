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

			_, _, flag := snaps[path.last()].Live(last.params[0])
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

// 在从 entry -> D 的路径搜索上，可能存在大量 paht
// e.g., 0x003e523dfb5d91e508745256603d222891e65828 在 TestDTypeBlock 中路径无法穷举结束
// 需要提前测试 Liveness 的减枝 和 JumpVector 重复路径减枝针对这个 case 是否成立？
func TestDTypeBlockWithLiveness(t *testing.T) {
	path := "../../instrument_migrate.db"
	prov, err := provider.NewLocalBenchmarkDB(path)
	if err != nil {
		t.Fatal(err)
	}

	addresses, err := prov.GeetAddresses()
	if err != nil {
		t.Fatal(err)
	}
	// addresses := []string{"0x336e296a9f021e459d20d2c249990ebc4f6b4a56"}

	//  DONE: panic at this address
	// 0x265807d818ff14d44b552f5897abdad6eed603e2
	// last.params[0].value() is nil
	// 0 -> 2 -> 3 -> 26 -> 37 -> 38 -> 39 -> 40 -> 41 -> 68 -> 221 -> 223 -> 226 -> 232 -> 233^
	// 0xf37(3895) JUMP
	// ├── 0x1f(31) SHR
	// │   ├── 0x1c(28) CALLDATALOAD
	// │   │   ├── 0x1a(26) PUSH1 0x00
	// │   ├── 0x1d(29) PUSH1 0xe0
	//
	// 0x152df2cb3868bef5c6ed6b9feb0fb091c9e11a96  DONE: stuck at here
	// INVALID 不一定截断 code，offset 也可能失效，validDest 也可能不考虑 offset
	//
	// 0x336e296a9f021e459d20d2c249990ebc4f6b4a56 DONE: 引入 Jump Vector 即不会无限 stuck

	total := len(addresses)
	for i, address := range addresses {
		// fmt.Println(address)
		fmt.Printf("Processed %d/%d addresses\n", i, total)
		hexcode, err := prov.GetRuntimeBinary(address)
		if err != nil {
			t.Fatal(err)
		}
		bytecode, err := hex.DecodeString(string(hexcode))
		if err != nil {
			// fmt.Println(address, err)
			// fmt.Printf("decode bytecode error: %s\n, address: %s\n", err, address)
			continue
		}
		c := NewCFG(bytecode)
		if c == nil {
			continue
		}
		// Step 1: LiteralConnectSuccessors
		c.LiteralConnectSuccessors()

		cb := c.CFGBuilder()

		// c.PrintReadableCFG()

		liveness := NewLiveness()

		// Collect all paths form entry to D_sets
		var SumD []*Path
		visited := make(map[*BasicBlock]bool)
		// var D_Path_Found func(path *Path) bool
		D_Path_Found := func(path *Path) bool {
			i := NewInterpreter(path, 1024)
			last, _, snaps, err := i.Run()
			if err != nil {
				fmt.Println("err", err)
				return false
			}
			if last == nil {
				fmt.Println("last is nil")
				return false
			}

			// fmt.Println(last.Expand())

			_, _, flag := snaps[path.last()].Live(last.params[0])
			// 如果 last 的 dest 解析的值不依赖于 S_in，那么它应当直接来源于最后一个块
			// fmt.Println(path.String())
			if !flag {
				path = NewPath(path.data[len(path.data)-1])
				i := NewInterpreter(path, 1024)
				st := NewAnyStack(c)
				i.SetStack(st)
				slast, _, _, err := i.Run()
				if err != nil {
					fmt.Println("any test failed, err: ", err)
					panic(fmt.Sprintf("any test failed, err: %s", address))
					return false
				}
				if last.params[0].value().Uint64() != slast.params[0].value().Uint64() {
					fmt.Println(path.String())
					fmt.Println("any test failed, last is not last")
					fmt.Println(last.params[0].value())
					fmt.Println(slast.params[0].value())
					fmt.Println(last.Expand())
					fmt.Println(slast.Expand())
					panic(fmt.Sprintf("any test failed, last is not last: %s", address))
					return false
				}
				// fmt.Println(address, ": any test passed") // TODO: add this to fixed_D set
				// NOTE: append a JUMP edge

				dst := c.verifyDest(last.params[0].value().Uint64())
				if dst == nil {
					panic(fmt.Sprintf("verifyDest failed: %s", address))
				}

				path.last().AppendSuccessor(dst, Jump)

				return false
			}

			// flag is true，即 last 依赖输入栈
			// 对于 path 中的每一个块，都有一个特定的 Liveness 的组合
			// 一个 Liveness 意味着：一个块的一个特定栈状态导出到特定的 D
			// fmt.Println(path.String())
			if last.params[0].value() == nil {
				fmt.Println("last.params[0].value() is nil")
				fmt.Println(path.String())
				fmt.Println(last.Expand())
				return false
			}
			dest := last.params[0].value().Uint64()
			D_Block := path.last().index

			var meaningful bool // update liveness at least once
			for i := len(path.data) - 1; i >= 0; i-- {
				slots, _, flag := snaps[path.data[i]].Live(last.params[0])
				if liveness.Live(path.data[i], slots, D_Block, dest) {
					meaningful = true
				}
				// fmt.Println(path.data[i].String())
				// fmt.Println(res)
				// fmt.Println(flag)
				if !flag {
					// fmt.Printf("cutoff at block %d of path %s\n", path.data[i].index, path.String())
					break
				}
			}
			return meaningful
		} // end of D_Path_Found()

		var dfs func(bb *BasicBlock, currentPath *Path)
		dfs = func(bb *BasicBlock, currentPath *Path) {
			// liveness test
			// 给定一个 path，我们需要检查它现在的输出 stack 是否命中任何一个当前 Block 的 liveness？
			// 如果命中任何一个 liveness，那么提前减枝，并且记录到 prune 中
			// 这里的减枝可能有精度损失，所以我们要记录原始 stack 到 p1)
			// fmt.Println(currentPath.String())
			_, err := cb.execute(currentPath)
			if err != nil {
				return
			}

			// 需要测试最后一个块的输入栈，因此 执行的 path 是 path[0:len(path)-1]
			if len(currentPath.data) > 1 {
				testPath := currentPath.copy()
				testPath.data = testPath.data[:len(testPath.data)-1]
				i := NewInterpreter(testPath, 1024)
				// fmt.Println(testPath.String())
				_, st, _, err := i.Run()
				if err != nil {
					return
				}
				// 如果 liveness 命中，记录到 prune 中并且跳出循环
				if liveness.Check(st, currentPath, bb.index) {
					// fmt.Println("liveness test passed, match one of the liveness")
					// fmt.Println(currentPath.String())
					return
				}
			}

			if bb.isLeadingHalt() {
				// fmt.Println("leading halt")
				return
			}

			if visited[bb] {
				return
			}
			visited[bb] = true

			// If this block is in D, we've found a path for sum_D
			if bb.isContextAware() {
				// fmt.Printf("Path: %s\nPath length: %d, Visiting: %d\n", currentPath.String(), len(currentPath.data), bb.index)
				newPath := currentPath.copy()
				if D_Path_Found(newPath) {
					SumD = append(SumD, newPath)
				}
			}

			// if len(SumD) > 10 {
			// 	return
			// }

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
		} // end of dfs()

		entry := c.BasicBlocks[0]
		dfs(entry, NewPath(entry))

	}
}

func TestCFGBuilder(t *testing.T) {
	path := "../../instrument_migrate.db"
	prov, err := provider.NewLocalBenchmarkDB(path)
	if err != nil {
		t.Fatal(err)
	}

	// addresses, err := prov.GeetAddresses()
	// if err != nil {
	// 	t.Fatal(err)
	// }

	addresses := []string{"0x103a9d0ee6fdc4762ec08172ff0881ebb68c73c8"}

	for _, address := range addresses {
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
		if c == nil {
			continue
		}
		c.PrintReadableCFG()
		p := NewPathFromString("0 -> 1 -> 2 -> 3 -> 4 -> 5 -> 6 -> 7 -> 8 -> 9 -> 10 -> 71 -> 73 -> 122", c.BasicBlocks)
		c.CFGBuilder().Build(p, 1, 1024)
	}
}
