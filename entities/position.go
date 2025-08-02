package entities

import (
	"errors"
	"math/big"

	"github.com/KyberNetwork/uniswapv3-sdk-uint256/constants"
	"github.com/KyberNetwork/uniswapv3-sdk-uint256/utils"
	"github.com/daoleno/uniswap-sdk-core/entities"
	"github.com/holiman/uint256"
)

var (
	ErrTickOrder = errors.New("tick order error")
	ErrTickLower = errors.New("tick lower error")
	ErrTickUpper = errors.New("tick upper error")
	Zero         = uint256.NewInt(0)
	MaxUint256U  = uint256.MustFromBig(entities.MaxUint256)
)

// Position Represents a position on a Uniswap V3 Pool
type Position struct {
	Pool       *Pool
	TickLower  int32
	TickUpper  int32
	PriceLower *utils.Uint160
	PriceUpper *utils.Uint160
	Liquidity  *utils.Uint128

	// static cache
	token0Amount           *entities.CurrencyAmount
	token1Amount           *entities.CurrencyAmount
	mintAmounts            []*uint256.Int
	amount1Tmp, amount2Tmp *utils.Uint256
}

/**
 * Constructs a position for a given pool with the given liquidity
 * @param pool For which pool the liquidity is assigned
 * @param liquidity The amount of liquidity that is in the position
 * @param tickLower The lower tick of the position
 * @param tickUpper The upper tick of the position
 */
func NewPosition(pool *Pool, liquidity *uint256.Int, tickLower, tickUpper int32) (*Position, error) {
	if tickLower >= tickUpper {
		return nil, ErrTickOrder
	}
	if tickLower < utils.MinTick || tickLower%int32(pool.TickSpacing) != 0 {
		return nil, ErrTickLower
	}
	if tickUpper > utils.MaxTick || tickUpper%int32(pool.TickSpacing) != 0 {
		return nil, ErrTickUpper
	}

	position := Position{
		Pool:       pool,
		Liquidity:  liquidity,
		TickLower:  tickLower,
		TickUpper:  tickUpper,
		PriceLower: new(utils.Uint160),
		PriceUpper: new(utils.Uint160),
		amount1Tmp: new(utils.Uint256),
		amount2Tmp: new(utils.Uint256),
	}

	// сразу посчитаем цены границ позиции, они часто бывают необходимы
	pool.TickCalculator.GetSqrtRatioAtTickV2(tickLower, position.PriceLower)
	pool.TickCalculator.GetSqrtRatioAtTickV2(tickUpper, position.PriceUpper)

	return &position, nil
}

// Token0PriceLower Returns the price of token0 at the lower tick
func (p *Position) Token0PriceLower() (*entities.Price, error) {
	return utils.TickToPrice(p.Pool.Token0, p.Pool.Token1, p.TickLower)
}

// Token0PriceUpper Returns the price of token0 at the upper tick
func (p *Position) Token0PriceUpper() (*entities.Price, error) {
	return utils.TickToPrice(p.Pool.Token0, p.Pool.Token1, p.TickUpper)
}

// Amount0 Returns the amount of token0 that this position's liquidity could be burned for at the current pool price
func (p *Position) Amount0(forceRecalc bool) (*entities.CurrencyAmount, error) {
	if forceRecalc || p.token0Amount == nil {
		if p.Pool.TickCurrent < p.TickLower {
			p.token0Amount = entities.FromRawAmount(p.Pool.Token0, utils.GetAmount0Delta(p.PriceLower.ToBig(), p.PriceUpper.ToBig(), p.Liquidity.ToBig(), false))
		} else if p.Pool.TickCurrent < p.TickUpper {
			p.token0Amount = entities.FromRawAmount(p.Pool.Token0, utils.GetAmount0Delta(p.Pool.SqrtRatioX96.ToBig(), p.PriceUpper.ToBig(), p.Liquidity.ToBig(), true))
		} else {
			p.token0Amount = entities.FromRawAmount(p.Pool.Token0, constants.Zero)
		}
	}
	return p.token0Amount, nil
}

// Amount1 Returns the amount of token1 that this position's liquidity could be burned for at the current pool price
func (p *Position) Amount1(forceRecalc bool) (*entities.CurrencyAmount, error) {
	if forceRecalc || p.token1Amount == nil {
		if p.Pool.TickCurrent < p.TickLower {
			p.token1Amount = entities.FromRawAmount(p.Pool.Token1, constants.Zero)
		} else if p.Pool.TickCurrent < p.TickUpper {
			p.token1Amount = entities.FromRawAmount(p.Pool.Token1, utils.GetAmount1Delta(p.PriceLower.ToBig(), p.Pool.SqrtRatioX96.ToBig(), p.Liquidity.ToBig(), false))
		} else {
			p.token1Amount = entities.FromRawAmount(p.Pool.Token1, utils.GetAmount1Delta(p.PriceLower.ToBig(), p.PriceUpper.ToBig(), p.Liquidity.ToBig(), false))
		}
	}
	return p.token1Amount, nil
}

