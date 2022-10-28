package app

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/simapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authrest "github.com/cosmos/cosmos-sdk/x/auth/client/rest"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authsims "github.com/cosmos/cosmos-sdk/x/auth/simulation"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	authzmodule "github.com/cosmos/cosmos-sdk/x/authz/module"
	"github.com/cosmos/cosmos-sdk/x/bank"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/capability"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"

	"github.com/cosmos/cosmos-sdk/x/evidence"
	evidencekeeper "github.com/cosmos/cosmos-sdk/x/evidence/keeper"
	evidencetypes "github.com/cosmos/cosmos-sdk/x/evidence/types"
	"github.com/cosmos/cosmos-sdk/x/feegrant"
	feegrantkeeper "github.com/cosmos/cosmos-sdk/x/feegrant/keeper"
	feegrantmodule "github.com/cosmos/cosmos-sdk/x/feegrant/module"

	"github.com/cosmos/cosmos-sdk/x/params"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"

	"github.com/cosmos/cosmos-sdk/x/upgrade"
	upgradekeeper "github.com/cosmos/cosmos-sdk/x/upgrade/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	ica "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts"
	icacontroller "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/controller"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/controller/keeper"
	icacontrollertypes "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/controller/types"
	icahost "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/host"
	icahostkeeper "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/host/keeper"
	icahosttypes "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/types"
	transfer "github.com/cosmos/ibc-go/v3/modules/apps/transfer"
	ibctransferkeeper "github.com/cosmos/ibc-go/v3/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/v3/modules/core"
	porttypes "github.com/cosmos/ibc-go/v3/modules/core/05-port/types"
	ibchost "github.com/cosmos/ibc-go/v3/modules/core/24-host"
	ibckeeper "github.com/cosmos/ibc-go/v3/modules/core/keeper"

	// Note: please do your research before using this in production app, this is a demo and not an officially
	// supported IBC team implementation. It has no known issues, but do your own research before using it.
	ibctesting "github.com/cosmos/ibc-go/v3/testing"
	intertx "github.com/cosmos/interchain-accounts/x/inter-tx"
	intertxkeeper "github.com/cosmos/interchain-accounts/x/inter-tx/keeper"
	intertxtypes "github.com/cosmos/interchain-accounts/x/inter-tx/types"
	"github.com/gorilla/mux"
	"github.com/rakyll/statik/fs"
	"github.com/spf13/cast"

	"github.com/tendermint/spm/cosmoscmd"
	abci "github.com/tendermint/tendermint/abci/types"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	dbm "github.com/tendermint/tm-db"

	wasmappparams "github.com/CosmWasm/wasmd/app/params"
	"github.com/CosmWasm/wasmd/x/wasm"

	ibcclienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	ibcconsumer "github.com/cosmos/interchain-security/x/ccv/consumer"
	ibcconsumerkeeper "github.com/cosmos/interchain-security/x/ccv/consumer/keeper"
	ibcconsumertypes "github.com/cosmos/interchain-security/x/ccv/consumer/types"

	// unnamed import of statik for swagger UI support
	_ "github.com/cosmos/cosmos-sdk/client/docs/statik"
)

const appName = "WasmApp"

// We pull these out so we can set them with LDFLAGS in the Makefile
var (
	NodeDir      = ".wasmd"
	Bech32Prefix = "wasm"

	// If EnabledSpecificProposals is "", and this is "true", then enable all x/wasm proposals.
	// If EnabledSpecificProposals is "", and this is not "true", then disable all x/wasm proposals.
	ProposalsEnabled = "false"
	// If set to non-empty string it must be comma-separated list of values that are all a subset
	// of "EnableAllProposals" (takes precedence over ProposalsEnabled)
	// https://github.com/CosmWasm/wasmd/blob/02a54d33ff2c064f3539ae12d75d027d9c665f05/x/wasm/internal/types/proposal.go#L28-L34
	EnableSpecificProposals = ""
)

// TODO[Ethernal]: democracy-enabled chain should support governance, so modify this method accordingly
// GetEnabledProposals parses the ProposalsEnabled / EnableSpecificProposals values to
// produce a list of enabled proposals to pass into wasmd app.
func GetEnabledProposals() []wasm.ProposalType {
	// if EnableSpecificProposals == "" {
	// 	if ProposalsEnabled == "true" {
	// 		return wasm.EnableAllProposals
	// 	}
	// 	return wasm.DisableAllProposals
	// }
	// chunks := strings.Split(EnableSpecificProposals, ",")
	// proposals, err := wasm.ConvertToProposals(chunks)
	// if err != nil {
	// 	panic(err)
	// }
	// return proposals

	return wasm.DisableAllProposals
}

