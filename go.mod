module github.com/zhiqiangxu/okex-verify

go 1.15

require github.com/okex/okexchain-go-sdk v0.16.0

replace (
	github.com/cosmos/cosmos-sdk => github.com/okex/cosmos-sdk v0.39.2-okexchain9
	github.com/tendermint/iavl => github.com/okex/iavl v0.14.1-okexchain1
	github.com/tendermint/tendermint => github.com/okex/tendermint v0.33.9-okexchain5
)
