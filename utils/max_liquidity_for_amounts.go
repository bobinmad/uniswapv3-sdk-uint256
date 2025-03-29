package utils

import (
	"github.com/KyberNetwork/uniswapv3-sdk-uint256/constants"
	"github.com/holiman/uint256"
)

type MaxLiquidityForAmountsCalculator struct {
	res  *uint256.Int
	res0 *uint256.Int
	res1 *uint256.Int
	tmp  *uint256.Int
	tmp2 *uint256.Int
}

func NewMaxLiquidityForAmountsCalculator() *MaxLiquidityForAmountsCalculator {
	return &MaxLiquidityForAmountsCalculator{
		res:  new(uint256.Int),
		res0: new(uint256.Int),
		res1: new(uint256.Int),
		tmp:  new(uint256.Int),
		tmp2: new(uint256.Int),
	}
}

/**
 * Returns an imprecise maximum amount of liquidity received for a given amount of token 0.
 * This function is available to accommodate LiquidityAmounts#getLiquidityForAmount0 in the v3 periphery,
 * which could be more precise by at least 32 bits by dividing by Q64 instead of Q96 in the intermediate step,
 * and shifting the subtracted ratio left by 32 bits. This imprecise calculation will likely be replaced in a future
 * v3 router contract.
 * @param sqrtRatioAX96 The price at the lower boundary
 * @param sqrtRatioBX96 The price at the upper boundary
 * @param amount0 The token0 amount
 * @returns liquidity for amount0, imprecise
 */
func (c *MaxLiquidityForAmountsCalculator) maxLiquidityForAmount0Imprecise(sqrtRatioAX96, sqrtRatioBX96, amount0 *uint256.Int, result *uint256.Int) {
	if sqrtRatioAX96.Gt(sqrtRatioBX96) {
		sqrtRatioAX96, sqrtRatioBX96 = sqrtRatioBX96, sqrtRatioAX96
	}

	intermediate := c.tmp.Div(c.tmp2.Mul(sqrtRatioAX96, sqrtRatioBX96), constants.Q96U256)
	result.Div(c.tmp.Mul(amount0, intermediate), c.tmp2.Sub(sqrtRatioBX96, sqrtRatioAX96))
}

/**
 * Returns a precise maximum amount of liquidity received for a given amount of token 0 by dividing by Q64 instead of Q96 in the intermediate step,
 * and shifting the subtracted ratio left by 32 bits.
 * @param sqrtRatioAX96 The price at the lower boundary
 * @param sqrtRatioBX96 The price at the upper boundary
 * @param amount0 The token0 amount
 * @returns liquidity for amount0, precise
 */
func (c *MaxLiquidityForAmountsCalculator) maxLiquidityForAmount0Precise(sqrtRatioAX96, sqrtRatioBX96, amount0 *uint256.Int, result *uint256.Int) {
	if sqrtRatioAX96.Gt(sqrtRatioBX96) {
		sqrtRatioAX96, sqrtRatioBX96 = sqrtRatioBX96, sqrtRatioAX96
	}

	numerator := new(uint256.Int).Mul(new(uint256.Int).Mul(amount0, sqrtRatioAX96), sqrtRatioBX96)
	denominator := new(uint256.Int).Mul(constants.Q96U256, new(uint256.Int).Sub(sqrtRatioBX96, sqrtRatioAX96))
	result.Div(numerator, denominator)
}

/**
 * Computes the maximum amount of liquidity received for a given amount of token1
 * @param sqrtRatioAX96 The price at the lower tick boundary
 * @param sqrtRatioBX96 The price at the upper tick boundary
 * @param amount1 The token1 amount
 * @returns liquidity for amount1
 */
func (c *MaxLiquidityForAmountsCalculator) maxLiquidityForAmount1(sqrtRatioAX96, sqrtRatioBX96, amount1 *uint256.Int, result *uint256.Int) {
	if sqrtRatioAX96.Gt(sqrtRatioBX96) {
		sqrtRatioAX96, sqrtRatioBX96 = sqrtRatioBX96, sqrtRatioAX96
	}

	result.Div(c.tmp.Mul(amount1, constants.Q96U256), c.tmp2.Sub(sqrtRatioBX96, sqrtRatioAX96))
}

/**
 * Computes the maximum amount of liquidity received for a given amount of token0, token1,
 * and the prices at the tick boundaries.
 * @param sqrtRatioCurrentX96 the current price
 * @param sqrtRatioAX96 price at lower boundary
 * @param sqrtRatioBX96 price at upper boundary
 * @param amount0 token0 amount
 * @param amount1 token1 amount
 * @param useFullPrecision if false, liquidity will be maximized according to what the router can calculate,
 * not what core can theoretically support
 */
func (c *MaxLiquidityForAmountsCalculator) MaxLiquidityForAmounts(sqrtRatioCurrentX96, sqrtRatioAX96, sqrtRatioBX96, amount0, amount1 *uint256.Int, useFullPrecision bool) *uint256.Int {
	if sqrtRatioAX96.Gt(sqrtRatioBX96) {
		sqrtRatioAX96, sqrtRatioBX96 = sqrtRatioBX96, sqrtRatioAX96
	}

	var maxLiquidityForAmount0 func(*uint256.Int, *uint256.Int, *uint256.Int, *uint256.Int)
	if useFullPrecision {
		maxLiquidityForAmount0 = c.maxLiquidityForAmount0Precise
	} else {
		maxLiquidityForAmount0 = c.maxLiquidityForAmount0Imprecise
	}

	if !sqrtRatioCurrentX96.Gt(sqrtRatioAX96) {
		maxLiquidityForAmount0(sqrtRatioAX96, sqrtRatioBX96, amount0, c.res)
		return c.res
	} else if sqrtRatioCurrentX96.Lt(sqrtRatioBX96) {
		// res0 := new(uint256.Int)
		res1 := new(uint256.Int)
		maxLiquidityForAmount0(sqrtRatioCurrentX96, sqrtRatioBX96, amount0, c.res0)
		c.maxLiquidityForAmount1(sqrtRatioAX96, sqrtRatioCurrentX96, amount1, res1)

		if c.res0.Lt(res1) {
			return c.res0
		}

		return res1
	}

	c.maxLiquidityForAmount1(sqrtRatioAX96, sqrtRatioBX96, amount1, c.res)
	return c.res
}
