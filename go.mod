module github.com/fetchai/fetchd

go 1.16

require (
	github.com/99designs/keyring v1.1.6 // indirect
	github.com/CosmWasm/wasmvm v0.14.0 // indirect
	github.com/armon/go-metrics v0.3.8 // indirect
	github.com/cockroachdb/apd/v2 v2.0.2 // indirect
	github.com/cosmos/go-bip39 v1.0.0 // indirect
	github.com/cosmos/iavl v0.16.0 // indirect
	github.com/cosmos/ledger-cosmos-go v0.11.1 // indirect
	github.com/enigmampc/btcutil v1.0.3-0.20200723161021-e2fb6adb2a25 // indirect
	github.com/gogo/protobuf v1.3.3 // indirect
	github.com/golang/mock v1.4.4 // indirect
	github.com/golang/snappy v0.0.3-0.20201103224600-674baa8c7fc3 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/gorilla/handlers v1.5.1 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/ipfs/go-cid v0.0.7 // indirect
	github.com/lib/pq v1.10.2 // indirect
	github.com/magiconair/properties v1.8.5 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/mitchellh/mapstructure v1.3.3 // indirect
	github.com/multiformats/go-multihash v0.0.14 // indirect
	github.com/otiai10/copy v1.6.0 // indirect
	github.com/pelletier/go-toml v1.8.1 // indirect
	github.com/prometheus/client_golang v1.10.0
	github.com/prometheus/common v0.23.0 // indirect
	github.com/rakyll/statik v0.1.7 // indirect
	github.com/regen-network/cosmos-proto v0.3.1 // indirect
	github.com/rs/zerolog v1.21.0 // indirect
	github.com/spf13/cast v1.3.1
	github.com/spf13/cobra v1.1.3
	github.com/supranational/blst v0.3.4 // indirect
	github.com/tendermint/btcd v0.1.1 // indirect
	github.com/tendermint/crypto v0.0.0-20191022145703-50d29ede1e15 // indirect
	github.com/tendermint/go-amino v0.16.0 // indirect
	github.com/tendermint/tendermint v0.34.11
	github.com/tendermint/tm-db v0.6.4
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2 // indirect
	golang.org/x/sys v0.0.0-20210510120138-977fb7262007 // indirect
	google.golang.org/genproto v0.0.0-20210406143921-e86de6bf7a46 // indirect
	gopkg.in/ini.v1 v1.61.0 // indirect
)

// fix for "invalid Go type types.Dec for field ..." errors
// see: https://github.com/cosmos/cosmos-sdk/issues/8426
replace google.golang.org/grpc => google.golang.org/grpc v1.33.2

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1

//replace github.com/cosmos/cosmos-sdk => github.com/fetchai/cosmos-sdk v0.17.3
//replace github.com/cosmos/cosmos-sdk => github.com/kitounliu/cosmos-sdk v0.17.4-0.20210802124821-c9eafc940426

//replace github.com/cosmos/cosmos-sdk => github.com/fetchai/cosmos-sdk v0.17.4-0.20210726151136-fcd3a279a7dd
replace github.com/cosmos/cosmos-sdk => github.com/kitounliu/cosmos-sdk v0.17.4-0.20210824151315-ea33b8ffa04f

replace github.com/regen-network/regen-ledger => github.com/kitounliu/regen-ledger v1.0.0-fetchai-1

replace github.com/tendermint/tendermint => github.com/fetchai/tendermint v1.0.0
