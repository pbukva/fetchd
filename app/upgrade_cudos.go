package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	ibccore "github.com/cosmos/ibc-go/v3/modules/core/24-host"
	"github.com/spf13/cast"
	tmtypes "github.com/tendermint/tendermint/types"
	"strings"
)

const (
	Bech32Chars        = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
	AddrDataLength     = 32
	WasmAddrDataLength = 52
	MaxAddrDataLength  = 100
	AddrChecksumLength = 6

	AccAddressPrefix  = ""
	ValAddressPrefix  = "valoper"
	ConsAddressPrefix = "valcons"

	NewAddrPrefix = "fetch"

	FlagGenesisTime = "genesis-time"

	ModuleAccount            = "/cosmos.auth.v1beta1.ModuleAccount"
	BaseAccount              = "/cosmos.auth.v1beta1.BaseAccount"
	DelayedVestingAccount    = "/cosmos.vesting.v1beta1.DelayedVestingAccount"
	ContinuousVestingAccount = "/cosmos.vesting.v1beta1.ContinuousVestingAccount"
	PermanentLockedAccount   = "/cosmos.vesting.v1beta1.PermanentLockedAccount"
	PeriodicVestingAccount   = "/cosmos.vesting.v1beta1.PeriodicVestingAccount"

	UnbondedStatus  = "BOND_STATUS_UNBONDED"
	UnbondingStatus = "BOND_STATUS_UNBONDING"
	BondedStatus    = "BOND_STATUS_BONDED"

	// Modules with balance
	BondedPoolAccName    = "bonded_tokens_pool"
	NotBondedPoolAccName = "not_bonded_tokens_pool"
	GravityAccName       = "gravity"
	DistributionAccName  = "distribution"

	// Modules without balance
	MintAccName         = "cudoMint"
	GovAccName          = "gov"
	MarketplaceAccName  = "marketplace"
	FeeCollectorAccName = "fee_collector"

	RecursionDepthLimit = 50
)

func convertAddressToFetch(addr string, addressPrefix string) (string, error) {
	_, decodedAddrData, err := bech32.DecodeAndConvert(addr)
	if err != nil {
		return "", err
	}

	newAddress, err := bech32.ConvertAndEncode(NewAddrPrefix+addressPrefix, decodedAddrData)
	if err != nil {
		return "", err
	}

	err = sdk.VerifyAddressFormat(decodedAddrData)
	if err != nil {
		return "", err
	}

	return newAddress, nil
}
func convertAddressPrefix(addr string, newPrefix string) (string, error) {
	_, decodedAddrData, err := bech32.DecodeAndConvert(addr)
	if err != nil {
		return "", err
	}

	newAddress, err := bech32.ConvertAndEncode(newPrefix, decodedAddrData)
	if err != nil {
		return "", err
	}

	return newAddress, nil
}

func convertAddressToRaw(addr string, cudosCfg *CudosMergeConfig) (sdk.AccAddress, error) {
	prefix, decodedAddrData, err := bech32.DecodeAndConvert(addr)

	if prefix != cudosCfg.config.OldAddrPrefix {
		return nil, fmt.Errorf("unknown prefix: %s", prefix)
	}

	if err != nil {
		return nil, err
	}

	return decodedAddrData, nil
}

type AccountType string

const (
	BaseAccountType              AccountType = "base_acc"
	ModuleAccountType            AccountType = "module_acc"
	ContractAccountType          AccountType = "contract_acc"
	IBCAccountType               AccountType = "IBC_acc"
	DelayedVestingAccountType    AccountType = "delayed_vesting_acc"
	ContinuousVestingAccountType AccountType = "continuous_vesting_acc"
	PermanentLockedAccountType   AccountType = "permanent_locked_vesting_acc"
	PeriodicVestingAccountType   AccountType = "periodic_vesting_acc"
)

type GenesisData struct {
	totalSupply sdk.Coins

	accounts    *OrderedMap[string, *AccountInfo]
	contracts   *OrderedMap[string, *ContractInfo]
	ibcAccounts *OrderedMap[string, *IBCInfo]
	delegations *OrderedMap[string, *OrderedMap[string, sdk.Coins]]

	validators           *OrderedMap[string, *ValidatorInfo]
	bondedPoolAddress    string
	notBondedPoolAddress string

	distributionInfo *DistributionInfo

	gravityModuleAccountAddress string

	collisionMap *OrderedMap[string, string]
}

func LoadCudosGenesis(app *App, manifest *UpgradeManifest) (*map[string]interface{}, *tmtypes.GenesisDoc, error) {

	if app.cudosGenesisPath == "" {
		return nil, nil, fmt.Errorf("cudos path not set")
	}

	actualGenesisSha256Hex, err := GenerateSHA256FromFile(app.cudosGenesisPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate sha256 out of genesis file %v: %w", app.cudosGenesisPath, err)
	}
	if app.cudosGenesisSha256 != actualGenesisSha256Hex {
		return nil, nil, fmt.Errorf("failed to verify sha256: genesis file \"%v\" hash \"%v\" does not match expected hash \"%v\"", app.cudosGenesisPath, actualGenesisSha256Hex, app.cudosGenesisSha256)
	}
	manifest.GenesisFileSha256 = actualGenesisSha256Hex

	app.Logger().Info("cudos merge: loading merge source genesis json", "file", app.cudosGenesisPath, "expected sha256", app.cudosGenesisSha256)

	_, genDoc, err := genutiltypes.GenesisStateFromGenFile(app.cudosGenesisPath)
	if err != nil {
		return nil, nil, fmt.Errorf("cudos merge: failed to unmarshal genesis state: %w", err)
	}

	// unmarshal the app state
	var jsonData map[string]interface{}
	if err = json.Unmarshal(genDoc.AppState, &jsonData); err != nil {
		return nil, nil, fmt.Errorf("cudos merge: failed to unmarshal app state: %w", err)
	}

	/*
		genesisData, err := parseGenesisData(jsonData, cudosCfg, manifest)
		if err != nil {
			return fmt.Errorf("cudos merge: failed to parse genesis data: %w", err)
		}
	*/

	//genDoc.AppState = nil

	return &jsonData, genDoc, nil

}

func CudosMergeUpgradeHandler(app *App, ctx sdk.Context, cudosCfg *CudosMergeConfig, genesisData *GenesisData, manifest *UpgradeManifest) error {
	if cudosCfg == nil {
		return fmt.Errorf("cudos merge: cudos CudosMergeConfig not provided (null pointer passed in)")
	}

	if app.cudosGenesisPath == "" {
		return fmt.Errorf("cudos merge: cudos path not set")
	}

	err := genesisUpgradeWithdrawIBCChannelsBalances(genesisData, cudosCfg, manifest)
	if err != nil {
		return fmt.Errorf("cudos merge: failed to withdraw IBC channels balances: %w", err)
	}

	err = withdrawGenesisContractBalances(genesisData, manifest, cudosCfg)
	if err != nil {
		return fmt.Errorf("cudos merge: failed to withdraw genesis contracts balances: %w", err)
	}

	err = withdrawGenesisStakingDelegations(app, genesisData, cudosCfg, manifest)
	if err != nil {
		return fmt.Errorf("cudos merge: failed to withdraw genesis staked tokens: %w", err)
	}

	err = withdrawGenesisDistributionRewards(app, genesisData, cudosCfg, manifest)
	if err != nil {
		return fmt.Errorf("cudos merge: failed to withdraw genesis rewards: %w", err)
	}

	err = withdrawGenesisGravity(genesisData, cudosCfg, manifest)
	if err != nil {
		return fmt.Errorf("cudos merge: failed to withdraw gravity: %w", err)
	}

	err = DoGenesisAccountMovements(genesisData, cudosCfg, manifest)
	if err != nil {
		return fmt.Errorf("cudos merge: failed to move funds: %w", err)
	}

	err = MigrateGenesisAccounts(genesisData, ctx, app, cudosCfg, manifest)
	if err != nil {
		return fmt.Errorf("cudos merge: failed process accounts: %w", err)
	}

	err = createGenesisDelegations(ctx, app, genesisData, cudosCfg, manifest)
	if err != nil {
		return fmt.Errorf("cudos merge: failed process delegations: %w", err)
	}

	err = fundCommunityPool(ctx, app, genesisData, cudosCfg, manifest)
	if err != nil {
		return fmt.Errorf("cudos merge: failed to fund community pool: %w", err)
	}

	err = verifySupply(genesisData, cudosCfg, manifest)
	if err != nil {
		return fmt.Errorf("cudos merge: failed to verify supply: %w", err)
	}

	return nil
}

