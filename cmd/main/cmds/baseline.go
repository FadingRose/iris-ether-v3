package cmds

import (
	"database/sql"
	"fmt"
	"time"

	"fadingrose/iris-ether-v3/cfg"
	"fadingrose/iris-ether-v3/provider"
)

func RunBaseline() error {
	dbPath := "./database.db"
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	query := `
		SELECT address, covered_blocks, total_blocks, time, halt_reason
		FROM stat_base 
	`
	// WHERE halt_reason = 'stagnation'
	rows, err := db.Query(query)
	if err != nil {
		return err
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
			fmt.Println(err)
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

	path := "./runtime_binary.db"
	prov, err := provider.NewLocalBenchmarkDB(path)
	if err != nil {
		return err
	}

	resDbPath := "v3_baseline.db"
	resDb, err := sql.Open("sqlite3", resDbPath)
	if err != nil {
		return err
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
		return err
	}

	executed := make(map[string]bool)
	rowsEx, err := resDb.Query("SELECT address FROM v3_baseline")
	if err == nil {
		for rowsEx.Next() {
			var addr string
			if err := rowsEx.Scan(&addr); err == nil {
				executed[addr] = true
			}
		}
		rowsEx.Close()
	}

	var (
		better  int
		worse   int
		equal   int
		faster  int
		slower  int
		timeOld int64
		timeNew float64
	)

	fmt.Printf("\n%-42s | %-13s | %-17s | %-15s | %-15s | %s\n", "Address", "Cov(Old/New)", "Ratio(Old/New/Diff)", "Time(Ratio)", "Halt(Old)", "Halt(New)")
	fmt.Println("-----------------------------------------------------------------------------------------------------------------------------------------")

	for _, tc := range tcs {

		if executed[tc.address] {
			continue
		}
		bytecode, err := prov.GetRuntimeBinary(tc.address)
		if err != nil {
			return err
		}

		c := cfg.NewCFG(bytecode)
		if c == nil {
			continue
		}

		start := time.Now()
		c.CFGBuilder().Build(nil, 1, 1024, 30*time.Second)
		elapsed := time.Since(start)
		newCostMs := float64(elapsed.Nanoseconds()) / 1e6

		cover, _ := c.Coverage()
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
			tc.address, tc.coveredBlocks, cover, oldRatio*100, newRatio*100, (newRatio-oldRatio)*100, timeRatio, tc.haltReason, c.HaltReason())

		// Record results to v3_baseline.db
		_, err = resDb.Exec(`
			INSERT OR REPLACE INTO v3_baseline (
				address, old_covered, new_covered, total_blocks, 
				old_ratio, new_ratio, old_time_ms, new_time_ms, 
				old_halt_reason, new_halt_reason
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			tc.address, tc.coveredBlocks, cover, total,
			oldRatio, newRatio, float64(tc.timecost), newCostMs,
			tc.haltReason, c.HaltReason(),
		)
		if err != nil {
			fmt.Printf("failed to save baseline for %s: %v", tc.address, err)
		}

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

		// if cover < tc.coveredBlocks {
		// 	// t.Fatalf("covered block mismatch: %d < %d", cover, tc.coveredBlocks)
		// 	t.Errorf("covered block mismatch for %s: %d < %d", tc.address, cover, tc.coveredBlocks)
		// }
		// if c.haltReason != "fixed" {
		// 	t.Errorf("halt reason mismatch for %s: %s != fixed", tc.address, c.haltReason)
		// 	// t.Fatalf("halt reason mismatch: %s != fixed", c.haltReason)
		// }
		//
	}

	fmt.Println("-----------------------------------------------------------------------------------------------------------------------------------------")
	fmt.Printf("Summary (Total %d):\n", len(tcs))
	fmt.Printf("Coverage: Better %d, Worse %d, Equal %d\n", better, worse, equal)
	fmt.Printf("Time    : Faster %d, Slower %d\n", faster, slower)
	fmt.Printf("Total Time: %d ms (Old) vs %.3f ms (New)\n", timeOld, timeNew)
	return nil
}