func (p *Position) CalcAmount0() *utils.Uint256 {
	if p.Pool.TickCurrent < p.TickLower {
		p.Pool.SqrtPriceCalculator.GetAmount0DeltaV2(p.PriceLower, p.PriceUpper, p.Liquidity, false, p.amount2Tmp)
		return p.amount2Tmp
	} else if p.Pool.TickCurrent < p.TickUpper {
		p.Pool.SqrtPriceCalculator.GetAmount0DeltaV2(p.Pool.SqrtRatioX96, p.PriceUpper, p.Liquidity, false, p.amount2Tmp)
		return p.amount2Tmp
	}

	return Zero
}

func (p *Position) CalcAmount1() *utils.Uint256 {
	if p.Pool.TickCurrent < p.TickLower {
		return Zero
	} else if p.Pool.TickCurrent < p.TickUpper {
		p.Pool.SqrtPriceCalculator.GetAmount1DeltaV2(p.PriceLower, p.Pool.SqrtRatioX96, p.Liquidity, false, p.amount2Tmp)

		return p.amount2Tmp
	} else {
		p.Pool.SqrtPriceCalculator.GetAmount1DeltaV2(p.PriceLower, p.PriceUpper, p.Liquidity, false, p.amount2Tmp)

		return p.amount2Tmp
	}
}

func (p *Position) CalcAmounts() (*utils.Uint256, *utils.Uint256) {
	if p.Pool.TickCurrent < p.TickLower {
		// calc amount0
		p.Pool.SqrtPriceCalculator.GetAmount0DeltaV2(p.PriceLower, p.PriceUpper, p.Liquidity, true, p.amount1Tmp)

		// amount1 is zero
		return p.amount1Tmp, Zero
	} else if p.Pool.TickCurrent < p.TickUpper {
		// calc amount0
		p.Pool.SqrtPriceCalculator.GetAmount0DeltaV2(p.Pool.SqrtRatioX96, p.PriceUpper, p.Liquidity, true, p.amount1Tmp)

		// calc amount1
		p.Pool.SqrtPriceCalculator.GetAmount1DeltaV2(p.PriceLower, p.Pool.SqrtRatioX96, p.Liquidity, true, p.amount2Tmp)

		return p.amount1Tmp, p.amount2Tmp
	} else {
		// calc amount1
		p.Pool.SqrtPriceCalculator.GetAmount1DeltaV2(p.PriceLower, p.PriceUpper, p.Liquidity, true, p.amount2Tmp)

		// amount0 is zero
		return Zero, p.amount2Tmp
	}
}

/**
 * Returns the lower and upper sqrt ratios if the price 'slips' up to slippage tolerance percentage
 * @param slippageTolerance The amount by which the price can 'slip' before the transaction will revert
 * @returns The sqrt ratios after slippage
 */
func (p *Position) ratiosAfterSlippage(slippageTolerance *entities.Percent) (sqrtRatioX96Lower *uint256.Int, sqrtRatioX96Upper *uint256.Int) {
	priceLower := p.Pool.Token0Price().Fraction.Multiply(entities.NewPercent(big.NewInt(1), big.NewInt(1)).Subtract(slippageTolerance).Fraction)
	priceUpper := p.Pool.Token0Price().Fraction.Multiply(entities.NewPercent(big.NewInt(1), big.NewInt(1)).Add(slippageTolerance).Fraction)

	sqrtRatioX96Lower = utils.EncodeSqrtRatioX96(priceLower.Numerator, priceLower.Denominator)
	if sqrtRatioX96Lower.Cmp(utils.MinSqrtRatioU256) <= 0 {
		// if !sqrtRatioX96Lower.Gt(utils.MinSqrtRatioU256) {
		sqrtRatioX96Lower = new(uint256.Int).Add(utils.MinSqrtRatioU256, utils.U256One)
	}

	sqrtRatioX96Upper = utils.EncodeSqrtRatioX96(priceUpper.Numerator, priceUpper.Denominator)
	if sqrtRatioX96Upper.Cmp(utils.MaxSqrtRatioU256) >= 0 {
		// if !sqrtRatioX96Upper.Lt(utils.MaxSqrtRatioU256) {
		sqrtRatioX96Upper = new(uint256.Int).Sub(utils.MinSqrtRatioU256, utils.U256One)
	}

	// sqrtRatioX96Lower = utils.EncodeSqrtRatioX96(uint256.MustFromBig(priceLower.Numerator), uint256.MustFromBig(priceLower.Denominator))
	// if sqrtRatioX96Lower.Cmp(utils.MinSqrtRatioU256) <= 0 {
	// 	sqrtRatioX96Lower = new(uint256.Int).Add(utils.MinSqrtRatioU256, utils.U256One)
	// }
	// sqrtRatioX96Upper = utils.EncodeSqrtRatioX96(uint256.MustFromBig(priceUpper.Numerator), uint256.MustFromBig(priceUpper.Denominator))
	// if sqrtRatioX96Upper.Cmp(utils.MaxSqrtRatioU256) >= 0 {
	// 	sqrtRatioX96Upper = new(uint256.Int).Sub(utils.MaxSqrtRatioU256, utils.U256One)
	// }

	return sqrtRatioX96Lower, sqrtRatioX96Upper
}

