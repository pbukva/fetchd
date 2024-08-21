package app

var NetworkInfos = map[string]NetworkConfig{
	"fetchhub-4": {
		ibcTargetAddr:                    "cudos1qqz5ezf9ylgft0eq97d66v5aakynux540ds9mv", // Replace!!
		remainingStakingBalanceAddr:      "cudos1qqz5ezf9ylgft0eq97d66v5aakynux540ds9mv", // Replace!!
		remainingGravityBalanceAddr:      "cudos1qqz5ezf9ylgft0eq97d66v5aakynux540ds9mv", // Replace!!
		remainingDistributionBalanceAddr: "cudos1qqz5ezf9ylgft0eq97d66v5aakynux540ds9mv", // Replace!!

		newAddrPrefix: "fetch",
		oldAddrPrefix: "cudos",

		originalDenom:  "acudos",
		convertedDenom: "afet",
		stakingDenom:   "afet",

		mergeTime:     123456,                // Epoch time of merge
		vestingPeriod: 3 * 30 * 24 * 60 * 60, // 3 months period

		balanceConversionConstants: map[string]int{
			"acudos": 11},

		notVestedAccounts: map[string]bool{
			"cudos1qqz5ezf9ylgft0eq97d66v5aakynux540ds9mv": true,
		},

		backupValidators: []string{"fetchvaloper122j02czdt5ca8cf576wy2hassyxyx67wdsecml"},
		validatorsMap: map[string]string{
			"cudosvaloper1s5qa3dpghnre6dqfgfhudxqjhwsv0mx43xayku": "fetchvaloper122j02czdt5ca8cf576wy2hassyxyx67wdsecml",
			"cudosvaloper1ctcrpuyumt60733u0yd5htwzedgfae0n8gql5n": "fetchvaloper122j02czdt5ca8cf576wy2hassyxyx67wdsecml"},
	},

	"dorado-1": {
		ibcTargetAddr: "fetchvaloper14w6a4al72uc3fpfy4lqtg0a7xtkx3w7hda0vel", // Replace!!
	},
}

type NetworkConfig struct {
	ibcTargetAddr                    string
	remainingStakingBalanceAddr      string // Account for remaining bonded and not-bonded pool balances and balances from all other module accounts
	remainingGravityBalanceAddr      string // Account for remaining bonded and not-bonded pool balances and balances from all other module accounts
	remainingDistributionBalanceAddr string // Account for remaining bonded and not-bonded pool balances and balances from all other module accounts

	newAddrPrefix string
	oldAddrPrefix string

	originalDenom  string
	convertedDenom string
	stakingDenom   string

	mergeTime     int64 // Epoch time of merge - beginning of vesting period
	vestingPeriod int64 // Vesting period

	balanceConversionConstants map[string]int

	notVestedAccounts map[string]bool

	validatorsMap    map[string]string
	backupValidators []string
}