func parseGenesisData(jsonData map[string]interface{}, cudosCfg *CudosMergeConfig, manifest *UpgradeManifest) (*GenesisData, error) {
	genesisData := GenesisData{}

	totalSupply, err := parseGenesisTotalSupply(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to get total supply: %w", err)
	}
	genesisData.totalSupply = totalSupply

	genesisData.contracts, err = parseGenesisWasmContracts(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to get contracts: %w", err)
	}

	genesisData.ibcAccounts, err = parseGenesisIBCAccounts(jsonData, cudosCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get ibc accounts: %w", err)
	}

	// Get all accounts and balances into map
	genesisData.accounts, err = parseGenesisAccounts(jsonData, genesisData.contracts, genesisData.ibcAccounts, cudosCfg, manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts map: %w", err)
	}

	// Staking module
	bondedPoolAddress, err := GetAddressByName(genesisData.accounts, BondedPoolAccName)
	if err != nil {
		return nil, fmt.Errorf("failed to get bonded pool account: %w", err)
	}
	genesisData.bondedPoolAddress = bondedPoolAddress

	genesisData.notBondedPoolAddress, err = GetAddressByName(genesisData.accounts, NotBondedPoolAccName)
	if err != nil {
		return nil, fmt.Errorf("failed to get not-bonded pool account: %w", err)
	}

	genesisData.validators, err = parseGenesisValidators(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to get validators map: %w", err)
	}

	genesisData.delegations, err = parseGenesisDelegations(genesisData.validators, genesisData.contracts, cudosCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get delegations map: %w", err)
	}

	distributionInfo, err := parseGenesisDistribution(jsonData, genesisData.accounts)
	if err != nil {
		return nil, fmt.Errorf("failed to get distribution module map: %w", err)
	}
	genesisData.distributionInfo = distributionInfo

	gravityModuleAccountAddress, err := GetAddressByName(genesisData.accounts, GravityAccName)
	if err != nil {
		return nil, fmt.Errorf("failed to get gravity module account: %w", err)
	}
	genesisData.gravityModuleAccountAddress = gravityModuleAccountAddress

	genesisData.collisionMap = NewOrderedMap[string, string]()
	return &genesisData, nil
}

type AccountInfo struct {
	// Base
	pubkey     cryptotypes.PubKey
	address    string
	rawAddress sdk.AccAddress

	// Bank
	balance sdk.Coins

	// Module
	name string

	// BaseVesting
	endTime         int64
	originalVesting sdk.Coins
	//delegated_free
	//delegated_vesting

	// DelayedVesting
	// --

	// ContinuousVesting
	startTime int64

	// Custom
	accountType AccountType
	migrated    bool

	rawAccData map[string]interface{}
}

func parseGenesisBaseVesting(baseVestingAccData map[string]interface{}, accountInfo *AccountInfo, cudosCfg *CudosMergeConfig) error {
	// Parse specific base vesting account types
	accountInfo.endTime = cast.ToInt64(baseVestingAccData["end_time"].(string))

	originalVesting, err := getCoinsFromInterfaceSlice(baseVestingAccData["original_vesting"].([]interface{}))
	if err != nil {
		return err
	}
	accountInfo.originalVesting = originalVesting

	// Parse inner base account
	baseAccData := baseVestingAccData["base_account"].(map[string]interface{})
	err = parseGenesisBaseAccount(baseAccData, accountInfo, cudosCfg)
	if err != nil {
		return err
	}

	return nil
}

func parseGenesisBaseAccount(baseAccData map[string]interface{}, accountInfo *AccountInfo, cudosCfg *CudosMergeConfig) error {
	accountInfo.address = baseAccData["address"].(string)

	// Parse pubkey
	var AccPubKey cryptotypes.PubKey
	var err error
	if pk, ok := baseAccData["pub_key"]; ok {
		if pk != nil {
			AccPubKey, err = decodePubKeyFromMap(pk.(map[string]interface{}))
			if err != nil {
				return err
			}
		}
	}
	accountInfo.pubkey = AccPubKey

	// Get raw address
	accRawAddr, err := convertAddressToRaw(accountInfo.address, cudosCfg)
	accountInfo.rawAddress = accRawAddr
	if err != nil {
		return err
	}

	return nil
}

func parseGenesisDelayedVestingAccount(accMap map[string]interface{}, accountInfo *AccountInfo, cudosCfg *CudosMergeConfig) error {
	// Specific delayed vesting stuff
	// Nothing

	baseVestingAccData := accMap["base_vesting_account"].(map[string]interface{})
	err := parseGenesisBaseVesting(baseVestingAccData, accountInfo, cudosCfg)
	if err != nil {
		return err
	}

	return nil
}

func parseGenesisContinuousVestingAccount(accMap map[string]interface{}, accountInfo *AccountInfo, cudosCfg *CudosMergeConfig) error {
	// Specific continuous vesting stuff

	accountInfo.startTime = cast.ToInt64(accMap["start_time"].(string))

	baseVestingAccData := accMap["base_vesting_account"].(map[string]interface{})
	err := parseGenesisBaseVesting(baseVestingAccData, accountInfo, cudosCfg)
	if err != nil {
		return err
	}

	return nil
}

func parseGenesisPermanentLockedAccount(accMap map[string]interface{}, accountInfo *AccountInfo, cudosCfg *CudosMergeConfig) error {
	baseVestingAccData := accMap["base_vesting_account"].(map[string]interface{})
	err := parseGenesisBaseVesting(baseVestingAccData, accountInfo, cudosCfg)
	if err != nil {
		return err
	}

	return nil
}

func parseGenesisPeriodicVestingAccount(accMap map[string]interface{}, accountInfo *AccountInfo, cudosCfg *CudosMergeConfig) error {
	// Specific periodic stuff
	accountInfo.startTime = cast.ToInt64(accMap["start_time"].(string))

	// parse periods
	// Do we care?

	baseVestingAccData := accMap["base_vesting_account"].(map[string]interface{})
	err := parseGenesisBaseVesting(baseVestingAccData, accountInfo, cudosCfg)
	if err != nil {
		return err
	}

	return nil
}

func parseGenesisModuleAccount(accMap map[string]interface{}, accountInfo *AccountInfo, cudosCfg *CudosMergeConfig) error {
	// Specific module account values
	accountInfo.name = accMap["name"].(string)

	// parse inner base account
	baseAccData := accMap["base_account"].(map[string]interface{})
	err := parseGenesisBaseAccount(baseAccData, accountInfo, cudosCfg)
	if err != nil {
		return err
	}

	return nil
}

func parseGenesisAccount(accMap map[string]interface{}, cudosCfg *CudosMergeConfig) (*AccountInfo, error) {
	accountInfo := AccountInfo{balance: sdk.NewCoins(), migrated: false, rawAccData: accMap}
	accType := accMap["@type"]

	// Extract base account and special values
	if accType == ModuleAccount {
		err := parseGenesisModuleAccount(accMap, &accountInfo, cudosCfg)
		if err != nil {
			return nil, err
		}
		accountInfo.accountType = ModuleAccountType
	} else if accType == DelayedVestingAccount {
		err := parseGenesisDelayedVestingAccount(accMap, &accountInfo, cudosCfg)
		if err != nil {
			return nil, err
		}
		accountInfo.accountType = DelayedVestingAccountType
	} else if accType == ContinuousVestingAccount {
		err := parseGenesisContinuousVestingAccount(accMap, &accountInfo, cudosCfg)
		if err != nil {
			return nil, err
		}
		accountInfo.accountType = ContinuousVestingAccountType
	} else if accType == PermanentLockedAccount {
		err := parseGenesisPermanentLockedAccount(accMap, &accountInfo, cudosCfg)
		if err != nil {
			return nil, err
		}
		accountInfo.accountType = PermanentLockedAccountType
	} else if accType == PeriodicVestingAccount {
		err := parseGenesisPeriodicVestingAccount(accMap, &accountInfo, cudosCfg)
		if err != nil {
			return nil, err
		}
		accountInfo.accountType = PeriodicVestingAccountType
	} else if accType == BaseAccount {
		err := parseGenesisBaseAccount(accMap, &accountInfo, cudosCfg)
		if err != nil {
			return nil, err
		}
		accountInfo.accountType = BaseAccountType

	} else {
		return nil, fmt.Errorf("unknown account type %s", accType)
	}
	return &accountInfo, nil
}

func parseGenesisAccounts(jsonData map[string]interface{}, contractAccountMap *OrderedMap[string, *ContractInfo], IBCAccountsMap *OrderedMap[string, *IBCInfo], cudosCfg *CudosMergeConfig, manifest *UpgradeManifest) (*OrderedMap[string, *AccountInfo], error) {
	var err error

	// Map to verify that account exists in auth module
	auth := jsonData[authtypes.ModuleName].(map[string]interface{})
	accounts := auth["accounts"].([]interface{})
	accountMap := NewOrderedMap[string, *AccountInfo]()

	for _, acc := range accounts {
		accMap := acc.(map[string]interface{})
		accountInfo, err := parseGenesisAccount(accMap, cudosCfg)
		if err != nil {
			return nil, err
		}

		// Check if not contract or IBC type
		if _, exists := contractAccountMap.Get(accountInfo.address); exists {
			accountInfo.accountType = ContractAccountType
		} else if _, exists := IBCAccountsMap.Get(accountInfo.address); exists {
			accountInfo.accountType = IBCAccountType
		}

		accountMap.SetNew(accountInfo.address, accountInfo)
	}

	// Add balances to accounts map
	err = fillGenesisBalancesToAccountsMap(jsonData, accountMap, cudosCfg, manifest)
	if err != nil {
		return nil, err
	}

	return accountMap, nil
}

