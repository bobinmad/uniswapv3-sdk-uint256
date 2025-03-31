package utils

import (
	"math/big"

	"github.com/holiman/uint256"
)

/**
 * Returns the sqrt ratio as a Q64.96 corresponding to a given ratio of amount1 and amount0
 * @param amount1 The numerator amount i.e., the amount of token1
 * @param amount0 The denominator amount i.e., the amount of token0
 * @returns The sqrt ratio
 */
func EncodeSqrtRatioX96(amount1, amount0 *uint256.Int) *uint256.Int {
	// здесь почему-то нормально считает только на big.Int
	numerator := new(big.Int).Lsh(amount1.ToBig(), 192)
	denominator := amount0.ToBig()
	ratioX192 := new(big.Int).Div(numerator, denominator)
	return uint256.MustFromBig(new(big.Int).Sqrt(ratioX192))

	// numerator := new(uint256.Int).Lsh(amount1, 192)
	// denominator := amount0
	// ratioX192 := new(uint256.Int).Div(numerator, denominator)
	// return new(uint256.Int).Sqrt(ratioX192)
}
