package cmds

import (
	"fadingrose/iris-ether-v3/cfg"
	"fadingrose/iris-ether-v3/provider"
	"os"
	"strings"

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
		bytecode, err = prov.GetRuntimeBinary(target)
	} else {
		bytecode, err = os.ReadFile(target)
	}

	if err != nil {
		return err
	}

	c := cfg.NewCFG(bytecode)
	c.Build(1, 1024, timeout, false)

	return nil
}