func parseGenesisDelegations(validators *OrderedMap[string, *ValidatorInfo], contracts *OrderedMap[string, *ContractInfo], cudosCfg *CudosMergeConfig) (*OrderedMap[string, *OrderedMap[string, sdk.Coins]], error) {
	// Handle delegations
	delegatedBalanceMap := NewOrderedMap[string, *OrderedMap[string, sdk.Coins]]()
	for _, validatorOperatorAddress := range validators.Keys() {
		validator := validators.MustGet(validatorOperatorAddress)
		for _, delegatorAddress := range validator.delegations.Keys() {
			delegation := validator.delegations.MustGet(delegatorAddress)
			resolvedDelegatorAddress, err := resolveIfContractAddressWithFallback(delegatorAddress, contracts, cudosCfg)
			if err != nil {
				return nil, err
			}

			currentValidatorInfo := validators.MustGet(validatorOperatorAddress)
			delegatorTokens := currentValidatorInfo.TokensFromShares(delegation.shares).TruncateInt()

			// Move balance to delegator address
			delegatorBalance := sdk.NewCoins(sdk.NewCoin(cudosCfg.config.OriginalDenom, delegatorTokens))

			if delegatorTokens.IsZero() {
				// This happens when number of shares is less than 1
				continue
			}

			// Subtract balance from bonded or not-bonded pool
			if currentValidatorInfo.status == BondedStatus {

				// Store delegation to delegated map
				if _, exists := delegatedBalanceMap.Get(resolvedDelegatorAddress); !exists {
					delegatedBalanceMap.Set(resolvedDelegatorAddress, NewOrderedMap[string, sdk.Coins]())
				}

				resolvedDelegatorMap := delegatedBalanceMap.MustGet(resolvedDelegatorAddress)

				if _, exists := resolvedDelegatorMap.Get(validatorOperatorAddress); !exists {
					resolvedDelegatorMap.Set(validatorOperatorAddress, sdk.NewCoins())
				}
				resolvedDelegator := resolvedDelegatorMap.MustGet(validatorOperatorAddress)

				resolvedDelegatorMap.Set(validatorOperatorAddress, resolvedDelegator.Add(delegatorBalance...))

				delegatedBalanceMap.Set(resolvedDelegatorAddress, resolvedDelegatorMap)
			}
		}
	}

	return delegatedBalanceMap, nil
}

type DelegationInfo struct {
	delegatorAddress string
	shares           sdk.Dec
}

type UnbondingDelegationInfo struct {
	delegatorAddress string
	entries          []*UnbondingDelegationEntry
}

type UnbondingDelegationEntry struct {
	balance        sdk.Int
	initialBalance sdk.Int
	creationHeight uint64
	completionTime string
}

type ValidatorInfo struct {
	stake                sdk.Int
	shares               sdk.Dec
	status               string
	operatorAddress      string
	consensusPubkey      cryptotypes.PubKey
	delegations          *OrderedMap[string, *DelegationInfo]
	unbondingDelegations *OrderedMap[string, *UnbondingDelegationInfo]
}

func (v ValidatorInfo) TokensFromShares(shares sdk.Dec) sdk.Dec {
	return (shares.MulInt(v.stake)).Quo(v.shares)
}

func parseGenesisValidators(jsonData map[string]interface{}) (*OrderedMap[string, *ValidatorInfo], error) {
	// Validator pubkey hex -> ValidatorInfo
	validatorInfoMap := NewOrderedMap[string, *ValidatorInfo]()

	staking := jsonData[stakingtypes.ModuleName].(map[string]interface{})
	validators := staking["validators"].([]interface{})

	for _, validator := range validators {

		validatorMap := validator.(map[string]interface{})
		tokens := validatorMap["tokens"].(string)
		operatorAddress := validator.(map[string]interface{})["operator_address"].(string)

		consensusPubkey := validator.(map[string]interface{})["consensus_pubkey"].(map[string]interface{})
		decodedConsensusPubkey, err := decodePubKeyFromMap(consensusPubkey)
		if err != nil {
			return nil, err
		}

		// Convert amount to big.Int
		tokensInt, ok := sdk.NewIntFromString(tokens)
		if !ok {
			return nil, fmt.Errorf("failed to convert validator tokens to big.Int")
		}

		status := validatorMap["status"].(string)

		validatorShares := validatorMap["delegator_shares"].(string)
		validatorSharesDec, err := sdk.NewDecFromStr(validatorShares)
		if err != nil {
			return nil, err
		}

		validatorInfoMap.SetNew(operatorAddress, &ValidatorInfo{
			stake:                tokensInt,
			shares:               validatorSharesDec,
			status:               status,
			operatorAddress:      operatorAddress,
			consensusPubkey:      decodedConsensusPubkey,
			delegations:          NewOrderedMap[string, *DelegationInfo](),
			unbondingDelegations: NewOrderedMap[string, *UnbondingDelegationInfo](),
		})

	}

	// Map of delegatorAddress -> validatorPubkey -> sdk.coins balance
	delegations := staking["delegations"].([]interface{})
	for _, delegation := range delegations {
		delegationMap := delegation.(map[string]interface{})
		delegatorAddress := delegationMap["delegator_address"].(string)
		validatorAddress := delegationMap["validator_address"].(string)

		delegatorSharesDec, err := sdk.NewDecFromStr(delegationMap["shares"].(string))
		if err != nil {
			return nil, err
		}

		validator := validatorInfoMap.MustGet(validatorAddress)
		validator.delegations.SetNew(delegatorAddress, &DelegationInfo{delegatorAddress: delegatorAddress, shares: delegatorSharesDec})
	}

	unbondingDelegations := staking["unbonding_delegations"].([]interface{})
	for _, unbondingDelegation := range unbondingDelegations {
		unbondingDelegationMap := unbondingDelegation.(map[string]interface{})
		delegatorAddress := unbondingDelegationMap["delegator_address"].(string)
		validatorAddress := unbondingDelegationMap["validator_address"].(string)

		entriesMap := unbondingDelegationMap["entries"].([]interface{})

		var unbondingDelegationEntries []*UnbondingDelegationEntry

		for _, entry := range entriesMap {
			entryMap := entry.(map[string]interface{})
			balance, ok := sdk.NewIntFromString(entryMap["balance"].(string))
			if !ok {
				return nil, fmt.Errorf("failed to convert unbonding delegation balance to int")
			}

			initialBalance, ok := sdk.NewIntFromString(entryMap["initial_balance"].(string))
			if !ok {
				return nil, fmt.Errorf("failed to convert unbonding delegation initial balance to int")
			}

			creationHeight := cast.ToUint64(entryMap["creation_height"].(string))

			completionTime := entryMap["completion_time"].(string)

			unbondingDelegationEntries = append(unbondingDelegationEntries, &UnbondingDelegationEntry{balance: balance, initialBalance: initialBalance, creationHeight: creationHeight, completionTime: completionTime})
		}

		validator := validatorInfoMap.MustGet(validatorAddress)
		validator.unbondingDelegations.SetNew(delegatorAddress, &UnbondingDelegationInfo{delegatorAddress: delegatorAddress, entries: unbondingDelegationEntries})
	}

	return validatorInfoMap, nil
}