// These constants are derived from the above variables.
// These are the ones we will want to use in the code, based on
// any overrides above
var (
	// DefaultNodeHome default home directories for wasmd
	DefaultNodeHome = os.ExpandEnv("$HOME/") + NodeDir

	// Bech32PrefixAccAddr defines the Bech32 prefix of an account's address
	Bech32PrefixAccAddr = Bech32Prefix
	// Bech32PrefixAccPub defines the Bech32 prefix of an account's public key
	Bech32PrefixAccPub = Bech32Prefix + sdk.PrefixPublic
	// Bech32PrefixValAddr defines the Bech32 prefix of a validator's operator address
	Bech32PrefixValAddr = Bech32Prefix + sdk.PrefixValidator + sdk.PrefixOperator
	// Bech32PrefixValPub defines the Bech32 prefix of a validator's operator public key
	Bech32PrefixValPub = Bech32Prefix + sdk.PrefixValidator + sdk.PrefixOperator + sdk.PrefixPublic
	// Bech32PrefixConsAddr defines the Bech32 prefix of a consensus node address
	Bech32PrefixConsAddr = Bech32Prefix + sdk.PrefixValidator + sdk.PrefixConsensus
	// Bech32PrefixConsPub defines the Bech32 prefix of a consensus node public key
	Bech32PrefixConsPub = Bech32Prefix + sdk.PrefixValidator + sdk.PrefixConsensus + sdk.PrefixPublic
)

var (
	// ModuleBasics defines the module BasicManager is in charge of setting up basic,
	// non-dependant module elements, such as codec registration
	// and genesis verification.
	ModuleBasics = module.NewBasicManager(
		auth.AppModuleBasic{},
		bank.AppModuleBasic{},
		capability.AppModuleBasic{},
		params.AppModuleBasic{},
		crisis.AppModuleBasic{},
		slashing.AppModuleBasic{},
		feegrantmodule.AppModuleBasic{},
		authzmodule.AppModuleBasic{},
		ibc.AppModuleBasic{},
		upgrade.AppModuleBasic{},
		evidence.AppModuleBasic{},
		transfer.AppModuleBasic{},
		vesting.AppModuleBasic{},
		wasm.AppModuleBasic{},
		ica.AppModuleBasic{},
		intertx.AppModuleBasic{},
		ibcconsumer.AppModuleBasic{},
	)

	// module account permissions
	maccPerms = map[string][]string{
		authtypes.FeeCollectorName:                    nil,
		ibcconsumertypes.ConsumerRedistributeName:     nil,
		ibcconsumertypes.ConsumerToSendToProviderName: nil,
		ibctransfertypes.ModuleName:                   {authtypes.Minter, authtypes.Burner},
		icatypes.ModuleName:                           nil,
		wasm.ModuleName:                               {authtypes.Burner},
	}
)

var (
	_ simapp.App              = (*WasmApp)(nil)
	_ servertypes.Application = (*WasmApp)(nil)
	_ ibctesting.TestingApp   = (*WasmApp)(nil)
)

// WasmApp extended ABCI application
type WasmApp struct {
	*baseapp.BaseApp
	legacyAmino       *codec.LegacyAmino //nolint:staticcheck
	appCodec          codec.Codec
	interfaceRegistry types.InterfaceRegistry

	invCheckPeriod uint

	// keys to access the substores
	keys    map[string]*sdk.KVStoreKey
	tkeys   map[string]*sdk.TransientStoreKey
	memKeys map[string]*sdk.MemoryStoreKey

	// keepers
	accountKeeper       authkeeper.AccountKeeper
	bankKeeper          bankkeeper.Keeper
	capabilityKeeper    *capabilitykeeper.Keeper
	slashingKeeper      slashingkeeper.Keeper
	crisisKeeper        crisiskeeper.Keeper
	upgradeKeeper       upgradekeeper.Keeper
	paramsKeeper        paramskeeper.Keeper
	evidenceKeeper      evidencekeeper.Keeper
	ibcKeeper           *ibckeeper.Keeper // IBC Keeper must be a pointer in the app, so we can SetRouter on it correctly
	icaControllerKeeper icacontrollerkeeper.Keeper
	icaHostKeeper       icahostkeeper.Keeper
	interTxKeeper       intertxkeeper.Keeper
	transferKeeper      ibctransferkeeper.Keeper
	feeGrantKeeper      feegrantkeeper.Keeper
	authzKeeper         authzkeeper.Keeper
	wasmKeeper          wasm.Keeper
	ConsumerKeeper      ibcconsumerkeeper.Keeper

	scopedIBCKeeper           capabilitykeeper.ScopedKeeper
	scopedICAHostKeeper       capabilitykeeper.ScopedKeeper
	scopedICAControllerKeeper capabilitykeeper.ScopedKeeper
	scopedInterTxKeeper       capabilitykeeper.ScopedKeeper
	scopedTransferKeeper      capabilitykeeper.ScopedKeeper
	scopedWasmKeeper          capabilitykeeper.ScopedKeeper
	scopedIBCConsumerKeeper   capabilitykeeper.ScopedKeeper

	// the module manager
	mm *module.Manager

	// simulation manager
	sm *module.SimulationManager

	// module configurator
	configurator module.Configurator
}

