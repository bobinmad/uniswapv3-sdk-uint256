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
func EncodeSqrtRatioX96(amount1, amount0 *big.Int) *uint256.Int {
	// здесь почему-то нормально считает только на big.Int

	ratioX192 := new(big.Int).Div(new(big.Int).Lsh(amount1, 192), amount0)
	return uint256.MustFromBig(new(big.Int).Sqrt(ratioX192))

	// numerator := new(uint256.Int).Lsh(uint256.MustFromBig(amount1), 192)
	// denominator := uint256.MustFromBig(amount0)
	// ratioX192 := new(uint256.Int).Div(numerator, denominator)
	// return new(uint256.Int).Sqrt(ratioX192)
}