func withdrawGenesisStakingDelegations(app *App, genesisData *GenesisData, cudosCfg *CudosMergeConfig, manifest *UpgradeManifest) error {
	// Handle delegations
	for _, validatorOperatorAddress := range genesisData.validators.Keys() {
		validator := genesisData.validators.MustGet(validatorOperatorAddress)
		for _, delegatorAddress := range validator.delegations.Keys() {
			delegation := validator.delegations.MustGet(delegatorAddress)
			resolvedDelegatorAddress, err := resolveIfContractAddressWithFallback(delegatorAddress, genesisData.contracts, cudosCfg)
			if err != nil {
				return err
			}

			currentValidatorInfo := genesisData.validators.MustGet(validatorOperatorAddress)
			delegatorTokens := currentValidatorInfo.TokensFromShares(delegation.shares).TruncateInt()

			// Move balance to delegator address
			delegatorBalance := sdk.NewCoins(sdk.NewCoin(cudosCfg.config.OriginalDenom, delegatorTokens))

			if delegatorTokens.IsZero() {
				// This happens when number of shares is less than 1
				continue
			}

			// Subtract balance from bonded or not-bonded pool
			if currentValidatorInfo.status == BondedStatus {
				// Move balance from bonded pool to delegator
				err := moveGenesisBalance(genesisData, genesisData.bondedPoolAddress, resolvedDelegatorAddress, delegatorBalance, "bonded_delegation", manifest, cudosCfg)
				if err != nil {
					return err
				}

			} else {
				// Delegations to unbonded/jailed/tombstoned validators are not re-delegated

				// Move balance from not-bonded pool to delegator
				err := moveGenesisBalance(genesisData, genesisData.notBondedPoolAddress, resolvedDelegatorAddress, delegatorBalance, "not_bonded_delegation", manifest, cudosCfg)
				if err != nil {
					return err
				}
			}

		}

		// Handle unbonding delegations
		for _, delegatorAddress := range validator.unbondingDelegations.Keys() {
			unbondingDelegation := validator.unbondingDelegations.MustGet(delegatorAddress)
			resolvedDelegatorAddress, err := resolveIfContractAddressWithFallback(delegatorAddress, genesisData.contracts, cudosCfg)
			if err != nil {
				return err
			}

			for _, entry := range unbondingDelegation.entries {
				unbondingDelegationBalance := sdk.NewCoins(sdk.NewCoin(cudosCfg.config.OriginalDenom, entry.balance))

				// Move unbonding balance from not-bonded pool to delegator address
				err := moveGenesisBalance(genesisData, genesisData.notBondedPoolAddress, resolvedDelegatorAddress, unbondingDelegationBalance, "unbonding_delegation", manifest, cudosCfg)
				if err != nil {
					return err
				}

			}
		}
	}

	// Handle remaining pool balances

	// Handle remaining bonded pool balance
	bondedPool := genesisData.accounts.MustGet(genesisData.bondedPoolAddress)

	// TODO: Write to manifest?
	err := checkTolerance(bondedPool.balance, maxToleratedRemainingStakingBalance)
	if err != nil {
		return fmt.Errorf("remaining bonded pool balance %s is too high", bondedPool.balance.String())
	}

	app.Logger().Info("cudos merge: remaining bonded pool balance", "amount", bondedPool.balance.String())
	err = moveGenesisBalance(genesisData, genesisData.bondedPoolAddress, cudosCfg.config.RemainingStakingBalanceAddr, bondedPool.balance, "remaining_bonded_pool_balance", manifest, cudosCfg)
	if err != nil {
		return err
	}

	// Handle remaining not-bonded pool balance
	notBondedPool := genesisData.accounts.MustGet(genesisData.notBondedPoolAddress)

	// TODO: Write to manifest?
	err = checkTolerance(notBondedPool.balance, maxToleratedRemainingStakingBalance)
	if err != nil {
		return fmt.Errorf("remaining not-bonded pool balance %s is too high", notBondedPool.balance.String())
	}

	app.Logger().Info("cudos merge: remaining not-bonded pool balance", "amount", notBondedPool.balance.String())
	err = moveGenesisBalance(genesisData, genesisData.notBondedPoolAddress, cudosCfg.config.RemainingStakingBalanceAddr, notBondedPool.balance, "remaining_not_bonded_pool_balance", manifest, cudosCfg)
	if err != nil {
		return err
	}

	return nil
}

func resolveDestinationValidator(ctx sdk.Context, app *App, operatorAddress string, cudosCfg *CudosMergeConfig) (*stakingtypes.Validator, error) {
	if targetOperatorStringAddress, exists := cudosCfg.validatorsMap.Get(operatorAddress); exists {
		targetOperatorAddress, err := sdk.ValAddressFromBech32(targetOperatorStringAddress)
		if err != nil {
			return nil, err
		}

		if targetValidator, found := app.StakingKeeper.GetValidator(ctx, targetOperatorAddress); found {
			if targetValidator.Status.String() == BondedStatus && !targetValidator.Jailed {
				return &targetValidator, nil
			}
		}

	}

	for _, targetOperatorStringAddress := range cudosCfg.config.BackupValidators {
		targetOperatorAddress, err := sdk.ValAddressFromBech32(targetOperatorStringAddress)
		if err != nil {
			return nil, err
		}

		if targetValidator, found := app.StakingKeeper.GetValidator(ctx, targetOperatorAddress); found {
			if targetValidator.Status.String() == BondedStatus && !targetValidator.Jailed {
				return &targetValidator, nil
			}
		}
	}

	return nil, fmt.Errorf("failed to resolve validator")
}

func getIntAmountFromCoins(balance sdk.Coins, expectedDenom string) (*sdk.Int, error) {
	coin := balance.AmountOf(expectedDenom)
	if coin.IsZero() {
		return nil, fmt.Errorf("denom %s not found in balance", expectedDenom)
	}
	return &coin, nil
}

func createDelegation(ctx sdk.Context, app *App, originalValidator string, newDelegatorRawAddr sdk.AccAddress, validator stakingtypes.Validator, originalBalance sdk.Coins, tokensToDelegate sdk.Int, manifest *UpgradeManifest) error {

	newShares, err := app.StakingKeeper.Delegate(ctx, newDelegatorRawAddr, tokensToDelegate, stakingtypes.Unbonded, validator, true)
	if err != nil {
		return err
	}

	if manifest.Delegate == nil {
		manifest.Delegate = &UpgradeDelegate{}
	}

	delegation := UpgradeDelegation{
		NewDelegator:      newDelegatorRawAddr.String(),
		NewValidator:      validator.OperatorAddress,
		OriginalTokens:    originalBalance,
		NewTokens:         tokensToDelegate,
		NewShares:         newShares,
		OriginalValidator: originalValidator,
	}
	manifest.Delegate.Delegations = append(manifest.Delegate.Delegations, delegation)

	if manifest.Delegate.AggregatedDelegatedAmount == nil {
		manifest.Delegate.AggregatedDelegatedAmount = &tokensToDelegate
	} else {
		*manifest.Delegate.AggregatedDelegatedAmount = manifest.Delegate.AggregatedDelegatedAmount.Add(tokensToDelegate)
	}

	manifest.Delegate.NumberOfDelegations = len(manifest.Delegate.Delegations)

	return nil
}

func fundCommunityPool(ctx sdk.Context, app *App, genesisData *GenesisData, cudosCfg *CudosMergeConfig, manifest *UpgradeManifest) error {
	// Fund community pool
	communityPoolBalance, _ := genesisData.distributionInfo.feePool.communityPool.TruncateDecimal()
	convertedCommunityPoolBalance, err := convertBalance(communityPoolBalance, cudosCfg)
	if err != nil {
		return err
	}

	communityPoolSourceAccountRawAddress := genesisData.accounts.MustGet(cudosCfg.config.RemainingDistributionBalanceAddr).rawAddress

	if cudosCfg.config.CommunityPoolBalanceDestAddr == "" {
		// Move balance to community pool if destination address is not set

		err = app.DistrKeeper.FundCommunityPool(ctx, convertedCommunityPoolBalance, communityPoolSourceAccountRawAddress)
		if err != nil {
			return err
		}

		registerBalanceMovement(cudosCfg.config.RemainingDistributionBalanceAddr, distrtypes.ModuleName, communityPoolBalance, convertedCommunityPoolBalance, "community_pool_balance", manifest)

	} else {
		// Move balance to given account

		destAccRawAddr, err := convertAddressToRaw(cudosCfg.config.CommunityPoolBalanceDestAddr, cudosCfg)
		if err != nil {
			return err
		}

		err = app.BankKeeper.SendCoins(ctx, communityPoolSourceAccountRawAddress, destAccRawAddr, convertedCommunityPoolBalance)
		if err != nil {
			return err
		}
		registerBalanceMovement(cudosCfg.config.RemainingDistributionBalanceAddr, cudosCfg.config.CommunityPoolBalanceDestAddr, communityPoolBalance, convertedCommunityPoolBalance, "community_pool_balance", manifest)

	}

	return nil
}

func createGenesisDelegations(ctx sdk.Context, app *App, genesisData *GenesisData, cudosCfg *CudosMergeConfig, manifest *UpgradeManifest) error {

	for _, delegatorAddr := range genesisData.delegations.Keys() {
		delegatorAddrMap := genesisData.delegations.MustGet(delegatorAddr)

		// Skip accounts that shouldn't be delegated
		if cudosCfg.notDelegatedAccounts.Has(delegatorAddr) {
			continue
		}

		for _, validatorOperatorStringAddr := range delegatorAddrMap.Keys() {
			delegatedBalance := delegatorAddrMap.MustGet(validatorOperatorStringAddr)

			destValidator, err := resolveDestinationValidator(ctx, app, validatorOperatorStringAddr, cudosCfg)
			if err != nil {
				return err
			}

			// Get int amount in native tokens
			convertedBalance, err := convertBalance(delegatedBalance, cudosCfg)
			if err != nil {
				return err
			}

			if convertedBalance.Empty() {
				// Very small balance gets truncated to 0 during conversion
				continue
			}

			tokensToDelegate, err := getIntAmountFromCoins(convertedBalance, cudosCfg.config.StakingDenom)
			if err != nil {
				return err
			}

			var delegatorRawAddr []byte
			if remappedDelegatorAddr, exists := genesisData.collisionMap.Get(delegatorAddr); exists {
				// Vesting collision
				_, delegatorRawAddr, err = bech32.DecodeAndConvert(remappedDelegatorAddr)
				if err != nil {
					return err
				}
			} else {
				// Regular case
				delegatorRawAddr, err = convertAddressToRaw(delegatorAddr, cudosCfg)
				if err != nil {
					return err
				}
			}

			err = createDelegation(ctx, app, validatorOperatorStringAddr, delegatorRawAddr, *destValidator, delegatedBalance, *tokensToDelegate, manifest)
			if err != nil {
				return err
			}

		}
	}

	return nil
}