// NewWasmApp returns a reference to an initialized WasmApp.
func NewWasmApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	skipUpgradeHeights map[int64]bool,
	homePath string,
	invCheckPeriod uint,
	encodingConfig wasmappparams.EncodingConfig,
	enabledProposals []wasm.ProposalType,
	appOpts servertypes.AppOptions,
	wasmOpts []wasm.Option,
	baseAppOptions ...func(*baseapp.BaseApp),
) *WasmApp {
	appCodec, legacyAmino := encodingConfig.Marshaler, encodingConfig.Amino
	interfaceRegistry := encodingConfig.InterfaceRegistry

	bApp := baseapp.NewBaseApp(appName, logger, db, encodingConfig.TxConfig.TxDecoder(), baseAppOptions...)
	bApp.SetCommitMultiStoreTracer(traceStore)
	bApp.SetInterfaceRegistry(interfaceRegistry)

	keys := sdk.NewKVStoreKeys(
		authtypes.StoreKey, banktypes.StoreKey, slashingtypes.StoreKey,
		paramstypes.StoreKey, ibchost.StoreKey, upgradetypes.StoreKey,
		evidencetypes.StoreKey, ibctransfertypes.StoreKey, capabilitytypes.StoreKey,
		feegrant.StoreKey, authzkeeper.StoreKey, wasm.StoreKey, icahosttypes.StoreKey,
		icacontrollertypes.StoreKey, intertxtypes.StoreKey, ibcconsumertypes.StoreKey,
	)

	tkeys := sdk.NewTransientStoreKeys(paramstypes.TStoreKey)
	memKeys := sdk.NewMemoryStoreKeys(capabilitytypes.MemStoreKey)

	app := &WasmApp{
		BaseApp:           bApp,
		legacyAmino:       legacyAmino,
		appCodec:          appCodec,
		interfaceRegistry: interfaceRegistry,
		invCheckPeriod:    invCheckPeriod,
		keys:              keys,
		tkeys:             tkeys,
		memKeys:           memKeys,
	}

	app.paramsKeeper = initParamsKeeper(
		appCodec,
		legacyAmino,
		keys[paramstypes.StoreKey],
		tkeys[paramstypes.TStoreKey],
	)

	// set the BaseApp's parameter store
	bApp.SetParamStore(app.paramsKeeper.Subspace(baseapp.Paramspace).WithKeyTable(paramskeeper.ConsensusParamsKeyTable()))

	// add capability keeper and ScopeToModule for ibc module
	app.capabilityKeeper = capabilitykeeper.NewKeeper(
		appCodec,
		keys[capabilitytypes.StoreKey],
		memKeys[capabilitytypes.MemStoreKey],
	)
	scopedIBCKeeper := app.capabilityKeeper.ScopeToModule(ibchost.ModuleName)
	scopedICAHostKeeper := app.capabilityKeeper.ScopeToModule(icahosttypes.SubModuleName)
	scopedICAControllerKeeper := app.capabilityKeeper.ScopeToModule(icacontrollertypes.SubModuleName)
	scopedInterTxKeeper := app.capabilityKeeper.ScopeToModule(intertxtypes.ModuleName)
	scopedTransferKeeper := app.capabilityKeeper.ScopeToModule(ibctransfertypes.ModuleName)
	scopedWasmKeeper := app.capabilityKeeper.ScopeToModule(wasm.ModuleName)
	scopedIBCConsumerKeeper := app.capabilityKeeper.ScopeToModule(ibcconsumertypes.ModuleName)
	app.capabilityKeeper.Seal()

	// add keepers
	app.accountKeeper = authkeeper.NewAccountKeeper(
		appCodec,
		keys[authtypes.StoreKey],
		app.getSubspace(authtypes.ModuleName),
		authtypes.ProtoBaseAccount,
		maccPerms,
	)

	// Remove the fee-pool from the group of blocked recipient addresses in bank
	// this is required for the provider chain to be able to receive tokens from
	// the consumer chain
	bankBlockedAddrs := app.ModuleAccountAddrs()
	delete(bankBlockedAddrs, authtypes.NewModuleAddress(
		authtypes.FeeCollectorName).String())

	app.bankKeeper = bankkeeper.NewBaseKeeper(
		appCodec,
		keys[banktypes.StoreKey],
		app.accountKeeper,
		app.getSubspace(banktypes.ModuleName),
		bankBlockedAddrs,
	)
	app.authzKeeper = authzkeeper.NewKeeper(
		keys[authzkeeper.StoreKey],
		appCodec,
		app.BaseApp.MsgServiceRouter(),
	)
	app.feeGrantKeeper = feegrantkeeper.NewKeeper(
		appCodec,
		keys[feegrant.StoreKey],
		app.accountKeeper,
	)

	app.slashingKeeper = slashingkeeper.NewKeeper(
		appCodec,
		keys[slashingtypes.StoreKey],
		&app.ConsumerKeeper,
		app.getSubspace(slashingtypes.ModuleName),
	)
	app.crisisKeeper = crisiskeeper.NewKeeper(
		app.getSubspace(crisistypes.ModuleName),
		invCheckPeriod,
		app.bankKeeper,
		authtypes.FeeCollectorName,
	)
	app.upgradeKeeper = upgradekeeper.NewKeeper(
		skipUpgradeHeights,
		keys[upgradetypes.StoreKey],
		appCodec,
		homePath,
		app.BaseApp,
	)

	app.ibcKeeper = ibckeeper.NewKeeper(
		appCodec,
		keys[ibchost.StoreKey],
		app.getSubspace(ibchost.ModuleName),
		&app.ConsumerKeeper,
		app.upgradeKeeper,
		scopedIBCKeeper,
	)

	// Create CCV consumer keeper and module
	app.ConsumerKeeper = ibcconsumerkeeper.NewKeeper(
		appCodec,
		keys[ibcconsumertypes.StoreKey],
		app.getSubspace(ibcconsumertypes.ModuleName),
		scopedIBCConsumerKeeper,
		app.ibcKeeper.ChannelKeeper,
		&app.ibcKeeper.PortKeeper,
		app.ibcKeeper.ConnectionKeeper,
		app.ibcKeeper.ClientKeeper,
		app.slashingKeeper,
		app.bankKeeper,
		app.accountKeeper,
		&app.transferKeeper,
		app.ibcKeeper,
		authtypes.FeeCollectorName,
	)

	// consumer keeper satisfies the staking keeper interface of the slashing module
	app.slashingKeeper = slashingkeeper.NewKeeper(
		appCodec,
		keys[slashingtypes.StoreKey],
		&app.ConsumerKeeper,
		app.getSubspace(slashingtypes.ModuleName),
	)

	// register slashing module StakingHooks to the consumer keeper
	app.ConsumerKeeper = *app.ConsumerKeeper.SetHooks(app.slashingKeeper.Hooks())
	consumerModule := ibcconsumer.NewAppModule(app.ConsumerKeeper)

	// Create Transfer Keepers
	app.transferKeeper = ibctransferkeeper.NewKeeper(
		appCodec,
		keys[ibctransfertypes.StoreKey],
		app.getSubspace(ibctransfertypes.ModuleName),
		app.ibcKeeper.ChannelKeeper,
		app.ibcKeeper.ChannelKeeper,
		&app.ibcKeeper.PortKeeper,
		app.accountKeeper,
		app.bankKeeper,
		scopedTransferKeeper,
	)
	transferModule := transfer.NewAppModule(app.transferKeeper)
	transferIBCModule := transfer.NewIBCModule(app.transferKeeper)

	app.icaHostKeeper = icahostkeeper.NewKeeper(
		appCodec,
		keys[icahosttypes.StoreKey],
		app.getSubspace(icahosttypes.SubModuleName),
		app.ibcKeeper.ChannelKeeper,
		&app.ibcKeeper.PortKeeper,
		app.accountKeeper,
		scopedICAHostKeeper,
		app.MsgServiceRouter(),
	)
	app.icaControllerKeeper = icacontrollerkeeper.NewKeeper(
		appCodec,
		keys[icacontrollertypes.StoreKey],
		app.getSubspace(icacontrollertypes.SubModuleName),
		app.ibcKeeper.ChannelKeeper, // may be replaced with middleware such as ics29 fee
		app.ibcKeeper.ChannelKeeper,
		&app.ibcKeeper.PortKeeper,
		scopedICAControllerKeeper,
		app.MsgServiceRouter(),
	)
	icaModule := ica.NewAppModule(&app.icaControllerKeeper, &app.icaHostKeeper)
	icaHostIBCModule := icahost.NewIBCModule(app.icaHostKeeper)

	// For wasmd we use the demo controller from https://github.com/cosmos/interchain-accounts but see notes below
	app.interTxKeeper = intertxkeeper.NewKeeper(appCodec, keys[intertxtypes.StoreKey], app.icaControllerKeeper, scopedInterTxKeeper)
	// Note: please do your research before using this in production app, this is a demo and not an officially
	// supported IBC team implementation. Do your own research before using it.
	interTxModule := intertx.NewAppModule(appCodec, app.interTxKeeper)
	interTxIBCModule := intertx.NewIBCModule(app.interTxKeeper)
	// You will likely want to swap out the second argument with your own reviewed and maintained ica auth module
	icaControllerIBCModule := icacontroller.NewIBCModule(app.icaControllerKeeper, interTxIBCModule)

	// create evidence keeper with router
	evidenceKeeper := evidencekeeper.NewKeeper(
		appCodec,
		keys[evidencetypes.StoreKey],
		&app.ConsumerKeeper,
		app.slashingKeeper,
	)
	app.evidenceKeeper = *evidenceKeeper

	wasmDir := filepath.Join(homePath, "wasm")
	wasmConfig, err := wasm.ReadWasmConfig(appOpts)
	if err != nil {
		panic(fmt.Sprintf("error while reading wasm config: %s", err))
	}

	// The last arguments can contain custom message handlers, and custom query handlers,
	// if we want to allow any custom callbacks
	// TODO[Ethernal]: democracy-enabled chain should include "staking" feature as well
	supportedFeatures := "iterator,stargate"
	app.wasmKeeper = wasm.NewKeeper(
		appCodec,
		keys[wasm.StoreKey],
		app.getSubspace(wasm.ModuleName),
		app.accountKeeper,
		app.bankKeeper,
		nil,
		nil,
		app.ibcKeeper.ChannelKeeper,
		&app.ibcKeeper.PortKeeper,
		scopedWasmKeeper,
		app.transferKeeper,
		app.MsgServiceRouter(),
		app.GRPCQueryRouter(),
		wasmDir,
		wasmConfig,
		supportedFeatures,
		wasmOpts...,
	)

	// Create static IBC router, add app routes, then set and seal it
	ibcRouter := porttypes.NewRouter()
	ibcRouter.
		AddRoute(wasm.ModuleName, wasm.NewIBCHandler(app.wasmKeeper, app.ibcKeeper.ChannelKeeper)).
		AddRoute(ibctransfertypes.ModuleName, transferIBCModule).
		AddRoute(icacontrollertypes.SubModuleName, icaControllerIBCModule).
		AddRoute(icahosttypes.SubModuleName, icaHostIBCModule).
		AddRoute(intertxtypes.ModuleName, icaControllerIBCModule).
		AddRoute(ibcconsumertypes.ModuleName, consumerModule)
	app.ibcKeeper.SetRouter(ibcRouter)

	/****  Module Options ****/

	// NOTE: we may consider parsing `appOpts` inside module constructors. For the moment
	// we prefer to be more strict in what arguments the modules expect.
	skipGenesisInvariants := cast.ToBool(appOpts.Get(crisis.FlagSkipGenesisInvariants))

	// NOTE: Any module instantiated in the module manager that is later modified
	// must be passed by reference here.
	app.mm = module.NewManager(
		auth.NewAppModule(appCodec, app.accountKeeper, nil),
		vesting.NewAppModule(app.accountKeeper, app.bankKeeper),
		bank.NewAppModule(appCodec, app.bankKeeper, app.accountKeeper),
		capability.NewAppModule(appCodec, *app.capabilityKeeper),
		slashing.NewAppModule(appCodec, app.slashingKeeper, app.accountKeeper, app.bankKeeper, app.ConsumerKeeper),
		upgrade.NewAppModule(app.upgradeKeeper),
		wasm.NewAppModule(appCodec, &app.wasmKeeper, app.accountKeeper, app.bankKeeper),
		evidence.NewAppModule(app.evidenceKeeper),
		feegrantmodule.NewAppModule(appCodec, app.accountKeeper, app.bankKeeper, app.feeGrantKeeper, app.interfaceRegistry),
		authzmodule.NewAppModule(appCodec, app.authzKeeper, app.accountKeeper, app.bankKeeper, app.interfaceRegistry),
		ibc.NewAppModule(app.ibcKeeper),
		params.NewAppModule(app.paramsKeeper),
		transferModule,
		icaModule,
		interTxModule,
		consumerModule,
		crisis.NewAppModule(&app.crisisKeeper, skipGenesisInvariants), // always be last to make sure that it checks for all invariants and not only part of them
	)

	// During begin block slashing happens after distr.BeginBlocker so that
	// there is nothing left over in the validator fee pool, so as to keep the
	// CanWithdrawInvariant invariant.
	// NOTE: staking module is required if HistoricalEntries param > 0
	app.mm.SetOrderBeginBlockers(
		upgradetypes.ModuleName,
		capabilitytypes.ModuleName,
		slashingtypes.ModuleName,
		evidencetypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		crisistypes.ModuleName,
		authz.ModuleName,
		feegrant.ModuleName,
		paramstypes.ModuleName,
		vestingtypes.ModuleName,
		ibctransfertypes.ModuleName,
		ibchost.ModuleName,
		icatypes.ModuleName,
		intertxtypes.ModuleName,
		ibcconsumertypes.ModuleName,
		wasm.ModuleName,
	)

	app.mm.SetOrderEndBlockers(
		crisistypes.ModuleName,
		capabilitytypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		slashingtypes.ModuleName,
		evidencetypes.ModuleName,
		authz.ModuleName,
		feegrant.ModuleName,
		paramstypes.ModuleName,
		upgradetypes.ModuleName,
		vestingtypes.ModuleName,
		ibctransfertypes.ModuleName,
		ibchost.ModuleName,
		icatypes.ModuleName,
		intertxtypes.ModuleName,
		ibcconsumertypes.ModuleName,
		wasm.ModuleName,
	)

	// NOTE: The genutils module must occur after staking so that pools are
	// properly initialized with tokens from genesis accounts.
	// NOTE: Capability module must occur first so that it can initialize any capabilities
	// so that other modules that want to create or claim capabilities afterwards in InitChain
	// can do so safely.
	// NOTE: wasm module should be at the end as it can call other module functionality direct or via message dispatching during
	// genesis phase. For example bank transfer, auth account check, staking, ...
	app.mm.SetOrderInitGenesis(
		capabilitytypes.ModuleName,
		banktypes.ModuleName,
		slashingtypes.ModuleName,
		crisistypes.ModuleName,
		evidencetypes.ModuleName,
		ibctransfertypes.ModuleName,
		authz.ModuleName,
		authtypes.ModuleName,
		feegrant.ModuleName,
		paramstypes.ModuleName,
		upgradetypes.ModuleName,
		vestingtypes.ModuleName,
		ibchost.ModuleName,
		icatypes.ModuleName,
		intertxtypes.ModuleName,
		ibcconsumertypes.ModuleName,
		// wasm after ibc transfer
		wasm.ModuleName,
	)

	// Uncomment if you want to set a custom migration order here.
	// app.mm.SetOrderMigrations(custom order)

	app.mm.RegisterInvariants(&app.crisisKeeper)
	app.mm.RegisterRoutes(app.Router(), app.QueryRouter(), encodingConfig.Amino)

	app.configurator = module.NewConfigurator(app.appCodec, app.MsgServiceRouter(), app.GRPCQueryRouter())
	app.mm.RegisterServices(app.configurator)

	// create the simulation manager and define the order of the modules for deterministic simulations
	//
	// NOTE: this is not required apps that don't use the simulator for fuzz testing
	// transactions
	app.sm = module.NewSimulationManager(
		auth.NewAppModule(appCodec, app.accountKeeper, authsims.RandomGenesisAccounts),
		bank.NewAppModule(appCodec, app.bankKeeper, app.accountKeeper),
		capability.NewAppModule(appCodec, *app.capabilityKeeper),
		feegrantmodule.NewAppModule(appCodec, app.accountKeeper, app.bankKeeper, app.feeGrantKeeper, app.interfaceRegistry),
		authzmodule.NewAppModule(appCodec, app.authzKeeper, app.accountKeeper, app.bankKeeper, app.interfaceRegistry),
		slashing.NewAppModule(appCodec, app.slashingKeeper, app.accountKeeper, app.bankKeeper, app.ConsumerKeeper),
		params.NewAppModule(app.paramsKeeper),
		evidence.NewAppModule(app.evidenceKeeper),
		wasm.NewAppModule(appCodec, &app.wasmKeeper, app.accountKeeper, app.bankKeeper),
		ibc.NewAppModule(app.ibcKeeper),
		transferModule,
	)

	app.sm.RegisterStoreDecoders()
	// initialize stores
	app.MountKVStores(keys)
	app.MountTransientStores(tkeys)
	app.MountMemoryStores(memKeys)

	anteHandler, err := NewAnteHandler(
		HandlerOptions{
			HandlerOptions: ante.HandlerOptions{
				AccountKeeper:   app.accountKeeper,
				BankKeeper:      app.bankKeeper,
				FeegrantKeeper:  app.feeGrantKeeper,
				SignModeHandler: encodingConfig.TxConfig.SignModeHandler(),
				SigGasConsumer:  ante.DefaultSigVerificationGasConsumer,
			},
			IBCKeeper:         app.ibcKeeper,
			WasmConfig:        &wasmConfig,
			TXCounterStoreKey: keys[wasm.StoreKey],
			ConsumerKeeper:    app.ConsumerKeeper,
		},
	)
	if err != nil {
		panic(fmt.Errorf("failed to create AnteHandler: %s", err))
	}

	app.SetAnteHandler(anteHandler)
	app.SetInitChainer(app.InitChainer)
	app.SetBeginBlocker(app.BeginBlocker)
	app.SetEndBlocker(app.EndBlocker)

	// TODO[Ethernal]: SnapshotManager() and CommitMultiStore() do not exist in cosmos-sdk v0.45.2-ics
	// // must be before Loading version
	// // requires the snapshot store to be created and registered as a BaseAppOption
	// // see cmd/wasmd/root.go: 206 - 214 approx
	// if manager := app.SnapshotManager(); manager != nil {
	// 	err := manager.RegisterExtensions(
	// 		wasmkeeper.NewWasmSnapshotter(app.CommitMultiStore(), &app.wasmKeeper),
	// 	)
	// 	if err != nil {
	// 		panic(fmt.Errorf("failed to register snapshot extension: %s", err))
	// 	}
	// }

	app.scopedIBCKeeper = scopedIBCKeeper
	app.scopedTransferKeeper = scopedTransferKeeper
	app.scopedWasmKeeper = scopedWasmKeeper
	app.scopedICAHostKeeper = scopedICAHostKeeper
	app.scopedICAControllerKeeper = scopedICAControllerKeeper
	app.scopedInterTxKeeper = scopedInterTxKeeper
	app.scopedIBCConsumerKeeper = scopedIBCConsumerKeeper

	if loadLatest {
		if err := app.LoadLatestVersion(); err != nil {
			tmos.Exit(fmt.Sprintf("failed to load latest version: %s", err))
		}
		ctx := app.BaseApp.NewUncachedContext(true, tmproto.Header{})

		// Initialize pinned codes in wasmvm as they are not persisted there
		if err := app.wasmKeeper.InitializePinnedCodes(ctx); err != nil {
			tmos.Exit(fmt.Sprintf("failed initialize pinned codes %s", err))
		}
	}

	return app
}

