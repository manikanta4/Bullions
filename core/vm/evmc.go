// Copyright 2019 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Implements interaction with EVMC-based VMs.
// https://github.com/ethereum/evmc

package vm

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/evmc/bindings/go/evmc"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params/vars"
	"github.com/holiman/uint256"
)

// EVMC represents the reference to a common EVMC-based VM instance and
// the current execution context as required by go-ethereum design.
type EVMC struct {
	instance *evmc.Instance  // The reference to the EVMC VM instance.
	env      *EVM            // The execution context.
	cap      evmc.Capability // The supported EVMC capability (EVM or Ewasm)
	readOnly bool            // The readOnly flag (TODO: Try to get rid of it).
}

var (
	evmModule       *evmc.Instance
	ewasmModule     *evmc.Instance
	evmcModuleError = errors.New("EVMC internal error")
)

func InitEVMCEVM(config string) {
	evmModule = initEVMC(evmc.CapabilityEVM1, config)
}

func InitEVMCEwasm(config string) {
	ewasmModule = initEVMC(evmc.CapabilityEWASM, config)
}

func initEVMC(cap evmc.Capability, config string) *evmc.Instance {
	options := strings.Split(config, ",")
	path := options[0]

	if path == "" {
		panic("EVMC VM path not provided, set --vm.(evm|ewasm)=/path/to/vm")
	}

	instance, err := evmc.Load(path)
	if err != nil {
		panic(err.Error())
	}
	log.Info("EVMC VM loaded", "name", instance.Name(), "version", instance.Version(), "path", path)

	// Set options before checking capabilities.
	for _, option := range options[1:] {
		if idx := strings.Index(option, "="); idx >= 0 {
			name := option[:idx]
			value := option[idx+1:]
			err := instance.SetOption(name, value)
			if err == nil {
				log.Info("EVMC VM option set", "name", name, "value", value)
			} else {
				log.Warn("EVMC VM option setting failed", "name", name, "error", err)
			}
		}
	}

	if !instance.HasCapability(cap) {
		panic(fmt.Errorf("The EVMC module %s does not have requested capability %d", path, cap))
	}
	return instance
}

// hostContext implements evmc.HostContext interface.
type hostContext struct {
	env      *EVM      // The reference to the EVM execution context.
	contract *Contract // The reference to the current contract, needed by Call-like methods.
}

func (host *hostContext) AccountExists(addr common.Address) bool {
	// if host.env.ChainConfig().IsEIP158(host.env.Context.BlockNumber) {
	if host.env.ChainConfig().IsEnabled(host.env.ChainConfig().GetEIP161dTransition, host.env.Context.BlockNumber) {
		if !host.env.StateDB.Empty(addr) {
			return true
		}
	} else if host.env.StateDB.Exist(addr) {
		return true
	}
	return false
}

func (host *hostContext) GetStorage(addr common.Address, key common.Hash) common.Hash {
	return host.env.StateDB.GetState(addr, key)
}

func (host *hostContext) SetStorage(addr common.Address, key common.Hash, value common.Hash) (status evmc.StorageStatus) {
	oldValue := host.env.StateDB.GetState(addr, key)
	if oldValue == value {
		return evmc.StorageUnchanged
	}

	current := host.env.StateDB.GetState(addr, key)
	original := host.env.StateDB.GetCommittedState(addr, key)

	host.env.StateDB.SetState(addr, key, value)
	
	// Here's a great example of one of the limits of our (core-geth) current chainconfig interface model.
	// Should we handle the logic here about historic-featuro logic (which really is nice, because when reading the strange-incantation implemations, it's nice to see why it is),
	// or should we handle the question where we handle the rest of the questions like this, since this logic is
	// REALLY logic that belongs to the abstract idea of a chainconfiguration (aka chainconfig), which makes sense
	// but depends on ECIPs having steadier and more predictable logic.
	hasEIP2200 := host.env.ChainConfig().IsEnabled(host.env.ChainConfig().GetEIP2200Transition, host.env.Context.BlockNumber)
	hasEIP1884 := host.env.ChainConfig().IsEnabled(host.env.ChainConfig().GetEIP1884Transition, host.env.Context.BlockNumber)

	// Here's an example where to me, it makes sense to use individual EIPs in the code...
	// when they don't specify each other in the spec.
	hasNetStorageCostEIP := hasEIP2200 && hasEIP1884
	if !hasNetStorageCostEIP {

		zero := common.Hash{}
		status = evmc.StorageModified
		if oldValue == zero {
			return evmc.StorageAdded
		} else if value == zero {
			host.env.StateDB.AddRefund(vars.SstoreRefundGas)
			return evmc.StorageDeleted
		}
		return evmc.StorageModified
	}

	resetClearRefund := vars.NetSstoreResetClearRefund
	cleanRefund := vars.NetSstoreResetRefund

	if hasEIP2200 {
		resetClearRefund = vars.SstoreSetGasEIP2200 - vars.SloadGasEIP2200 // 19200
		cleanRefund = vars.SstoreResetGasEIP2200 - vars.SloadGasEIP2200 // 4200
	}

	if original == current {
		if original == (common.Hash{}) { // create slot (2.1.1)
			return evmc.StorageAdded
		}
		if value == (common.Hash{}) { // delete slot (2.1.2b)
			host.env.StateDB.AddRefund(vars.NetSstoreClearRefund)
			return evmc.StorageDeleted
		}
		return evmc.StorageModified
	}
	if original != (common.Hash{}) {
		if current == (common.Hash{}) { // recreate slot (2.2.1.1)
			host.env.StateDB.SubRefund(vars.NetSstoreClearRefund)
		} else if value == (common.Hash{}) { // delete slot (2.2.1.2)
			host.env.StateDB.AddRefund(vars.NetSstoreClearRefund)
		}
	}
	if original == value {
		if original == (common.Hash{}) { // reset to original inexistent slot (2.2.2.1)
			host.env.StateDB.AddRefund(resetClearRefund)
		} else { // reset to original existing slot (2.2.2.2)
			host.env.StateDB.AddRefund(cleanRefund)
		}
	}
	return evmc.StorageModifiedAgain
}