func getCoinsFromInterfaceSlice(coins []interface{}) (sdk.Coins, error) {
	var resBalance sdk.Coins
	for _, coin := range coins {

		amount := coin.(map[string]interface{})["amount"].(string)

		denom := coin.(map[string]interface{})["denom"].(string)

		sdkAmount, ok := sdk.NewIntFromString(amount)
		if !ok {
			return nil, fmt.Errorf("failed to convert amount to sdk.Int")
		}

		sdkCoin := sdk.NewCoin(denom, sdkAmount)
		resBalance = resBalance.Add(sdkCoin)

	}

	return resBalance, nil
}

func getDecCoinsFromInterfaceSlice(coins []interface{}) (sdk.DecCoins, error) {
	var resBalance sdk.DecCoins
	for _, coin := range coins {

		amount := coin.(map[string]interface{})["amount"].(string)

		denom := coin.(map[string]interface{})["denom"].(string)

		sdkAmount, err := sdk.NewDecFromStr(amount)
		if err != nil {
			return nil, fmt.Errorf("failed to convert amount to sdk.Dec")
		}

		sdkCoin := sdk.NewDecCoinFromDec(denom, sdkAmount)
		resBalance = resBalance.Add(sdkCoin)

	}

	return resBalance, nil
}

func getInterfaceSliceFromCoins(coins sdk.Coins) []interface{} {
	var balance []interface{}
	for _, coin := range coins {
		balance = append(balance, map[string]interface{}{
			"denom":  coin.Denom,
			"amount": coin.Amount.String(),
		})
	}
	return balance
}

func withdrawGenesisContractBalances(genesisData *GenesisData, manifest *UpgradeManifest, cudosCfg *CudosMergeConfig) error {

	for _, contractAddress := range genesisData.contracts.Keys() {
		resolvedAddress, err := resolveIfContractAddressWithFallback(contractAddress, genesisData.contracts, cudosCfg)
		if err != nil {
			return err
		}

		contractBalance, contractBalancePresent := genesisData.accounts.Get(contractAddress)
		if contractBalancePresent {
			err := moveGenesisBalance(genesisData, contractAddress, resolvedAddress, contractBalance.balance, "contract_balance", manifest, cudosCfg)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func convertBalance(balance sdk.Coins, cudosCfg *CudosMergeConfig) (sdk.Coins, error) {
	var resBalance sdk.Coins

	for _, coin := range balance {
		if conversionConstant, exists := cudosCfg.balanceConversionConstants.Get(coin.Denom); exists {
			newAmount := coin.Amount.ToDec().Quo(conversionConstant).TruncateInt()
			sdkCoin := sdk.NewCoin(cudosCfg.config.ConvertedDenom, newAmount)
			resBalance = resBalance.Add(sdkCoin)
		}
		// Denominations that are not in conversion constant map are ignored
	}

	return resBalance, nil
}

func ensureAccount(addrStr string, genesisAccountsMap *OrderedMap[string, *AccountInfo], reason string, cudosCfg *CudosMergeConfig, manifest *UpgradeManifest) error {
	// Create new account if it doesn't exist
	if genesisAccountsMap.Has(addrStr) {
		// Already exist
		return nil
	}

	accRawAddress, err := convertAddressToRaw(addrStr, cudosCfg)
	if err != nil {
		return err
	}
	accountInfoEntry := &AccountInfo{
		rawAddress:  accRawAddress,
		address:     addrStr,
		accountType: BaseAccountType,
	}

	genesisAccountsMap.Set(addrStr, accountInfoEntry)

	if manifest.CreatedAccounts == nil {
		manifest.CreatedAccounts = &UpgradeCreatedAccounts{}
	}
	manifest.CreatedAccounts.Accounts = append(manifest.CreatedAccounts.Accounts, UpgradeAccountCreation{Address: addrStr, Reason: reason})
	manifest.CreatedAccounts.NumberOfCreations = len(manifest.CreatedAccounts.Accounts)

	return nil
}

func fillGenesisBalancesToAccountsMap(jsonData map[string]interface{}, genesisAccountsMap *OrderedMap[string, *AccountInfo], cudosCfg *CudosMergeConfig, manifest *UpgradeManifest) error {
	bank := jsonData[banktypes.ModuleName].(map[string]interface{})
	balances := bank["balances"].([]interface{})

	for _, balance := range balances {

		addr := balance.(map[string]interface{})["address"]
		if addr == nil {
			return fmt.Errorf("failed to get address")
		}
		addrStr := addr.(string)

		coins := balance.(map[string]interface{})["coins"]

		sdkBalance, err := getCoinsFromInterfaceSlice(coins.([]interface{}))
		if err != nil {
			return err
		}

		convertedBalance, err := convertBalance(sdkBalance, cudosCfg)
		if err != nil {
			return err
		}

		if !convertedBalance.IsZero() {
			// Create new account if it doesn't exist
			err := ensureAccount(addrStr, genesisAccountsMap, "bank_balance_no_auth_acc", cudosCfg, manifest)
			if err != nil {
				return err
			}
			accountInfoEntry := genesisAccountsMap.MustGet(addrStr)
			accountInfoEntry.balance = sdkBalance
			genesisAccountsMap.Set(addrStr, accountInfoEntry)
		}

	}
	return nil
}

func genesisUpgradeWithdrawIBCChannelsBalances(genesisData *GenesisData, cudosCfg *CudosMergeConfig, manifest *UpgradeManifest) error {
	if cudosCfg.config.IbcTargetAddr == "" {
		return fmt.Errorf("no IBC withdrawal address set")
	}

	ibcWithdrawalAddress := cudosCfg.config.IbcTargetAddr

	manifest.IBC = &UpgradeIBCTransfers{
		To: ibcWithdrawalAddress,
	}

	for _, IBCaccountAddress := range genesisData.ibcAccounts.Keys() {

		IBCaccount, IBCAccountExists := genesisData.accounts.Get(IBCaccountAddress)
		IBCinfo := genesisData.ibcAccounts.MustGet(IBCaccountAddress)

		var channelBalance sdk.Coins
		if IBCAccountExists {

			channelBalance = IBCaccount.balance
			err := moveGenesisBalance(genesisData, IBCaccountAddress, ibcWithdrawalAddress, channelBalance, "ibc_balance", manifest, cudosCfg)
			if err != nil {
				return err
			}
		}

		manifest.IBC.Transfers = append(manifest.IBC.Transfers, UpgradeIBCTransfer{From: IBCaccountAddress, ChannelID: fmt.Sprintf("%s/%s", IBCinfo.portId, IBCinfo.channelId), Amount: channelBalance})
		manifest.IBC.AggregatedTransferredAmount = manifest.IBC.AggregatedTransferredAmount.Add(channelBalance...)
		manifest.IBC.NumberOfTransfers = len(manifest.IBC.Transfers)
	}

	return nil
}

type IBCInfo struct {
	channelId string
	portId    string
}

func parseGenesisIBCAccounts(jsonData map[string]interface{}, cudosCfg *CudosMergeConfig) (*OrderedMap[string, *IBCInfo], error) {
	ibcAccountMap := NewOrderedMap[string, *IBCInfo]()

	ibc, ok := jsonData[ibccore.ModuleName].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("IBC module data not found in genesis")
	}

	channelGenesis, ok := ibc["channel_genesis"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("channel genesis data not found in IBC module")
	}

	ibcChannels, ok := channelGenesis["channels"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("channels data not found in channel genesis")
	}

	for _, channel := range ibcChannels {
		channelMap, ok := channel.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid channel format in genesis")
		}

		channelId, ok := channelMap["channel_id"].(string)
		if !ok {
			return nil, fmt.Errorf("channel_id not found or invalid in channel")
		}

		portId, ok := channelMap["port_id"].(string)
		if !ok {
			return nil, fmt.Errorf("port_id not found or invalid in channel")
		}

		rawAddr := ibctransfertypes.GetEscrowAddress(portId, channelId)
		channelAddr, err := sdk.Bech32ifyAddressBytes(cudosCfg.config.OldAddrPrefix, rawAddr)
		if err != nil {
			return nil, err
		}

		ibcAccountMap.Set(channelAddr, &IBCInfo{channelId: channelId, portId: portId})
	}

	return ibcAccountMap, nil
}

type ContractInfo struct {
	Admin   string
	Creator string
}

func parseGenesisWasmContracts(jsonData map[string]interface{}) (*OrderedMap[string, *ContractInfo], error) {
	contractAccountMap := NewOrderedMap[string, *ContractInfo]()

	// Navigate to the "wasm" module
	wasm, ok := jsonData["wasm"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("wasm module data not found in genesis")
	}

	// Navigate to the "contracts" section
	contracts, ok := wasm["contracts"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("contracts data not found in wasm module")
	}

	// Iterate over each contract to get the "contract_address"
	for _, contract := range contracts {
		contractMap, ok := contract.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid contract format in genesis")
		}

		contractAddr, ok := contractMap["contract_address"].(string)
		if !ok {
			return nil, fmt.Errorf("contract_address not found or invalid in contract")
		}

		contractInfo, ok := contractMap["contract_info"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("contract_info not found or invalid in contract")
		}

		admin := contractInfo["admin"].(string)
		creator := contractInfo["creator"].(string)

		contractAccountMap.Set(contractAddr, &ContractInfo{Admin: admin, Creator: creator})
	}

	return contractAccountMap, nil
}