/**
* Returns the minimum amounts that must be sent in order to safely mint the amount of liquidity held by the position
* with the given slippage tolerance
* @param slippageTolerance Tolerance of unfavorable slippage from the current price
* @returns The amounts, with slippage
 */
func (p *Position) MintAmountsWithSlippage(slippageTolerance *entities.Percent) (amount0, amount1 *uint256.Int, err error) {
	// get lower/upper prices
	sqrtRatioX96Upper, sqrtRatioX96Lower := p.ratiosAfterSlippage(slippageTolerance)

	// construct counterfactual pools
	tickLower, err := p.Pool.TickCalculator.GetTickAtSqrtRatioV2(sqrtRatioX96Lower)
	if err != nil {
		return nil, nil, err
	}

	poolLower := NewPoolV3(uint16(p.Pool.Fee), tickLower, sqrtRatioX96Lower, p.Pool.Token0, p.Pool.Token1, nil)

	tickUpper, err := p.Pool.TickCalculator.GetTickAtSqrtRatioV2(sqrtRatioX96Upper)
	if err != nil {
		return nil, nil, err
	}

	poolUpper := NewPoolV3(uint16(p.Pool.Fee), tickUpper, sqrtRatioX96Upper, p.Pool.Token0, p.Pool.Token1, nil)

	// because the router is imprecise, we need to calculate the position that will be created (assuming no slippage)
	// the mint amounts are what will be passed as calldata
	a0, a1, err := p.MintAmounts()
	if err != nil {
		return nil, nil, err
	}
	positionThatWillBeCreated, err := FromAmounts(p.Pool, p.TickLower, p.TickUpper, a0, a1, false)
	if err != nil {
		return nil, nil, err
	}

	// we want the smaller amounts...
	// ...which occurs at the upper price for amount0...
	positionUpper, err := NewPosition(poolUpper, positionThatWillBeCreated.Liquidity, p.TickLower, p.TickUpper)
	if err != nil {
		return nil, nil, err
	}
	// ...and the lower for amount1
	positionLower, err := NewPosition(poolLower, positionThatWillBeCreated.Liquidity, p.TickLower, p.TickUpper)
	if err != nil {
		return nil, nil, err
	}
	amount0, _, err = positionLower.MintAmounts()
	if err != nil {
		return nil, nil, err
	}
	_, amount1, err = positionUpper.MintAmounts()
	if err != nil {
		return nil, nil, err
	}
	return amount0, amount1, nil
}

/**
 * Returns the minimum amounts that should be requested in order to safely burn the amount of liquidity held by the
 * position with the given slippage tolerance
 * @param slippageTolerance tolerance of unfavorable slippage from the current price
 * @returns The amounts, with slippage
 */
func (p *Position) BurnAmountsWithSlippage(slippageTolerance *entities.Percent) (amount0, amount1 *uint256.Int, err error) {
	// get lower/upper prices
	sqrtRatioX96Lower, sqrtRatioX96Upper := p.ratiosAfterSlippage(slippageTolerance)

	// construct counterfactual pools
	tickLower, err := p.Pool.TickCalculator.GetTickAtSqrtRatioV2(sqrtRatioX96Lower)
	if err != nil {
		return nil, nil, err
	}
	poolLower := NewPoolV3(uint16(p.Pool.Fee), tickLower, sqrtRatioX96Lower, p.Pool.Token0, p.Pool.Token1, nil)

	tickUpper, err := p.Pool.TickCalculator.GetTickAtSqrtRatioV2(sqrtRatioX96Upper)
	if err != nil {
		return nil, nil, err
	}
	poolUpper := NewPoolV3(uint16(p.Pool.Fee), tickUpper, sqrtRatioX96Upper, p.Pool.Token0, p.Pool.Token1, nil)

	// we want the smaller amounts...
	// ...which occurs at the upper price for amount0...
	positionUpper, err := NewPosition(poolUpper, p.Liquidity, p.TickLower, p.TickUpper)
	if err != nil {
		return nil, nil, err
	}
	// ...and the lower for amount1
	positionLower, err := NewPosition(poolLower, p.Liquidity, p.TickLower, p.TickUpper)
	if err != nil {
		return nil, nil, err
	}

	return positionUpper.CalcAmount0(), positionLower.CalcAmount1(), nil
}