// Name returns the name of the App
func (app *WasmApp) Name() string { return app.BaseApp.Name() }

// application updates every begin block
func (app *WasmApp) BeginBlocker(ctx sdk.Context, req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	return app.mm.BeginBlock(ctx, req)
}

// EndBlocker application updates every end block
func (app *WasmApp) EndBlocker(ctx sdk.Context, req abci.RequestEndBlock) abci.ResponseEndBlock {
	return app.mm.EndBlock(ctx, req)
}

// InitChainer application update at chain initialization
func (app *WasmApp) InitChainer(ctx sdk.Context, req abci.RequestInitChain) abci.ResponseInitChain {
	var genesisState GenesisState
	if err := tmjson.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}

	app.upgradeKeeper.SetModuleVersionMap(ctx, app.mm.GetVersionMap())

	return app.mm.InitGenesis(ctx, app.appCodec, genesisState)
}

// LoadHeight loads a particular height
func (app *WasmApp) LoadHeight(height int64) error {
	return app.LoadVersion(height)
}

// ModuleAccountAddrs returns all the app's module account addresses.
func (app *WasmApp) ModuleAccountAddrs() map[string]bool {
	modAccAddrs := make(map[string]bool)
	for acc := range maccPerms {
		modAccAddrs[authtypes.NewModuleAddress(acc).String()] = true
	}

	return modAccAddrs
}