func resolveIfContractAddressWithFallback(address string, contracts *OrderedMap[string, *ContractInfo], cudosCfg *CudosMergeConfig) (string, error) {

	resolvedAddress, err := resolveIfContractAddress(address, contracts)
	if err != nil {
		return "", err
	}

	if resolvedAddress == nil || strings.TrimSpace(*resolvedAddress) == "" {
		// Use fallback address
		return cudosCfg.config.ContractDestinationFallbackAddr, nil
	} else {
		// Use resolved address
		return *resolvedAddress, nil
	}
}

func resolveIfContractAddress(address string, contracts *OrderedMap[string, *ContractInfo]) (*string, error) {
	adminsMap := map[string]bool{}
	creatorsMap := map[string]bool{}

	for {
		contractInfo, exists := contracts.Get(address)
		if !exists {
			return &address, nil
		}
		// If the contract has an admin that is not itself, continue with the admin address.
		if len(creatorsMap) == 0 && len(adminsMap) < RecursionDepthLimit && contractInfo.Admin != "" && contractInfo.Admin != address && !adminsMap[contractInfo.Admin] {
			adminsMap[contractInfo.Admin] = true
			address = contractInfo.Admin
		} else if len(creatorsMap) < RecursionDepthLimit && contractInfo.Creator != "" && !creatorsMap[contractInfo.Creator] {
			// Otherwise, if the creator is present, continue with the creator address.
			creatorsMap[contractInfo.Creator] = true
			address = contractInfo.Creator
		} else {
			// Failed to resolve
			return nil, nil
		}
	}
}

func decodePubKeyFromMap(pubKeyMap map[string]interface{}) (cryptotypes.PubKey, error) {
	keyType, ok := pubKeyMap["@type"].(string)
	if !ok {
		return nil, fmt.Errorf("@type field not found or is not a string in pubKeyMap")
	}

	switch keyType {
	case "/cosmos.crypto.secp256k1.PubKey":
		keyStr, ok := pubKeyMap["key"].(string)
		if !ok {
			return nil, fmt.Errorf("key field not found or is not a string in pubKeyMap")
		}

		keyBytes, err := base64.StdEncoding.DecodeString(keyStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 key: %w", err)
		}

		// Ensure the byte slice is the correct length for a secp256k1 public key
		if len(keyBytes) != secp256k1.PubKeySize {
			return nil, fmt.Errorf("invalid pubkey length: got %d, expected %d", len(keyBytes), secp256k1.PubKeySize)
		}

		pubKey := secp256k1.PubKey{
			Key: keyBytes,
		}
		return &pubKey, nil

	case "/cosmos.crypto.ed25519.PubKey":
		keyStr, ok := pubKeyMap["key"].(string)
		if !ok {
			return nil, fmt.Errorf("key field not found or is not a string in pubKeyMap")
		}

		keyBytes, err := base64.StdEncoding.DecodeString(keyStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 key: %w", err)
		}

		// Ensure the byte slice is the correct length for an ed25519 public key
		if len(keyBytes) != ed25519.PubKeySize {
			return nil, fmt.Errorf("invalid pubkey length: got %d, expected %d", len(keyBytes), ed25519.PubKeySize)
		}

		pubKey := ed25519.PubKey{
			Key: keyBytes,
		}
		return &pubKey, nil

	case "/cosmos.crypto.multisig.LegacyAminoPubKey":
		threshold, ok := pubKeyMap["threshold"].(float64) // JSON numbers are float64
		if !ok {
			return nil, fmt.Errorf("threshold field not found or is not a number in pubKeyMap")
		}

		pubKeysInterface, ok := pubKeyMap["public_keys"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("public_keys field not found or is not an array in pubKeyMap")
		}

		var pubKeys []cryptotypes.PubKey
		for _, pubKeyInterface := range pubKeysInterface {
			pubKeyMap, ok := pubKeyInterface.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("public key entry is not a valid map")
			}

			pubKey, err := decodePubKeyFromMap(pubKeyMap)
			if err != nil {
				return nil, fmt.Errorf("failed to decode public key: %w", err)
			}

			pubKeys = append(pubKeys, pubKey)
		}

		legacyAminoPubKey := multisig.NewLegacyAminoPubKey(int(threshold), pubKeys)
		return legacyAminoPubKey, nil

	default:
		return nil, fmt.Errorf("unsupported key type: %s", keyType)
	}
}

func getNewBaseAccount(ctx sdk.Context, app *App, accountInfo *AccountInfo) (*authtypes.BaseAccount, error) {
	// Create new account
	newAccNumber := app.AccountKeeper.GetNextAccountNumber(ctx)
	newBaseAccount := authtypes.NewBaseAccount(accountInfo.rawAddress, accountInfo.pubkey, newAccNumber, 0)
	return newBaseAccount, nil
}

func createNewVestingAccountFromBaseAccount(ctx sdk.Context, app *App, account *authtypes.BaseAccount, vestedCoins sdk.Coins, startTime int64, endTime int64) error {
	newBaseVestingAcc := authvesting.NewBaseVestingAccount(account, vestedCoins, endTime)
	newContinuousVestingAcc := authvesting.NewContinuousVestingAccountRaw(newBaseVestingAcc, startTime)

	app.AccountKeeper.SetAccount(ctx, newContinuousVestingAcc)

	return nil
}

func createNewNormalAccountFromBaseAccount(ctx sdk.Context, app *App, account *authtypes.BaseAccount) error {
	app.AccountKeeper.SetAccount(ctx, account)

	return nil
}

func migrateToAccount(ctx sdk.Context, app *App, fromAddress string, toAddress sdk.AccAddress, sourceCoins sdk.Coins, destCoins sdk.Coins, memo string, manifest *UpgradeManifest) error {

	err := app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, toAddress, destCoins)
	if err != nil {
		return err
	}

	if manifest.Migration == nil {
		manifest.Migration = &UpgradeMigation{}
	}

	migrate := UpgradeBalanceMovement{
		From:          fromAddress,
		To:            toAddress.String(),
		SourceBalance: sourceCoins,
		DestBalance:   destCoins,
		Memo:          memo,
	}
	manifest.Migration.Migrations = append(manifest.Migration.Migrations, migrate)

	manifest.Migration.AggregatedMigratedAmount = manifest.Migration.AggregatedMigratedAmount.Add(destCoins...)
	manifest.Migration.NumberOfMigrations = len(manifest.Migration.Migrations)

	return nil
}

func markAccountAsMigrated(genesisData *GenesisData, accountAddress string) error {
	AccountInfoRecord, exists := genesisData.accounts.Get(accountAddress)
	if !exists {
		return fmt.Errorf("genesis account %s not found", accountAddress)
	}

	if AccountInfoRecord.migrated {
		return fmt.Errorf("genesis account %s already migrated", accountAddress)
	}

	AccountInfoRecord.migrated = true

	genesisData.accounts.Set(accountAddress, AccountInfoRecord)

	return nil
}

func registerBalanceMovement(fromAddress, toAddress string, sourceAmount sdk.Coins, destAmount sdk.Coins, memo string, manifest *UpgradeManifest) {

	if manifest.MoveMintedBalance == nil {
		manifest.MoveMintedBalance = &UpgradeMoveMintedBalance{}
	}

	movement := UpgradeBalanceMovement{
		From:          fromAddress,
		To:            toAddress,
		SourceBalance: sourceAmount,
		DestBalance:   destAmount,
		Memo:          memo,
	}
	manifest.MoveMintedBalance.Movements = append(manifest.MoveMintedBalance.Movements, movement)
}

func registerManifestMoveDelegations(fromAddress, toAddress string, memo string, manifest *UpgradeManifest) {
	if manifest.MoveDelegations == nil {
		manifest.MoveDelegations = &UpgradeMoveDelegations{}
	}

	movement := UpgradeDelegationMovements{
		From: fromAddress,
		To:   toAddress,
		Memo: memo,
	}
	manifest.MoveDelegations.Movements = append(manifest.MoveDelegations.Movements, movement)
	manifest.MoveDelegations.NumberOfMovements = len(manifest.MoveDelegations.Movements)
}

func moveGenesisDelegations(genesisData *GenesisData, fromAddress, toAddress string, manifest *UpgradeManifest) error {
	sourceDelegations, exists := genesisData.delegations.Get(fromAddress)

	if !exists {
		registerManifestMoveDelegations(fromAddress, toAddress, "no_delegations", manifest)
		// Nothing to move
		return nil
	}

	if destDelegation, destDelegationExists := genesisData.delegations.Get(toAddress); destDelegationExists {
		// Add delegations - destination delegator has some delegations

		// List all validator addresses in source delegation
		for _, validatorAddr := range sourceDelegations.Keys() {
			srcCoins := sourceDelegations.MustGet(validatorAddr)

			// Same validator exists in destination delegation - Add coins
			if destCoins, destValidatorExists := destDelegation.Get(validatorAddr); destValidatorExists {
				destDelegation.Set(validatorAddr, destCoins.Add(srcCoins...))

			} else {
				// Validator doesn't exist on destination delegation, add it
				destDelegation.Set(validatorAddr, srcCoins)
			}

		}

	} else {
		// Destination has no delegations, just move source delegations pointer
		genesisData.delegations.Set(toAddress, sourceDelegations)
	}

	// Delete all source delegations
	genesisData.delegations.Delete(fromAddress)

	registerManifestMoveDelegations(fromAddress, toAddress, "", manifest)

	return nil
}

