package main

import (
	"fmt"
	"os"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	"github.com/cosmos/cosmos-sdk/version"

	"github.com/funai-wiki/funai-chain/app"
	"github.com/funai-wiki/funai-chain/cmd/funaid/cmd"
)

func init() {
	version.Name = "funai"
	version.AppName = "funaid"
	version.Version = "1.0.0"
}

func main() {
	app.SetAddressPrefixes()

	rootCmd := cmd.NewRootCmd()

	if err := svrcmd.Execute(rootCmd, "", app.DefaultNodeHome); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
