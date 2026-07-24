package utils

import "math/big"

// MethodParameters contains the calldata and native token value required to
// execute a periphery contract call.
type MethodParameters struct {
	Calldata []byte
	Value    *big.Int
}

// ToHex converts a big integer to the even-length hexadecimal representation
// used by the SDK tests and examples.
func ToHex(i *big.Int) string {
	if i == nil {
		return "0x00"
	}

	hex := i.Text(16)
	if len(hex)%2 != 0 {
		hex = "0" + hex
	}
	return "0x" + hex
}
