package main

import (
	"fmt"

	sdk "github.com/okex/okexchain-go-sdk"
)

var (
	rpcURL   = "tcp://18.167.142.95:26657"
	name     = "alice"
	passWd   = "12345678"
	mnemonic = "giggle sibling fun arrow elevator spoon blood grocery laugh tortoise culture tool"
	addr     = "okexchain1ntvyep3suq5z7789g7d5dejwzameu08m6gh7yl"
)

func main() {

	config, _ := sdk.NewClientConfig(rpcURL, "okexchain-65", sdk.BroadcastBlock, "0.01okt", 200000, 0, "")
	client := sdk.NewClient(config)

	height := int64(1580000)
	commitResult, err := client.Tendermint().QueryCommitResult(height)
	if err != nil {
		panic(err)
	}

	valResult, err := client.Tendermint().QueryValidatorsResult(height)
	if err != nil {
		panic(err)
	}

	fmt.Println("commitResult", commitResult)
	fmt.Println("valResult", valResult)
}