func (host *hostContext) GetBalance(addr common.Address) common.Hash {
	return common.BigToHash(host.env.StateDB.GetBalance(addr))
}

func (host *hostContext) GetCodeSize(addr common.Address) int {
	return host.env.StateDB.GetCodeSize(addr)
}

func (host *hostContext) GetCodeHash(addr common.Address) common.Hash {
	if host.env.StateDB.Empty(addr) {
		return common.Hash{}
	}
	return host.env.StateDB.GetCodeHash(addr)
}

func (host *hostContext) GetCode(addr common.Address) []byte {
	return host.env.StateDB.GetCode(addr)
}

func (host *hostContext) Selfdestruct(addr common.Address, beneficiary common.Address) {
	db := host.env.StateDB
	if !db.HasSuicided(addr) {
		db.AddRefund(vars.SelfdestructRefundGas)
	}
	db.AddBalance(beneficiary, db.GetBalance(addr))
	db.Suicide(addr)
}

func (host *hostContext) GetTxContext() evmc.TxContext {
	return evmc.TxContext{
		GasPrice:   common.BigToHash(host.env.GasPrice),
		Origin:     host.env.Origin,
		Coinbase:   host.env.Context.Coinbase,
		Number:     host.env.Context.BlockNumber.Int64(),
		Timestamp:  host.env.Context.Time.Int64(),
		GasLimit:   int64(host.env.Context.GasLimit),
		Difficulty: common.BigToHash(host.env.Context.Difficulty),
		//ChainID:    common.BigToHash(host.env.chainConfig.GetChainID()),
	}
}

func (host *hostContext) GetBlockHash(number int64) common.Hash {
	b := host.env.Context.BlockNumber.Int64()
	if number >= (b-256) && number < b {
		return host.env.Context.GetHash(uint64(number))
	}
	return common.Hash{}
}

func (host *hostContext) EmitLog(addr common.Address, topics []common.Hash, data []byte) {
	host.env.StateDB.AddLog(&types.Log{
		Address:     addr,
		Topics:      topics,
		Data:        data,
		BlockNumber: host.env.Context.BlockNumber.Uint64(),
	})
}

