package integration

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/stdlib"
	evmstdlib "github.com/onflow/flow-go/fvm/evm/stdlib"
)

// FVMtandardLibraryValues returns the standard library values which are provided by the FVM
// these are not part of the Cadence standard library
func FVMStandardLibraryValues() []stdlib.StandardLibraryValue {
	return []stdlib.StandardLibraryValue{
		// InternalEVM contract
		{
			Name:  evmstdlib.InternalEVMContractName,
			Type:  evmstdlib.InternalEVMContractType,
			Value: evmstdlib.NewInternalEVMContractValue(nil, nil, common.AddressLocation{}),
			Kind:  common.DeclarationKindContract,
		},
	}
}
