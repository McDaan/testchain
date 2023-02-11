package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/McDaan/testchain/cmd/config"
	appparams "github.com/McDaan/testchain/cmd/config"
	minttypes "github.com/McDaan/testchain/x/mint/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	icatypes "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"github.com/spf13/cobra"
	tmtypes "github.com/tendermint/tendermint/types"
)

// PrepareGenesisCmd returns generate-genesis cobra Command.
func GenerateGenesisCmd(defaultNodeHome string, mbm module.BasicManager) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate-genesis [chain_id]",
		Short: "Generate a genesis file with initial setup",
		Long: `Generate a genesis file with initial setup.
Example:
	acred generate-genesis testchain_9100-1
	- Check input genesis:
		file is at ~/.testd/config/genesis.json
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			depCdc := clientCtx.Codec
			cdc := depCdc
			serverCtx := server.GetServerContextFromCmd(cmd)
			config := serverCtx.Config

			// read genesis file
			genFile := config.GenesisFile()
			appState, genDoc, err := genutiltypes.GenesisStateFromGenFile(genFile)
			if err != nil {
				return fmt.Errorf("failed to unmarshal genesis state: %w", err)
			}

			// get genesis params
			chainID := args[0]

			// run Prepare Genesis
			appState, genDoc, err = PrepareGenesis(clientCtx, appState, genDoc, chainID)
			if err != nil {
				return err
			}

			// validate genesis state
			if err = mbm.ValidateGenesis(cdc, clientCtx.TxConfig, appState); err != nil {
				return fmt.Errorf("error validating genesis file: %s", err.Error())
			}

			// save genesis
			appStateJSON, err := json.Marshal(appState)
			if err != nil {
				return fmt.Errorf("failed to marshal application genesis state: %w", err)
			}

			genDoc.AppState = appStateJSON
			err = genutil.ExportGenesisFile(genDoc, genFile)
			return err
		},
	}

	cmd.Flags().String(flags.FlagHome, defaultNodeHome, "The application home directory")
	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func PrepareGenesis(clientCtx client.Context, appState map[string]json.RawMessage, genDoc *tmtypes.GenesisDoc, chainID string) (map[string]json.RawMessage, *tmtypes.GenesisDoc, error) {
	depCdc := clientCtx.Codec
	cdc := depCdc

	// chain params genesis
	genDoc.ChainID = chainID
	genDoc.GenesisTime = time.Unix(1676066400, 0) // Friday, February 10, 2023 09:00:00 PM GMT+0000
	genDoc.ConsensusParams = tmtypes.DefaultConsensusParams()
	genDoc.ConsensusParams.Block.MaxBytes = 21 * 1024 * 1024
	genDoc.ConsensusParams.Block.MaxGas = 300_000_000

	// mint module genesis
	mintGenState := minttypes.DefaultGenesisState()
	mintGenState.Params = minttypes.DefaultParams()
	mintGenState.Params.MintDenom = appparams.BaseDenom
	mintGenState.Params.MintingRewardsDistributionStartTime = 1676131200 // Fri Feb 11 2023 15:00:00 GMT+0000
	mintGenState.Params.NextRewardsReductionTime = 1685631600            // Thu Jun 01 2023 15:00:00 GMT+0000

	mintGenStateBz, err := cdc.MarshalJSON(mintGenState)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal mint genesis state: %w", err)
	}
	appState[minttypes.ModuleName] = mintGenStateBz

	// evm module genesis
	evmGenState := evmtypes.DefaultGenesisState()
	evmGenState.Params = evmtypes.DefaultParams()
	evmGenState.Params.EvmDenom = appparams.BaseDenom

	evmGenStateBz, err := cdc.MarshalJSON(evmGenState)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal evm genesis state: %w", err)
	}
	appState[evmtypes.ModuleName] = evmGenStateBz

	// bank module genesis
	bankGenState := banktypes.DefaultGenesisState()
	bankGenState.Params = banktypes.DefaultParams()

	bankGenState.Supply = sdk.NewCoins(sdk.NewCoin(appparams.BaseDenom, sdk.NewInt(350_000_000).Mul(config.DecimalReduction))) // 350M TEST

	genAccounts := []authtypes.GenesisAccount{}

	addrStrategicReserve, err := sdk.AccAddressFromBech32("test12geeqsg3f8k4tluk00lsqlpvx7r7jus46d3clc")
	if err != nil {
		return nil, nil, err
	}
	genAccounts = append(genAccounts, authtypes.NewBaseAccount(addrStrategicReserve, nil, 0, 0))

	// send tokens to genesis validators
	genesisValidators := []string{
		"test1004t4atdp0tmueedld40tvll6a4ecvzgt9nf0t", // val-1
		"test1mal8jxeaqaku5p4vs4vjtsmjd9zurragcwev6u", // val-2
	}

	totalValidatorInitialCoins := sdk.NewCoins()
	validatorInitialCoins := sdk.NewCoins(sdk.NewCoin(appparams.BaseDenom, sdk.NewInt(120).Mul(config.DecimalReduction))) // 120 TEST
	for _, address := range genesisValidators {
		bankGenState.Balances = append(bankGenState.Balances, banktypes.Balance{
			Address: address,
			Coins:   validatorInitialCoins,
		})
		addr, err := sdk.AccAddressFromBech32(address)
		if err != nil {
			return nil, nil, err
		}
		totalValidatorInitialCoins = totalValidatorInitialCoins.Add(validatorInitialCoins...)
		genAccounts = append(genAccounts, authtypes.NewBaseAccount(addr, nil, 0, 0))
	}

	// strategic reserve = supply - validator initial coins
	bankGenState.Balances = append(bankGenState.Balances, banktypes.Balance{
		Address: addrStrategicReserve.String(),
		Coins:   bankGenState.Supply.Sub(totalValidatorInitialCoins),
	})

	bankGenStateBz, err := cdc.MarshalJSON(bankGenState)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal bank genesis state: %w", err)
	}
	appState[banktypes.ModuleName] = bankGenStateBz

	// account module genesis
	authGenState := authtypes.GetGenesisStateFromAppState(depCdc, appState)
	authGenState.Params = authtypes.DefaultParams()

	accounts, err := authtypes.PackAccounts(genAccounts)
	if err != nil {
		panic(err)
	}

	authGenState.Accounts = append(authGenState.Accounts, accounts...)
	authGenStateBz, err := cdc.MarshalJSON(&authGenState)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal staking genesis state: %w", err)
	}
	appState[authtypes.ModuleName] = authGenStateBz

	// staking module genesis
	stakingGenState := stakingtypes.GetGenesisStateFromAppState(depCdc, appState)
	stakingGenState.Params = stakingtypes.DefaultParams()
	stakingGenState.Params.BondDenom = appparams.BaseDenom
	stakingGenStateBz, err := cdc.MarshalJSON(stakingGenState)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal staking genesis state: %w", err)
	}
	appState[stakingtypes.ModuleName] = stakingGenStateBz

	// distribution module genesis
	distributionGenState := distributiontypes.DefaultGenesisState()
	distributionGenState.Params = distributiontypes.DefaultParams()
	distributionGenState.Params.BaseProposerReward = sdk.ZeroDec()
	distributionGenState.Params.BonusProposerReward = sdk.ZeroDec()
	distributionGenState.Params.CommunityTax = sdk.ZeroDec()
	distributionGenState.FeePool.CommunityPool = sdk.NewDecCoinsFromCoins(sdk.NewCoins()...)
	distributionGenStateBz, err := cdc.MarshalJSON(distributionGenState)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal distribution genesis state: %w", err)
	}
	appState[distributiontypes.ModuleName] = distributionGenStateBz

	// gov module genesis
	govGenState := govtypes.DefaultGenesisState()
	defaultGovParams := govtypes.DefaultParams()
	govGenState.DepositParams = defaultGovParams.DepositParams
	govGenState.DepositParams.MinDeposit = sdk.Coins{sdk.NewCoin(appparams.BaseDenom, sdk.NewInt(500).Mul(config.DecimalReduction))} // 500 TEST
	govGenState.TallyParams = defaultGovParams.TallyParams
	govGenState.VotingParams = defaultGovParams.VotingParams
	govGenState.VotingParams.VotingPeriod = time.Hour * 24 * 2 // 2 days
	govGenStateBz, err := cdc.MarshalJSON(govGenState)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal gov genesis state: %w", err)
	}
	appState[govtypes.ModuleName] = govGenStateBz

	// slashing module genesis
	slashingGenState := slashingtypes.DefaultGenesisState()
	slashingGenState.Params = slashingtypes.DefaultParams()
	slashingGenState.Params.SignedBlocksWindow = 30000
	slashingGenState.Params.MinSignedPerWindow = sdk.NewDecWithPrec(5, 2)
	slashingGenState.Params.SlashFractionDowntime = sdk.NewDecWithPrec(1, 3)
	slashingGenStateBz, err := cdc.MarshalJSON(slashingGenState)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal slashing genesis state: %w", err)
	}
	appState[slashingtypes.ModuleName] = slashingGenStateBz

	// crisis module genesis
	crisisGenState := crisistypes.DefaultGenesisState()
	crisisGenState.ConstantFee = sdk.NewCoin(appparams.BaseDenom, sdk.NewInt(1).Mul(config.DecimalReduction)) // 1 TEST
	crisisGenStateBz, err := cdc.MarshalJSON(crisisGenState)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal crisis genesis state: %w", err)
	}
	appState[crisistypes.ModuleName] = crisisGenStateBz

	// ica module genesis
	icaGenState := icatypes.DefaultGenesis()
	icaGenState.HostGenesisState.Params.AllowMessages = []string{
		"/cosmos.bank.v1beta1.MsgSend",
		"/cosmos.bank.v1beta1.MsgMultiSend",
		"/cosmos.distribution.v1beta1.MsgSetWithdrawAddress",
		"/cosmos.distribution.v1beta1.MsgWithdrawValidatorCommission",
		"/cosmos.distribution.v1beta1.MsgFundCommunityPool",
		"/cosmos.distribution.v1beta1.MsgWithdrawDelegatorReward",
		"/cosmos.feegrant.v1beta1.MsgGrantAllowance",
		"/cosmos.feegrant.v1beta1.MsgRevokeAllowance",
		"/cosmos.gov.v1beta1.MsgVoteWeighted",
		"/cosmos.gov.v1beta1.MsgSubmitProposal",
		"/cosmos.gov.v1beta1.MsgDeposit",
		"/cosmos.gov.v1beta1.MsgVote",
		"/cosmos.staking.v1beta1.MsgEditValidator",
		"/cosmos.staking.v1beta1.MsgDelegate",
		"/cosmos.staking.v1beta1.MsgUndelegate",
		"/cosmos.staking.v1beta1.MsgBeginRedelegate",
		"/cosmos.staking.v1beta1.MsgCreateValidator",
		"/ibc.applications.transfer.v1.MsgTransfer",
	}
	icaGenStateBz, err := cdc.MarshalJSON(icaGenState)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal crisis genesis state: %w", err)
	}
	appState[icatypes.ModuleName] = icaGenStateBz

	// return appState and genDoc
	return appState, genDoc, nil
}