func (host *hostContext) Call(kind evmc.CallKind,
	destination common.Address, sender common.Address, value *big.Int, input []byte, gas int64, depth int,
	static bool, salt *big.Int) (output []byte, gasLeft int64, createAddr common.Address, err error) {

	gasU := uint64(gas)
	var gasLeftU uint64

	switch kind {
	case evmc.Call:
		if static {
			output, gasLeftU, err = host.env.StaticCall(host.contract, destination, input, gasU)
		} else {
			output, gasLeftU, err = host.env.Call(host.contract, destination, input, gasU, value)
		}
	case evmc.DelegateCall:
		output, gasLeftU, err = host.env.DelegateCall(host.contract, destination, input, gasU)
	case evmc.CallCode:
		output, gasLeftU, err = host.env.CallCode(host.contract, destination, input, gasU, value)
	case evmc.Create:
		var createOutput []byte
		createOutput, createAddr, gasLeftU, err = host.env.Create(host.contract, input, gasU, value)
		isHomestead := host.env.ChainConfig().IsEnabled(host.env.ChainConfig().GetEIP7Transition, host.env.Context.BlockNumber)
		if !isHomestead && err == ErrCodeStoreOutOfGas {
			err = nil
		}
		if err == ErrExecutionReverted {
			// Assign return buffer from REVERT.
			// TODO: Bad API design: return data buffer and the code is returned in the same place. In worst case
			//       the code is returned also when there is not enough funds to deploy the code.
			output = createOutput
		}
	case evmc.Create2:
		var createOutput []byte
		var saltUint256 = new(uint256.Int)
		saltUint256.SetUint64(salt.Uint64())
		createOutput, createAddr, gasLeftU, err = host.env.Create2(host.contract, input, gasU, value, saltUint256)
		if err == ErrExecutionReverted {
			// Assign return buffer from REVERT.
			// TODO: Bad API design: return data buffer and the code is returned in the same place. In worst case
			//       the code is returned also when there is not enough funds to deploy the code.
			output = createOutput
		}
	default:
		panic(fmt.Errorf("EVMC: Unknown call kind %d", kind))
	}

	// Map errors.
	if err == ErrExecutionReverted {
		err = evmc.Revert
	} else if err != nil {
		err = evmc.Failure
	}

	gasLeft = int64(gasLeftU)
	return output, gasLeft, createAddr, err
}

// getRevision translates ChainConfig's HF block information into EVMC revision.
func getRevision(env *EVM) evmc.Revision {
	n := env.Context.BlockNumber
	conf := env.ChainConfig()
	switch {
	// This is an example of choosing to use an "abstracted" idea
	// about chain config, where I'm choosing to prioritize "indicative" features
	// as identifiers for Fork-Feature-Groups. Note that this is very different
	// than using Feature-complete sets to assert "did Forkage."
	case conf.IsEnabled(conf.GetEIP1884Transition, n):
		return evmc.Istanbul
	case conf.IsEnabled(conf.GetEIP1283DisableTransition, n):
		return evmc.Petersburg
	case conf.IsEnabled(conf.GetEIP145Transition, n):
		return evmc.Constantinople
	case conf.IsEnabled(conf.GetEIP198Transition, n):
		return evmc.Byzantium
	case conf.IsEnabled(conf.GetEIP155Transition, n):
		return evmc.SpuriousDragon
	case conf.IsEnabled(conf.GetEIP150Transition, n):
		return evmc.TangerineWhistle
	case conf.IsEnabled(conf.GetEIP7Transition, n):
		return evmc.Homestead
	default:
		return evmc.Frontier
	}
}

// Run implements Interpreter.Run().
func (evm *EVMC) Run(contract *Contract, input []byte, readOnly bool) (ret []byte, err error) {
	evm.env.depth++
	defer func() { evm.env.depth-- }()

	// Don't bother with the execution if there's no code.
	if len(contract.Code) == 0 {
		return nil, nil
	}

	kind := evmc.Call
	if evm.env.StateDB.GetCodeSize(contract.Address()) == 0 {
		// Guess if this is a CREATE.
		kind = evmc.Create
	}

	// Make sure the readOnly is only set if we aren't in readOnly yet.
	// This makes also sure that the readOnly flag isn't removed for child calls.
	if readOnly && !evm.readOnly {
		evm.readOnly = true
		defer func() { evm.readOnly = false }()
	}

	output, gasLeft, err := evm.instance.Execute(
		&hostContext{evm.env, contract},
		getRevision(evm.env),
		kind,
		evm.readOnly,
		evm.env.depth-1,
		int64(contract.Gas),
		contract.Address(),
		contract.Caller(),
		input,
		common.BigToHash(contract.Value()),
		contract.Code,
		common.Hash{})

	contract.Gas = uint64(gasLeft)

	if err == evmc.Revert {
		err = ErrExecutionReverted
	} else if evmcError, ok := err.(evmc.Error); ok && evmcError.IsInternalError() {
		//panic(fmt.Sprintf("EVMC VM internal error: %s", evmcError.Error()))
		fmt.Println(fmt.Errorf("%s: %v (%v)", evmcModuleError, evmcError.Error(), err.Error()))
		return nil, fmt.Errorf("%s: %v", evmcModuleError, evmcError.Error())
	}

	return output, err
}

// CanRun implements Interpreter.CanRun().
func (evm *EVMC) CanRun(code []byte) bool {
	required := evmc.CapabilityEVM1
	wasmPreamble := []byte("\x00asm")
	if bytes.HasPrefix(code, wasmPreamble) {
		required = evmc.CapabilityEWASM
	}
	return evm.cap == required
}
