package ante

import (
	ibcante "github.com/cosmos/ibc-go/v3/modules/core/ante"
	ibckeeper "github.com/cosmos/ibc-go/v3/modules/core/keeper"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"

	ethante "github.com/evmos/ethermint/app/ante"
)

type HandlerOptions struct {
	authante.HandlerOptions
	IBCKeeper         *ibckeeper.Keeper
	TxCounterStoreKey sdk.StoreKey
	WasmConfig        wasmtypes.WasmConfig
}

// NewAnteHandler returns an ante handler responsible for attempting to route an
// Ethereum or SDK transaction to an internal ante handler for performing
// transaction-level processing (e.g. fee payment, signature verification) before
// being passed onto it's respective handler.
func NewAnteHandler(options HandlerOptions) sdk.AnteHandler {
	return func(
		ctx sdk.Context, tx sdk.Tx, sim bool,
	) (newCtx sdk.Context, err error) {
		var anteHandler sdk.AnteHandler

		defer ethante.Recover(ctx.Logger(), &err)

		txWithExtensions, ok := tx.(authante.HasExtensionOptionsTx)
		if ok {
			opts := txWithExtensions.GetExtensionOptions()
			if len(opts) > 0 {
				switch typeURL := opts[0].GetTypeUrl(); typeURL {
				case "/ethermint.evm.v1.ExtensionOptionsEthereumTx":
					// handle as *evmtypes.MsgEthereumTx
					anteHandler = newEthAnteHandler(options)
				case "/ethermint.types.v1.ExtensionOptionsWeb3Tx":
					// handle as normal Cosmos SDK tx, except signature is checked for EIP712 representation
					anteHandler = newCosmosAnteHandlerEip712(options)
				default:
					return ctx, sdkerrors.Wrapf(
						sdkerrors.ErrUnknownExtensionOptions,
						"rejecting tx with unsupported extension option: %s", typeURL,
					)
				}

				return anteHandler(ctx, tx, sim)
			}
		}

		// handle as totally normal Cosmos SDK tx
		switch tx.(type) {
		case sdk.Tx:
			anteHandler = newCosmosAnteHandler(options)
			if options.AccountKeeper == nil {
				return nil, errors.Wrap(errors.ErrLogic, "account keeper is required for ante handler")
			}
			if options.BankKeeper == nil {
				return nil, errors.Wrap(errors.ErrLogic, "bank keeper is required for ante handler")
			}
			if options.SignModeHandler == nil {
				return nil, errors.Wrap(errors.ErrLogic, "sign mode handler is required for ante handler")
			}

			var sigGasConsumer = options.SigGasConsumer
			if sigGasConsumer == nil {
				sigGasConsumer = authante.DefaultSigVerificationGasConsumer
			}

			anteDecorators := []sdk.AnteDecorator{
				authante.NewSetUpContextDecorator(),
				wasmkeeper.NewLimitSimulationGasDecorator(options.WasmConfig.SimulationGasLimit),
				wasmkeeper.NewCountTXDecorator(options.TxCounterStoreKey),
				authante.NewRejectExtensionOptionsDecorator(),
				authante.NewMempoolFeeDecorator(),
				authante.NewValidateBasicDecorator(),
				authante.NewTxTimeoutHeightDecorator(),
				authante.NewValidateMemoDecorator(options.AccountKeeper),
				authante.NewConsumeGasForTxSizeDecorator(options.AccountKeeper),
				authante.NewDeductFeeDecorator(options.AccountKeeper, options.BankKeeper, options.FeegrantKeeper),
				authante.NewSetPubKeyDecorator(options.AccountKeeper),
				authante.NewValidateSigCountDecorator(options.AccountKeeper),
				authante.NewSigGasConsumeDecorator(options.AccountKeeper, sigGasConsumer),
				authante.NewSigVerificationDecorator(options.AccountKeeper, options.SignModeHandler),
				authante.NewIncrementSequenceDecorator(options.AccountKeeper),
				ibcante.NewAnteDecorator(options.IBCKeeper),
			}
			
			return sdk.ChainAnteDecorators(anteDecorators...), nil
		default:
			return ctx, sdkerrors.Wrapf(sdkerrors.ErrUnknownRequest, "invalid transaction type: %T", tx)
		}

		return anteHandler(ctx, tx, sim)
	}
}
