package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/cosmos/cosmos-sdk/types"
	config2 "github.com/tendermint/tendermint/config"
	"os"
	"path"
)

type ASIUpgradeIBCTransfer struct {
	From      string      `json:"from"`
	ChannelID string      `json:"channel_id"`
	Amount    types.Coins `json:"amount"`
}

type ASIUpgradeIBCTransfers struct {
	Transfers                   []ASIUpgradeIBCTransfer `json:"transfer"`
	To                          string                  `json:"to"`
	AggregatedTransferredAmount types.Coins             `json:"aggregated_transferred_amount"`
	NumberOfTransfers           int                     `json:"number_of_transfers"`
}

type ASIUpgradeReconciliationTransfer struct {
	From    string      `json:"from"`
	EthAddr string      `json:"eth_addr"`
	Amount  types.Coins `json:"amount"`
}

type ASIUpgradeReconciliationTransfers struct {
	Transfers                   []ASIUpgradeReconciliationTransfer `json:"transfers"`
	To                          string                             `json:"to"`
	AggregatedTransferredAmount types.Coins                        `json:"aggregated_transferred_amount"`
	NumberOfTransfers           int                                `json:"number_of_transfers"`
}

type ASIUpgradeReconciliationContractStateBalanceRecord struct {
	EthAddr  string      `json:"eth_addr"`
	Balances types.Coins `json:"balances"`
}

type ASIUpgradeReconciliationContractState struct {
	Balances                 []ASIUpgradeReconciliationContractStateBalanceRecord `json:"balances"`
	AggregatedBalancesAmount types.Coins                                          `json:"aggregated_balances_amount"`
	NumberOfBalanceRecords   int                                                  `json:"number_of_balance_records"`
}

func NewASIUpgradeReconciliationContractState() *ASIUpgradeReconciliationContractState {
	return &ASIUpgradeReconciliationContractState{
		Balances: make([]ASIUpgradeReconciliationContractStateBalanceRecord, 0),
	}
}

type ASIUpgradeReconciliation struct {
	Transfers     ASIUpgradeReconciliationTransfers      `json:"transfers"`
	ContractState *ASIUpgradeReconciliationContractState `json:"contract_state"`
}

type ASIUpgradeSupply struct {
	LandingAddress       string      `json:"landing_address"`
	MintedAmount         types.Coins `json:"minted_amount"`
	ResultingTotalSupply types.Coins `json:"resulting_total_supply"`
}

type ContractValueUpdate struct {
	Address string `json:"address"`
	From    string `json:"from"`
	To      string `json:"to"`
}

type ValueUpdate struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type NetworkParams struct {
	GenesisTime   *ValueUpdate      `json:"genesis_time,omitempty"`
	ChainID       *ValueUpdate      `json:"chain_id,omitempty"`
	AddressPrefix *ValueUpdate      `json:"address_prefix,omitempty"`
	Supply        *ASIUpgradeSupply `json:"supply,omitempty"`
}

type ContractVersion struct {
	Contract string `json:"contract"`
	Version  string `json:"version"`
}

type ContractVersionUpdate struct {
	Address string           `json:"address"`
	From    *ContractVersion `json:"from,omitempty"`
	To      *ContractVersion `json:"to"`
}

type Contracts struct {
	StateCleaned   []string                `json:"state_cleaned,omitempty"`
	AdminUpdated   []ContractValueUpdate   `json:"admin_updated,omitempty"`
	LabelUpdated   []ContractValueUpdate   `json:"label_updated,omitempty"`
	VersionUpdated []ContractVersionUpdate `json:"version_updated,omitempty"`
}

type ASIUpgradeManifest struct {
	Network        *NetworkParams            `json:"network,omitempty"`
	IBC            *ASIUpgradeIBCTransfers   `json:"ibc,omitempty"`
	Reconciliation *ASIUpgradeReconciliation `json:"reconciliation,omitempty"`
	Contracts      *Contracts                `json:"contracts,omitempty"`
}

func SaveASIManifest(manifest *ASIUpgradeManifest, config *config2.Config) error {
	var serialisedManifest []byte
	var err error
	if serialisedManifest, err = json.MarshalIndent(manifest, "", "\t"); err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	var f *os.File
	const manifestFilename = "asi_upgrade_manifest.json"
	genesisFilePath := config.GenesisFile()
	manifestFilePath := path.Join(path.Dir(genesisFilePath), manifestFilename)
	if f, err = os.Create(manifestFilePath); err != nil {
		return fmt.Errorf("failed to create file \"%s\": %w", manifestFilePath, err)
	}
	defer f.Close()

	if _, err = f.Write(serialisedManifest); err != nil {
		return fmt.Errorf("failed to write manifest to the \"%s\" file : %w", manifestFilePath, err)
	}

	return nil
}