func registerManifestBalanceMovement(fromAddress, toAddress string, amount sdk.Coins, memo string, manifest *UpgradeManifest) {
	if manifest.MoveGenesisBalance == nil {
		manifest.MoveGenesisBalance = &UpgradeMoveGenesisBalance{}
	}

	movement := UpgradeBalanceMovement{
		From:        fromAddress,
		To:          toAddress,
		DestBalance: amount,
		Memo:        memo,
	}
	manifest.MoveGenesisBalance.Movements = append(manifest.MoveGenesisBalance.Movements, movement)

	manifest.MoveGenesisBalance.AggregatedMovedAmount = manifest.MoveGenesisBalance.AggregatedMovedAmount.Add(amount...)
	manifest.MoveGenesisBalance.NumberOfMovements = len(manifest.MoveGenesisBalance.Movements)

}

func moveGenesisBalance(genesisData *GenesisData, fromAddress, toAddress string, amount sdk.Coins, memo string, manifest *UpgradeManifest, cudosCfg *CudosMergeConfig) error {
	// Check if fromAddress exists
	if _, ok := genesisData.accounts.Get(fromAddress); !ok {
		return fmt.Errorf("fromAddress %s does not exist in genesis balances", fromAddress)
	}

	// Create to account if it doesn't exist
	err := ensureAccount(toAddress, genesisData.accounts, "balance_movement_destination", cudosCfg, manifest)
	if err != nil {
		return err
	}

	if toAcc := genesisData.accounts.MustGet(toAddress); toAcc.migrated {
		return fmt.Errorf("genesis account %s already migrated", toAddress)
	}
	if fromAcc := genesisData.accounts.MustGet(fromAddress); fromAcc.migrated {
		return fmt.Errorf("genesis account %s already migrated", fromAddress)
	}

	genesisToBalance := genesisData.accounts.MustGet(toAddress)
	genesisFromBalance := genesisData.accounts.MustGet(fromAddress)

	genesisToBalance.balance = genesisToBalance.balance.Add(amount...)
	genesisFromBalance.balance = genesisFromBalance.balance.Sub(amount)

	genesisData.accounts.Set(toAddress, genesisToBalance)
	genesisData.accounts.Set(fromAddress, genesisFromBalance)

	registerManifestBalanceMovement(fromAddress, toAddress, amount, memo, manifest)

	return nil
}

func createGenesisBalance(genesisData *GenesisData, toAddress string, amount sdk.Coins, memo string, manifest *UpgradeManifest, cudosCfg *CudosMergeConfig) error {
	// Create to account if it doesn't exist
	err := ensureAccount(toAddress, genesisData.accounts, "balance_creation_destination", cudosCfg, manifest)
	if err != nil {
		return err
	}

	if toAcc := genesisData.accounts.MustGet(toAddress); toAcc.migrated {
		return fmt.Errorf("genesis account %s already migrated", toAddress)
	}

	genesisToBalance := genesisData.accounts.MustGet(toAddress)

	genesisToBalance.balance = genesisToBalance.balance.Add(amount...)

	genesisData.accounts.Set(toAddress, genesisToBalance)

	registerManifestBalanceMovement("", toAddress, amount, memo, manifest)

	return nil
}

func GetAddressByName(genesisAccounts *OrderedMap[string, *AccountInfo], name string) (string, error) {

	for _, accAddress := range genesisAccounts.Keys() {
		acc := genesisAccounts.MustGet(accAddress)

		if acc.name == name {
			return accAddress, nil
		}

	}

	return "", fmt.Errorf("address not found in genesis accounts: %s", name)
}

func checkDecTolerance(coins sdk.DecCoins, maxToleratedDiff sdk.Int) error {
	for _, coin := range coins {
		if coin.Amount.TruncateInt().GT(maxToleratedDiff) {
			return fmt.Errorf("remaining balance %s is too high", coin.String())
		}
	}
	return nil
}

func withdrawGenesisGravity(genesisData *GenesisData, cudosCfg *CudosMergeConfig, manifest *UpgradeManifest) error {

	gravityBalance := genesisData.accounts.MustGet(genesisData.gravityModuleAccountAddress).balance
	err := moveGenesisBalance(genesisData, genesisData.gravityModuleAccountAddress, cudosCfg.config.RemainingGravityBalanceAddr, gravityBalance, "gravity_balance", manifest, cudosCfg)
	if err != nil {
		return err
	}

	return nil
}

func accountIToAccountInfo(existingAccount authtypes.AccountI) (*AccountInfo, error) {
	accountInfo := AccountInfo{}

	// Get existing account type
	if existingAccount != nil {
		accountInfo.pubkey = existingAccount.GetPubKey()
		accountInfo.rawAddress = existingAccount.GetAddress()
		accountInfo.address = accountInfo.rawAddress.String()

		if periodicVestingAccount, ok := existingAccount.(*authvesting.PeriodicVestingAccount); ok {
			accountInfo.accountType = PeriodicVestingAccountType
			accountInfo.endTime = periodicVestingAccount.EndTime
			accountInfo.originalVesting = periodicVestingAccount.OriginalVesting
		} else if delayedVestingAccount, ok := existingAccount.(*authvesting.DelayedVestingAccount); ok {
			accountInfo.accountType = DelayedVestingAccountType
			accountInfo.endTime = delayedVestingAccount.EndTime
			accountInfo.originalVesting = delayedVestingAccount.OriginalVesting
		} else if continuousVestingAccount, ok := existingAccount.(*authvesting.ContinuousVestingAccount); ok {
			accountInfo.accountType = ContinuousVestingAccountType
			accountInfo.endTime = continuousVestingAccount.EndTime
			accountInfo.startTime = continuousVestingAccount.StartTime
			accountInfo.originalVesting = continuousVestingAccount.OriginalVesting
		} else if permanentLockedAccount, ok := existingAccount.(*authvesting.PermanentLockedAccount); ok {
			accountInfo.accountType = PermanentLockedAccount
			accountInfo.originalVesting = permanentLockedAccount.OriginalVesting
		} else if _, ok := existingAccount.(*authtypes.BaseAccount); ok {
			// Handle base account
			accountInfo.accountType = BaseAccountType
		} else {
			return nil, fmt.Errorf("unexpected account type")
		}
	}

	return &accountInfo, nil
}

func resolveNewBaseAccount(ctx sdk.Context, app *App, genesisAccount *AccountInfo, existingAccount authtypes.AccountI) (*authtypes.BaseAccount, error) {
	var newBaseAccount *authtypes.BaseAccount
	var err error

	// Check for pubkey collision
	if existingAccount != nil {
		// Handle collision

		// Set pubkey from newAcc if is not in existingAccount
		if existingAccount.GetPubKey() == nil && genesisAccount.pubkey != nil {
			err := existingAccount.SetPubKey(genesisAccount.pubkey)
			if err != nil {
				return nil, err
			}
		}

		if genesisAccount.pubkey != nil && existingAccount.GetPubKey() != nil && !existingAccount.GetPubKey().Equals(genesisAccount.pubkey) {
			return nil, fmt.Errorf("account already exists with different pubkey: %s", genesisAccount.address)
		}

		newBaseAccount = authtypes.NewBaseAccount(genesisAccount.rawAddress, existingAccount.GetPubKey(), existingAccount.GetAccountNumber(), existingAccount.GetSequence())

	} else {

		// Handle regular migration
		newBaseAccount, err = getNewBaseAccount(ctx, app, genesisAccount)
		if err != nil {
			return nil, err
		}

	}

	return newBaseAccount, nil
}

func doRegularAccountMigration(ctx sdk.Context, app *App, genesisAccount *AccountInfo, existingAccount authtypes.AccountI, newBalance sdk.Coins, cudosCfg *CudosMergeConfig, manifest *UpgradeManifest) error {
	// Get base account and check for public keys collision
	newBaseAccount, err := resolveNewBaseAccount(ctx, app, genesisAccount, existingAccount)
	if err != nil {
		return err
	}

	// If there is anything to mint
	if newBalance != nil {

		// Account is not vesting
		if cudosCfg.notVestedAccounts.Has(genesisAccount.address) {
			err := createNewNormalAccountFromBaseAccount(ctx, app, newBaseAccount)
			if err != nil {
				return err
			}
		} else {
			// Account is vesting
			err := createNewVestingAccountFromBaseAccount(ctx, app, newBaseAccount, newBalance, ctx.BlockTime().Unix(), ctx.BlockTime().Unix()+cudosCfg.config.VestingPeriod)
			if err != nil {
				return err
			}
		}

		err = migrateToAccount(ctx, app, genesisAccount.address, genesisAccount.rawAddress, genesisAccount.balance, newBalance, "regular_account", manifest)
		if err != nil {
			return err
		}
		// There is nothing to mint
	} else {
		// Just create account if it's base account, but there is no balance for vesting
		err := createNewNormalAccountFromBaseAccount(ctx, app, newBaseAccount)
		if err != nil {
			return err
		}
	}

	return nil
}