// LegacyAmino returns legacy amino codec.
//
// NOTE: This is solely to be used for testing purposes as it may be desirable
// for modules to register their own custom testing types.
func (app *WasmApp) LegacyAmino() *codec.LegacyAmino { //nolint:staticcheck
	return app.legacyAmino
}

// getSubspace returns a param subspace for a given module name.
//
// NOTE: This is solely to be used for testing purposes.
func (app *WasmApp) getSubspace(moduleName string) paramstypes.Subspace {
	subspace, _ := app.paramsKeeper.GetSubspace(moduleName)
	return subspace
}

// SimulationManager implements the SimulationApp interface
func (app *WasmApp) SimulationManager() *module.SimulationManager {
	return app.sm
}

// RegisterAPIRoutes registers all application module routes with the provided
// API server.
func (app *WasmApp) RegisterAPIRoutes(apiSvr *api.Server, apiConfig config.APIConfig) {
	clientCtx := apiSvr.ClientCtx
	rpc.RegisterRoutes(clientCtx, apiSvr.Router)
	// Register legacy tx routes.
	authrest.RegisterTxRoutes(clientCtx, apiSvr.Router)
	// Register new tx routes from grpc-gateway.
	authtx.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	// Register new tendermint queries routes from grpc-gateway.
	tmservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// Register legacy and grpc-gateway routes for all modules.
	ModuleBasics.RegisterRESTRoutes(clientCtx, apiSvr.Router)
	ModuleBasics.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// register swagger API from root so that other applications can override easily
	if apiConfig.Swagger {
		RegisterSwaggerAPI(apiSvr.Router)
	}
}