/**
 * Returns the minimum amounts that must be sent in order to mint the amount of liquidity held by the position at
 * the current price for the pool
 */
func (p *Position) MintAmounts() (amount0, amount1 *uint256.Int, err error) {
	if p.mintAmounts == nil {
		var (
			amount0 = new(utils.Uint256)
			amount1 = new(utils.Uint256)
		)

		if p.Pool.TickCurrent < p.TickLower {
			p.Pool.SqrtPriceCalculator.GetAmount0DeltaV2(p.PriceLower, p.PriceUpper, p.Liquidity, true, amount0)
			amount1 = constants.ZeroU256
			return amount0, amount1, nil
		} else if p.Pool.TickCurrent < p.TickUpper {
			p.Pool.SqrtPriceCalculator.GetAmount0DeltaV2(p.Pool.SqrtRatioX96, p.PriceUpper, p.Liquidity, true, amount0)
			p.Pool.SqrtPriceCalculator.GetAmount1DeltaV2(p.PriceLower, p.Pool.SqrtRatioX96, p.Liquidity, true, amount1)
		} else {
			amount0 = constants.ZeroU256
			p.Pool.SqrtPriceCalculator.GetAmount1DeltaV2(p.PriceLower, p.PriceUpper, p.Liquidity, true, amount1)
		}

		return amount0, amount1, nil
	}

	return p.mintAmounts[0], p.mintAmounts[1], nil
}

/**
 * Computes the maximum amount of liquidity received for a given amount of token0, token1,
 * and the prices at the tick boundaries.
 * @param pool The pool for which the position should be created
 * @param tickLower The lower tick of the position
 * @param tickUpper The upper tick of the position
 * @param amount0 token0 amount
 * @param amount1 token1 amount
 * @param useFullPrecision If false, liquidity will be maximized according to what the router can calculate,
 * not what core can theoretically support
 * @returns The amount of liquidity for the position
 */
func FromAmounts(pool *Pool, tickLower, tickUpper int32, amount0, amount1 *uint256.Int, useFullPrecision bool) (*Position, error) {
	sqrtRatioAX96 := new(utils.Uint160)
	pool.TickCalculator.GetSqrtRatioAtTickV2(tickLower, sqrtRatioAX96)

	sqrtRatioBX96 := new(utils.Uint160)
	pool.TickCalculator.GetSqrtRatioAtTickV2(tickUpper, sqrtRatioBX96)

	return NewPosition(pool, utils.NewMaxLiquidityForAmountsCalculator().MaxLiquidityForAmounts(pool.SqrtRatioX96, sqrtRatioAX96, sqrtRatioBX96, amount0, amount1, useFullPrecision), tickLower, tickUpper)
}

/**
 * Computes a position with the maximum amount of liquidity received for a given amount of token0, assuming an unlimited amount of token1
 * @param pool The pool for which the position is created
 * @param tickLower The lower tick
 * @param tickUpper The upper tick
 * @param amount0 The desired amount of token0
 * @param useFullPrecision If true, liquidity will be maximized according to what the router can calculate,
 * not what core can theoretically support
 * @returns The position
 */
func FromAmount0(pool *Pool, tickLower, tickUpper int32, amount0 *uint256.Int, useFullPrecision bool) (*Position, error) {
	return FromAmounts(pool, tickLower, tickUpper, amount0, MaxUint256U, useFullPrecision)
}

/**
 * Computes a position with the maximum amount of liquidity received for a given amount of token1, assuming an unlimited amount of token0
 * @param pool The pool for which the position is created
 * @param tickLower The lower tick
 * @param tickUpper The upper tick
 * @param amount1 The desired amount of token1
 * @returns The position
 */
func FromAmount1(pool *Pool, tickLower, tickUpper int32, amount1 *uint256.Int) (*Position, error) {
	// this function always uses full precision,
	return FromAmounts(pool, tickLower, tickUpper, MaxUint256U, amount1, true)
}
