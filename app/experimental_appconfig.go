package app

import (

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"

	"github.com/cosmos/cosmos-sdk/types/module"


	group "github.com/cosmos/cosmos-sdk/x/group/module"

	regenmoduletypes "github.com/regen-network/regen-ledger/types/module"
	regenserver "github.com/regen-network/regen-ledger/types/module/server"
)

func setCustomModuleBasics() []module.AppModuleBasic {
	return []module.AppModuleBasic{
		group.Module{},
	}
}


// setCustomModules registers new modules with the server module manager.
func setCustomModules(app *App, interfaceRegistry types.InterfaceRegistry) *regenserver.Manager {

	/* New Module Wiring START */
	newModuleManager := regenserver.NewManager(app.BaseApp, codec.NewProtoCodec(interfaceRegistry))

	// BEGIN HACK: this is a total, ugly hack until x/auth supports ADR 033 or we have a suitable alternative
	groupModule := group.Module{AccountKeeper: app.AccountKeeper}
	// use a separate newModules from the global NewModules here because we need to pass state into the group module
	newModules := []regenmoduletypes.Module{
		groupModule,
	}
	err := newModuleManager.RegisterModules(newModules)
	if err != nil {
		panic(err)
	}
	// END HACK

	err = newModuleManager.CompleteInitialization()
	if err != nil {
		panic(err)
	}

	return newModuleManager
	/* New Module Wiring END */
}

