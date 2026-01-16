package cmds

import (
	"encoding/hex"
	"fadingrose/iris-ether-v3/cfg"
	"fadingrose/iris-ether-v3/provider"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Run is the main entry point for the command.
// argsp[0] is the path to the bytecode file or contract address.
func Run(cmd *cobra.Command, target string, path string, timeout int) error {
	var (
		bytecode []byte
		err      error
		prov     *provider.LocalProvider
	)

	if strings.HasPrefix(target, "0x") {
		prov, err = provider.NewLocalBenchmarkDB(path)
		if err != nil {
			return err
		}
		hexcode, err := prov.GetRuntimeBinary(target)
		if err != nil {
			return err
		}
		bytecode, err = hex.DecodeString(string(hexcode))
		if err != nil {
			return err
		}

	} else {
		bytecode, err = os.ReadFile(target)
	}

	if err != nil {
		return err
	}

	c := cfg.NewCFG(bytecode)
	c.CFGBuilder().Build(nil, 1, 1024,
		time.Duration(timeout)*time.Second)

	return nil
}