func doCollisionMigration(ctx sdk.Context, app *App, genesisData *GenesisData, genesisAccount *AccountInfo, existingAccount authtypes.AccountI, newBalance sdk.Coins, cudosCfg *CudosMergeConfig, manifest *UpgradeManifest) error {
	// Keep existing account intact and move cudos balance to account specified in config
	genesisData.collisionMap.SetNew(genesisAccount.address, cudosCfg.config.VestingCollisionDestAddr)

	_, destRawAddr, err := bech32.DecodeAndConvert(cudosCfg.config.VestingCollisionDestAddr)
	if err != nil {
		return err
	}

	err = migrateToAccount(ctx, app, genesisAccount.address, destRawAddr, genesisAccount.balance, newBalance, "vesting_collision_account", manifest)
	if err != nil {
		return err
	}

	return nil
}

func MigrateGenesisAccounts(genesisData *GenesisData, ctx sdk.Context, app *App, cudosCfg *CudosMergeConfig, manifest *UpgradeManifest) error {
	mintModuleAddr := app.AccountKeeper.GetModuleAddress(minttypes.ModuleName)
	initialMintBalance := app.BankKeeper.GetAllBalances(ctx, mintModuleAddr)

	// Mint donor chain total supply
	totalSupplyToMint := sdk.NewCoins(sdk.NewCoin(cudosCfg.config.ConvertedDenom, cudosCfg.config.TotalFetchSupplyToMint))
	totalCudosSupply := sdk.NewCoins(sdk.NewCoin(cudosCfg.config.OriginalDenom, cudosCfg.config.TotalCudosSupply))

	err := app.MintKeeper.MintCoins(ctx, totalSupplyToMint)
	if err != nil {
		return err
	}

	totalSupplyReducedByCommission, err := convertBalance(totalCudosSupply, cudosCfg)
	if err != nil {
		return err
	}

	totalCommission := totalSupplyToMint.Sub(totalSupplyReducedByCommission)

	_, commissionRawAcc, err := bech32.DecodeAndConvert(cudosCfg.config.CommissionFetchAddr)
	if err != nil {
		return fmt.Errorf("failed to get commission account raw address: %w", err)
	}

	err = migrateToAccount(ctx, app, "mint_module", commissionRawAcc, sdk.NewCoins(), totalCommission, "total_commission", manifest)

	extraSupplyInCudos := cudosCfg.config.TotalCudosSupply.Sub(genesisData.totalSupply.AmountOf(cudosCfg.config.OriginalDenom))
	extraSupplyCudosAddress, err := convertAddressPrefix(cudosCfg.config.ExtraSupplyFetchAddr, cudosCfg.config.OldAddrPrefix)
	if err != nil {
		return err
	}

	extraSupplyInCudosCoins := sdk.NewCoins(sdk.NewCoin(cudosCfg.config.OriginalDenom, extraSupplyInCudos))

	err = createGenesisBalance(genesisData, extraSupplyCudosAddress, extraSupplyInCudosCoins, "extra_supply", manifest, cudosCfg)
	if err != nil {
		return err
	}

	// Mint the rest of the supply
	for _, genesisAccountAddress := range genesisData.accounts.Keys() {
		genesisAccount := genesisData.accounts.MustGet(genesisAccountAddress)

		if genesisAccount.accountType == ContractAccountType {
			// All contracts balance should be handled already
			if genesisAccount.balance.Empty() {
				err = markAccountAsMigrated(genesisData, genesisAccountAddress)
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("unresolved contract balance: %s %s", genesisAccountAddress, genesisAccount.balance.String())
			}
			continue
		}
		if genesisAccount.accountType == ModuleAccountType {
			if genesisAccount.balance.Empty() {
				err = markAccountAsMigrated(genesisData, genesisAccountAddress)
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("unresolved module balance: %s %s %s", genesisAccountAddress, genesisAccount.balance.String(), genesisAccount.name)
			}
			continue
		}

		if genesisAccount.accountType == IBCAccountType {
			// All IBC balances should be handled already
			if genesisAccount.balance.Empty() {
				err = markAccountAsMigrated(genesisData, genesisAccountAddress)
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("unresolved contract balance: %s %s", genesisAccountAddress, genesisAccount.balance.String())
			}
			continue
		}

		existingAccount := app.AccountKeeper.GetAccount(ctx, genesisAccount.rawAddress)
		existingAccountInfo, err := accountIToAccountInfo(existingAccount)
		if err != nil {
			return err
		}

		// Get balance to mint
		newBalance, err := convertBalance(genesisAccount.balance, cudosCfg)
		if err != nil {
			return err
		}

		// Handle all collision cases
		regularMigration := true
		if existingAccount != nil && existingAccountInfo.accountType != BaseAccountType {
			regularMigration = false
		}

		if genesisAccount.accountType != BaseAccountType {
			regularMigration = false
		}

		if regularMigration {
			err := doRegularAccountMigration(ctx, app, genesisAccount, existingAccount, newBalance, cudosCfg, manifest)
			if err != nil {
				return fmt.Errorf("failed to migrate account %s: %w", genesisAccountAddress, err)
			}
		} else {
			err := RegisterVestingCollision(manifest, genesisAccount, newBalance, existingAccount)
			if err != nil {
				return err
			}

			// New balance goes to foundation wallet
			err = doCollisionMigration(ctx, app, genesisData, genesisAccount, existingAccount, newBalance, cudosCfg, manifest)
			if err != nil {
				return fmt.Errorf("failed to migrate account %s: %w", genesisAccountAddress, err)
			}
		}

		err = markAccountAsMigrated(genesisData, genesisAccountAddress)
		if err != nil {
			return err
		}

	}

	// Move remaining mint module balance
	remainingMintBalance := app.BankKeeper.GetAllBalances(ctx, mintModuleAddr)
	remainingMintBalance = remainingMintBalance.Sub(initialMintBalance)

	err = checkTolerance(remainingMintBalance, maxToleratedRemainingMintBalance)
	if err != nil {
		return err
	}

	err = migrateToAccount(ctx, app, mintModuleAddr.String(), commissionRawAcc, sdk.NewCoins(), remainingMintBalance, "remaining_mint_module_balance", manifest)

	return nil
}

func DoGenesisAccountMovements(genesisData *GenesisData, cudosCfg *CudosMergeConfig, manifest *UpgradeManifest) error {
	if cudosCfg.MovedAccounts == nil {
		// Nothing to move
		return nil
	}

	for _, fromAddr := range cudosCfg.MovedAccounts.Keys() {
		toAddr := cudosCfg.MovedAccounts.MustGet(fromAddr)

		fromAcc, exists := genesisData.accounts.Get(fromAddr)

		if !exists {
			registerManifestBalanceMovement(fromAddr, toAddr, nil, "non_existing_from_account", manifest)
			return nil
		}

		if fromAcc.balance.IsZero() {
			registerManifestBalanceMovement(fromAddr, toAddr, nil, "nothing_to_move_err", manifest)
			return nil
		}

		err := moveGenesisBalance(genesisData, fromAddr, toAddr, fromAcc.balance, "balance_movement", manifest, cudosCfg)
		if err != nil {
			return err
		}

		err = moveGenesisDelegations(genesisData, fromAddr, toAddr, manifest)
		if err != nil {
			return err
		}
	}

	return nil
}

func parseGenesisTotalSupply(jsonData map[string]interface{}) (sdk.Coins, error) {
	bank := jsonData[banktypes.ModuleName].(map[string]interface{})
	supply := bank["supply"].([]interface{})
	totalSupply, err := getCoinsFromInterfaceSlice(supply)
	if err != nil {
		return nil, err
	}

	return totalSupply, nil

}

func verifySupply(genesisData *GenesisData, cudosCfg *CudosMergeConfig, manifest *UpgradeManifest) error {

	expectedMintedSupply := sdk.NewCoins(sdk.NewCoin(cudosCfg.config.ConvertedDenom, cudosCfg.config.TotalFetchSupplyToMint))

	mintedSupply := manifest.Migration.AggregatedMigratedAmount

	maximumDifference, ok := sdk.NewIntFromString("10000000000")
	if !ok {
		return fmt.Errorf("invalid maximum difference value")
	}

	for _, expectedCoin := range expectedMintedSupply {
		for _, mintedCoin := range mintedSupply {
			if expectedCoin.Denom == mintedCoin.Denom {
				var difference sdk.Int
				if expectedCoin.Amount.GT(mintedCoin.Amount) {
					difference = expectedCoin.Amount.Sub(mintedCoin.Amount)
				} else {
					difference = mintedCoin.Amount.Sub(expectedCoin.Amount)
				}

				if difference.GT(maximumDifference) {
					return fmt.Errorf("Total supply is not correct, expected %s, got %s", expectedCoin.String(), mintedCoin.String())
				}

			}
		}

	}

	return nil
}
