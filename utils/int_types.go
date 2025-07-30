package utils

import (
	"errors"

	"github.com/KyberNetwork/int256"
	"github.com/holiman/uint256"
)

// define placeholders for these types, in case we need to customize them later
// (for example to add boundary check...)

type Uint256 = uint256.Int
type Uint160 = uint256.Int
type Uint128 = uint256.Int

type Int256 = int256.Int
type Int128 = int256.Int

type IntTypes struct {
	// yuint *Uint128
	// ba    [32]byte
}

func NewIntTypes() *IntTypes {
	return &IntTypes{
		// yuint: new(Uint128),
	}
}

var (
	ErrExceedMaxInt256 = errors.New("exceed max int256")
	ErrOverflowUint128 = errors.New("overflow uint128")
	ErrOverflowUint160 = errors.New("overflow uint160")

	Uint128Max = uint256.MustFromHex("0xffffffffffffffffffffffffffffffff")
	Uint160Max = uint256.MustFromHex("0xffffffffffffffffffffffffffffffffffffffff")
)

// deprecated, use pointer to Int256 instead
// https://github.com/Uniswap/v3-core/blob/main/contracts/libraries/SafeCast.sol
func ToInt256(value *Uint256, result *Int256) error {
	// if value (interpreted as a two's complement signed number) is negative -> it must be larger than max int256
	if value.Sign() < 0 {
		return ErrExceedMaxInt256
	}

	copy(result[:], value[:])
	// var ba [32]byte
	// value.WriteToArray32(&t.ba)
	// result.SetBytes32(t.ba[:])
	return nil
}

// deprecated, use pointer to Uint256 instead
func ToUInt256(value *Int256, result *Uint256) {
	copy(result[:], value[:])

	// var ba [32]byte
	// value.WriteToArray32(&t.ba)
	// result.SetBytes32(t.ba[:])
}

// https://github.com/Uniswap/v3-core/blob/main/contracts/libraries/SafeCast.sol
func CheckToUint160(value *Uint256) error {
	// we're using same type for Uint256 and Uint160, so use the original for now
	if value.Gt(Uint160Max) {
		return ErrOverflowUint160
	}
	return nil
}

// // x = x + y
// func (t *IntTypes) AddDeltaInPlaceV1(x *Uint128, y *Int128) error {
// 	// for now we're using int256 for Int128, and uint256 for Uint128
// 	// and both of them is using two's complement internally
// 	// so just cast `y` to uint256 and add them together
// 	// var ba [32]byte
// 	// y.WriteToArray32(&t.ba)
// 	// var yuint Uint128
// 	//t.yuint.SetBytes32(t.ba[:])

// 	copy(t.yuint[:], y[:])
// 	// t.yuint = (*Uint128)(y)
// 	if x.Add(x, t.yuint).Gt(Uint128Max) {
// 		// could be overflow or underflow
// 		return ErrOverflowUint128
// 	}
// 	return nil
// }

func AddDeltaInPlace(x *Uint128, y *Int128) {
	x.Add(x, (*Uint128)(y))
}
