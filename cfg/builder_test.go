package cfg

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

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

				path.last().AppendSuccessor(dst, Jump, nil)

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
	path := "../runtime_binary.db"
	prov, err := provider.NewLocalBenchmarkDB(path)
	if err != nil {
		t.Fatal(err)
	}

	addresses := []string{"0x31608722ac5ed1c2b1ae0af12602bec21f7fe0df"}
	// addresses := []string{"0x0389ca91dbc41d828f7d25347afc23610ad8a16d"}

	for _, address := range addresses {
		fmt.Println(address)
		bytecode, err := prov.GetRuntimeBinary(address)
		if err != nil {
			fmt.Println(err)
			continue
		}
		c := NewCFG(bytecode)
		if c == nil {
			continue
		}
		c.CFGBuilder().Build(nil, 1, 1024, 30*time.Second)
		cover, total := c.coverage()
		fmt.Printf("%s: %d/%d\n", address, cover, total)
		// c.PrintReadableCFG()
	}
}

func TestStagnation(t *testing.T) {
	dbPath := "../database.db"
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	query := `
		SELECT address, covered_blocks, total_blocks, time, halt_reason
		FROM stat_base 
	`
	// WHERE halt_reason = 'stagnation'
	rows, err := db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	type tc struct {
		address       string
		coveredBlocks int
		totalBlocks   int
		timecost      int
		haltReason    string
	}

	tcs := []tc{}

	fmt.Println("Addresses with stagnation halt reason:")
	for rows.Next() {
		var address string
		var coveredBlocks, totalBlocks int
		var timecost int
		var haltReason string
		if err := rows.Scan(&address, &coveredBlocks, &totalBlocks, &timecost, &haltReason); err != nil {
			t.Error(err)
			continue
		}

		// timecost is in 10 ns, convert to ms
		timecost = timecost / 1e5

		tcs = append(tcs, tc{
			address:       address,
			coveredBlocks: coveredBlocks,
			totalBlocks:   totalBlocks,
			timecost:      timecost,
			haltReason:    haltReason,
		})
	}

	path := "../runtime_binary.db"
	prov, err := provider.NewLocalBenchmarkDB(path)
	if err != nil {
		t.Fatal(err)
	}

	resDbPath := "v3_baseline.db"
	resDb, err := sql.Open("sqlite3", resDbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer resDb.Close()

	_, err = resDb.Exec(`
		CREATE TABLE IF NOT EXISTS v3_baseline (
			address TEXT PRIMARY KEY,
			old_covered INTEGER,
			new_covered INTEGER,
			total_blocks INTEGER,
			old_ratio REAL,
			new_ratio REAL,
			old_time_ms REAL,
			new_time_ms REAL,
			old_halt_reason TEXT,
			new_halt_reason TEXT,
			captured_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	var (
		better      int
		worse       int
		equal       int
		faster      int
		slower      int
		timeOld     int64
		timeNew     float64
		totalOldCov int
		totalNewCov int
		totalBlocks int
	)

	fmt.Printf("\n%-42s | %-13s | %-17s | %-15s | %-15s | %s\n", "Address", "Cov(Old/New)", "Ratio(Old/New/Diff)", "Time(Ratio)", "Halt(Old)", "Halt(New)")
	fmt.Println("-----------------------------------------------------------------------------------------------------------------------------------------")

	tcs = tcs[:500]
	// 0x493f28e684e76fd73cf241995cf20b7e311ed621

	for _, tc := range tcs {
		// if tc.address != "0x493f28e684e76fd73cf241995cf20b7e311ed621" {
		// 	continue
		// }
		bytecode, err := prov.GetRuntimeBinary(tc.address)
		if err != nil {
			t.Fatal(err)
		}

		c := NewCFG(bytecode)
		if c == nil {
			continue
		}

		start := time.Now()
		c.CFGBuilder().Build(nil, 1, 1024, 30*time.Second)
		elapsed := time.Since(start)
		newCostMs := float64(elapsed.Nanoseconds()) / 1e6

		cover, _ := c.coverage()
		total := tc.totalBlocks

		oldRatio := 0.0
		if tc.totalBlocks > 0 {
			oldRatio = float64(tc.coveredBlocks) / float64(tc.totalBlocks)
		}

		newRatio := 0.0
		if total > 0 {
			newRatio = float64(cover) / float64(total)
		}

		timeRatio := 0.0
		if tc.timecost > 0 {
			timeRatio = (newCostMs / float64(tc.timecost)) * 100
		}

		fmt.Printf("%s | %4d / %-4d | %5.1f%% / %5.1f%% (%+6.1f%%) | %12.8f%% | %-15s | %s\n",
			tc.address, tc.coveredBlocks, cover, oldRatio*100, newRatio*100, (newRatio-oldRatio)*100, timeRatio, tc.haltReason, c.haltReason)

		// Record results to v3_baseline.db
		_, err = resDb.Exec(`
			INSERT OR REPLACE INTO v3_baseline (
				address, old_covered, new_covered, total_blocks, 
				old_ratio, new_ratio, old_time_ms, new_time_ms, 
				old_halt_reason, new_halt_reason
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			tc.address, tc.coveredBlocks, cover, total,
			oldRatio, newRatio, float64(tc.timecost), newCostMs,
			tc.haltReason, c.haltReason,
		)
		if err != nil {
			t.Errorf("failed to save baseline for %s: %v", tc.address, err)
		}

		totalOldCov += tc.coveredBlocks
		totalNewCov += cover
		totalBlocks += total
		// Statistics updates
		if cover > tc.coveredBlocks {
			better++
		} else if cover < tc.coveredBlocks {
			worse++
		} else {
			equal++
		}

		if newCostMs < float64(tc.timecost) {
			faster++
		} else {
			slower++
		}

		timeOld += int64(tc.timecost)
		timeNew += newCostMs

		if cover < tc.coveredBlocks {
			// t.Fatalf("covered block mismatch: %d < %d", cover, tc.coveredBlocks)
			t.Errorf("covered block mismatch for %s: %d < %d", tc.address, cover, tc.coveredBlocks)
		}
		if c.haltReason != "fixed" {
			t.Errorf("halt reason mismatch for %s: %s != fixed", tc.address, c.haltReason)
			// t.Fatalf("halt reason mismatch: %s != fixed", c.haltReason)
		}

	}

	fmt.Println("-----------------------------------------------------------------------------------------------------------------------------------------")
	fmt.Printf("Summary (Total %d):\n", len(tcs))
	fmt.Printf("Coverage: Better %d, Worse %d, Equal %d\n", better, worse, equal)
	fmt.Printf("Total Cov: %d (Old) vs %d (New) | Total Blocks: %d\n", totalOldCov, totalNewCov, totalBlocks)
	fmt.Printf("Avg Ratio: %.2f%% (Old) vs %.2f%% (New)\n", float64(totalOldCov)/float64(totalBlocks)*100, float64(totalNewCov)/float64(totalBlocks)*100)
	fmt.Printf("Time    : Faster %d, Slower %d\n", faster, slower)
	fmt.Printf("Total Time: %d ms (Old) vs %.3f ms (New)\n", timeOld, timeNew)
}

// Coverage: Better 20, Worse 20, Equal 10
// Total Cov: 25107 (Old) vs 24225 (New) | Total Blocks: 26333
// Avg Ratio: 95.34% (Old) vs 91.99% (New)
// Time    : Faster 50, Slower 0
// Total Time: 466467 ms (Old) vs 4110.399 ms (New)
// --- FAIL: TestStagnation (4.24s)

// Summary (Total 50): with fix, pick up from pruned liveness and loop pruned, windows is 5
// Coverage: Better 24, Worse 16, Equal 10
// Total Cov: 25107 (Old) vs 24678 (New) | Total Blocks: 26333
// Avg Ratio: 95.34% (Old) vs 93.72% (New)
// Time    : Faster 50, Slower 0
// Total Time: 466467 ms (Old) vs 12623.683 ms (New)

// Summary (Total 50):with fix, pick up from pruned liveness and loop pruned, windows is 4
// Coverage: Better 23, Worse 17, Equal 10
// Total Cov: 25107 (Old) vs 24490 (New) | Total Blocks: 26333
// Avg Ratio: 95.34% (Old) vs 93.00% (New)
// Time    : Faster 50, Slower 0
// Total Time: 466467 ms (Old) vs 6127.067 ms (New)

// Summary (Total 50): with fix, pick up from pruned liveness and loop pruned, windows is 3
// Coverage: Better 21, Worse 19, Equal 10
// Total Cov: 25107 (Old) vs 24373 (New) | Total Blocks: 26333
// Avg Ratio: 95.34% (Old) vs 92.56% (New)
// Time    : Faster 50, Slower 0
// Total Time: 466467 ms (Old) vs 4795.513 ms (New)

// Summary (Total 50): with fix, pick up from pruned liveness, windows is 3
// Coverage: Better 22, Worse 18, Equal 10
// Total Cov: 25107 (Old) vs 24274 (New) | Total Blocks: 26333
// Avg Ratio: 95.34% (Old) vs 92.18% (New)
// Time    : Faster 50, Slower 0
// Total Time: 466467 ms (Old) vs 3792.279 ms (New)
//
//
// Summary (Total 50): does not pick up from pruned liveness, windows is 3
// Coverage: Better 4, Worse 39, Equal 7
// Total Cov: 25107 (Old) vs 16689 (New) | Total Blocks: 26333
// Avg Ratio: 95.34% (Old) vs 63.38% (New)
// Time    : Faster 50, Slower 0
// Total Time: 466467 ms (Old) vs 545.824 ms (New)