// RegisterTxService implements the Application.RegisterTxService method.
func (app *WasmApp) RegisterTxService(clientCtx client.Context) {
	authtx.RegisterTxService(app.BaseApp.GRPCQueryRouter(), clientCtx, app.BaseApp.Simulate, app.interfaceRegistry)
}

// RegisterTendermintService implements the Application.RegisterTendermintService method.
func (app *WasmApp) RegisterTendermintService(clientCtx client.Context) {
	tmservice.RegisterTendermintService(app.BaseApp.GRPCQueryRouter(), clientCtx, app.interfaceRegistry)
}

func (app *WasmApp) AppCodec() codec.Codec {
	return app.appCodec
}

// RegisterSwaggerAPI registers swagger route with API Server
func RegisterSwaggerAPI(rtr *mux.Router) {
	statikFS, err := fs.New()
	if err != nil {
		panic(err)
	}

	staticServer := http.FileServer(statikFS)
	rtr.PathPrefix("/swagger/").Handler(http.StripPrefix("/swagger/", staticServer))
}

// GetMaccPerms returns a copy of the module account permissions
func GetMaccPerms() map[string][]string {
	dupMaccPerms := make(map[string][]string)
	for k, v := range maccPerms {
		dupMaccPerms[k] = v
	}
	return dupMaccPerms
}

// initParamsKeeper init params keeper and its subspaces
func initParamsKeeper(appCodec codec.BinaryCodec, legacyAmino *codec.LegacyAmino, key, tkey sdk.StoreKey) paramskeeper.Keeper {
	paramsKeeper := paramskeeper.NewKeeper(appCodec, legacyAmino, key, tkey)

	paramsKeeper.Subspace(authtypes.ModuleName)
	paramsKeeper.Subspace(banktypes.ModuleName)
	paramsKeeper.Subspace(slashingtypes.ModuleName)
	paramsKeeper.Subspace(crisistypes.ModuleName)
	paramsKeeper.Subspace(ibctransfertypes.ModuleName)
	paramsKeeper.Subspace(ibchost.ModuleName)
	paramsKeeper.Subspace(icahosttypes.SubModuleName)
	paramsKeeper.Subspace(icacontrollertypes.SubModuleName)
	paramsKeeper.Subspace(wasm.ModuleName)
	paramsKeeper.Subspace(ibcconsumertypes.ModuleName)

	return paramsKeeper
}

// TestingApp functions

// GetBaseApp implements the TestingApp interface.
func (app *WasmApp) GetBaseApp() *baseapp.BaseApp {
	return app.BaseApp
}

// GetStakingKeeper implements the TestingApp interface.
func (app *WasmApp) GetStakingKeeper() ibcclienttypes.StakingKeeper {
	return app.ConsumerKeeper
}

// GetIBCKeeper implements the TestingApp interface.
func (app *WasmApp) GetIBCKeeper() *ibckeeper.Keeper {
	return app.ibcKeeper
}

// GetScopedIBCKeeper implements the TestingApp interface.
func (app *WasmApp) GetScopedIBCKeeper() capabilitykeeper.ScopedKeeper {
	return app.scopedIBCKeeper
}

// GetTxConfig implements the TestingApp interface.
func (app *WasmApp) GetTxConfig() client.TxConfig {
	return cosmoscmd.MakeEncodingConfig(ModuleBasics).TxConfig
}

func (app *WasmApp) GetSlashingKeeper() slashingkeeper.Keeper {
	return app.slashingKeeper
}

func (app *WasmApp) GetEvidenceKeeper() evidencekeeper.Keeper {
	return app.evidenceKeeper
}
