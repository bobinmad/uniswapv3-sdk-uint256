package entities

import (
	"math/big"
	"testing"

	"github.com/KyberNetwork/uniswapv3-sdk-uint256/constants"
	"github.com/KyberNetwork/uniswapv3-sdk-uint256/utils"
	"github.com/daoleno/uniswap-sdk-core/entities"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

var (
	B100e6         = uint256.NewInt(100e6)
	B100e12        = uint256.NewInt(100e12)
	B100e18        = decimal.NewFromBigInt(big.NewInt(100), 18).BigInt()
	B100e18Uint256 = uint256.MustFromBig(B100e18)
)

func initPool() (*Pool, int, int) {
	USDC := entities.NewToken(1, common.HexToAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"), 6, "USDC", "USD Coin")
	DAI := entities.NewToken(1, common.HexToAddress("0x6B175474E89094C44Da98b954EedeAC495271d0F"), 18, "DAI", "DAI Stablecoin")
	poolSqrtRatioStart := utils.EncodeSqrtRatioX96(B100e6, B100e18Uint256)
	poolTickCurrent, err := utils.GetTickAtSqrtRatio(poolSqrtRatioStart.ToBig())
	if err != nil {
		panic(err)
	}

	tickSpacing := constants.TickSpacings[constants.FeeLow]

	// p, err := NewPool(DAI, USDC, constants.FeeLow, poolSqrtRatioStart.ToBig(), big.NewInt(0), poolTickCurrent, nil)
	// if err != nil {
	// 	panic(err)
	// }

	pool := NewPoolV3(uint16(constants.FeeLow), int32(poolTickCurrent), poolSqrtRatioStart, DAI, USDC, nil)
	// pool.Liquidity = uint256.NewInt(0)

	return pool, poolTickCurrent, tickSpacing
}

func TestPosition(t *testing.T) {
	DAIUSDCPool, _, tickSpacing := initPool()

	// can be constructed around 0 tick
	p, err := NewPosition(DAIUSDCPool, uint256.NewInt(1), -10, 10)
	assert.NoError(t, err)
	assert.Equal(t, uint256.NewInt(1), p.Liquidity)

	// can use min and max ticks
	p, err = NewPosition(DAIUSDCPool, uint256.NewInt(1), NearestUsableTick(utils.MinTick, tickSpacing), 10)
	assert.NoError(t, err)
	assert.Equal(t, uint256.NewInt(1), p.Liquidity)

	// tick lower must be less than tick upper
	_, err = NewPosition(DAIUSDCPool, uint256.NewInt(1), 10, -10)
	assert.ErrorIs(t, err, ErrTickOrder)

	// tick lower cannot equal tick upper
	_, err = NewPosition(DAIUSDCPool, uint256.NewInt(1), -10, -10)
	assert.ErrorIs(t, err, ErrTickOrder)

	// tick lower must be multiple of tick spacing
	_, err = NewPosition(DAIUSDCPool, uint256.NewInt(1), -5, 10)
	assert.ErrorIs(t, err, ErrTickLower)

	// tick lower must be greater than MIN_TICK
	_, err = NewPosition(DAIUSDCPool, uint256.NewInt(1), NearestUsableTick(utils.MinTick, tickSpacing)-tickSpacing, 10)
	assert.ErrorIs(t, err, ErrTickLower)

	// tick upper must be multiple of tick spacing
	_, err = NewPosition(DAIUSDCPool, uint256.NewInt(1), -10, 15)
	assert.ErrorIs(t, err, ErrTickUpper)

	// tick upper must be less than MAX_TICK
	_, err = NewPosition(DAIUSDCPool, uint256.NewInt(1), -10, NearestUsableTick(utils.MaxTick, tickSpacing)+tickSpacing)
	assert.ErrorIs(t, err, ErrTickUpper)
}

func TestAmount0(t *testing.T) {
	DAIUSDCPool, poolTickCurrent, tickSpacing := initPool()

	// is correct for price above
	p, err := NewPosition(DAIUSDCPool, B100e12, NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing, NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing*2)
	assert.NoError(t, err)
	amount0 := p.CalcAmount0()
	// assert.NoError(t, err)
	assert.Equal(t, "49949961958869841", amount0.Dec())

	// is correct for price below
	p, err = NewPosition(DAIUSDCPool, B100e18Uint256, NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing*2, NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing)
	assert.NoError(t, err)
	amount0 = p.CalcAmount0()
	// assert.NoError(t, err)
	assert.Equal(t, "0", amount0.Dec())

	// is correct for in-range position
	p, err = NewPosition(DAIUSDCPool, B100e18Uint256, NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing*2, NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing*2)
	assert.NoError(t, err)
	amount0 = p.CalcAmount0()
	// assert.NoError(t, err)
	// assert.Equal(t, "120054069145287995769397", amount0.Dec())
	assert.Equal(t, "120054069145287995769396", amount0.Dec())
	// ! 120054069145287995769396 in v3-sdk(typescript)
}

func TestAmount1(t *testing.T) {
	DAIUSDCPool, poolTickCurrent, tickSpacing := initPool()

	// is correct for price above
	p, err := NewPosition(DAIUSDCPool, B100e18Uint256, NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing, NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing*2)
	assert.NoError(t, err)
	amount1 := p.CalcAmount1()
	// assert.NoError(t, err)
	assert.Equal(t, "0", amount1.Dec())

	// is correct for price below
	p, err = NewPosition(DAIUSDCPool, B100e18Uint256, NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing*2, NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing)
	assert.NoError(t, err)
	amount1 = p.CalcAmount1()
	// assert.NoError(t, err)
	assert.Equal(t, "49970077052", amount1.Dec())

	// is correct for in-range position
	p, err = NewPosition(DAIUSDCPool, B100e18Uint256, NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing*2, NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing*2)
	assert.NoError(t, err)
	amount1 = p.CalcAmount1()
	//assert.NoError(t, err)
	assert.Equal(t, "79831926242", amount1.Dec())
}

func TestMintAmountsWithSlippage(t *testing.T) {
	DAIUSDCPool, poolTickCurrent, tickSpacing := initPool()
	// 0 slippage
	slippageTolerance := entities.NewPercent(big.NewInt(0), big.NewInt(1))

	// is correct for positions below
	p, err := NewPosition(DAIUSDCPool, B100e18Uint256,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing*2)
	assert.NoError(t, err)
	amount0, amount1, err := p.MintAmountsWithSlippage(slippageTolerance)
	assert.NoError(t, err)
	assert.Equal(t, "49949961958869841738198", amount0.Dec())
	assert.Equal(t, "0", amount1.Dec())

	// is correct for positions above
	p, err = NewPosition(DAIUSDCPool, B100e18Uint256,
		NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing*2,
		NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing)
	assert.NoError(t, err)
	amount0, amount1, err = p.MintAmountsWithSlippage(slippageTolerance)
	assert.NoError(t, err)
	assert.Equal(t, "0", amount0.Dec())
	assert.Equal(t, "49970077053", amount1.Dec())

	// is correct for positions within
	p, err = NewPosition(DAIUSDCPool, B100e18Uint256,
		NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing*2,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing*2)
	assert.NoError(t, err)
	amount0, amount1, err = p.MintAmountsWithSlippage(slippageTolerance)
	assert.NoError(t, err)
	assert.Equal(t, "120054069145287995740584", amount0.Dec())
	assert.Equal(t, "79831926243", amount1.Dec())

	// 0.05% slippage
	slippageTolerance = entities.NewPercent(big.NewInt(5), big.NewInt(10000))

	// is correct for positions below
	p, err = NewPosition(DAIUSDCPool, B100e18Uint256,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing*2)
	assert.NoError(t, err)
	amount0, amount1, err = p.MintAmountsWithSlippage(slippageTolerance)
	assert.NoError(t, err)
	assert.Equal(t, "49949961958869841738198", amount0.Dec())
	assert.Equal(t, "0", amount1.Dec())

	// is correct for positions above
	p, err = NewPosition(DAIUSDCPool, B100e18Uint256,
		NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing*2,
		NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing)
	assert.NoError(t, err)
	amount0, amount1, err = p.MintAmountsWithSlippage(slippageTolerance)
	assert.NoError(t, err)
	assert.Equal(t, "0", amount0.Dec())
	assert.Equal(t, "49970077053", amount1.Dec())

	// is correct for positions within
	p, err = NewPosition(DAIUSDCPool, B100e18Uint256,
		NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing*2,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing*2)
	assert.NoError(t, err)
	amount0, amount1, err = p.MintAmountsWithSlippage(slippageTolerance)
	assert.NoError(t, err)
	assert.Equal(t, "95063440240746211432007", amount0.Dec())
	assert.Equal(t, "54828800461", amount1.Dec())

	// 5% slippage tolerance
	slippageTolerance = entities.NewPercent(big.NewInt(5), big.NewInt(100))

	// is correct for pool at min price
	minPricePool := NewPoolV3(uint16(constants.FeeLow), int32(utils.MinTick), utils.MinSqrtRatioU256, DAI, USDC, nil)
	minPricePool.Liquidity = uint256.MustFromBig(big.NewInt(1))

	assert.NoError(t, err)
	p, err = NewPosition(minPricePool, B100e18Uint256,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing*2)
	assert.NoError(t, err)
	amount0, amount1, err = p.BurnAmountsWithSlippage(slippageTolerance)
	assert.NoError(t, err)
	assert.Equal(t, "49949961958869841754181", amount0.Dec())
	assert.Equal(t, "0", amount1.Dec())

	// is correct for pool at max price
	// maxPricePool, err := NewPool(DAI, USDC, constants.FeeLow, new(big.Int).Sub(utils.MaxSqrtRatio, big.NewInt(1)), big.NewInt(0), utils.MaxTick-1, nil)
	maxPricePool := NewPoolV3(uint16(constants.FeeLow), int32(utils.MaxTick-1), new(uint256.Int).Sub(utils.MaxSqrtRatioU256, uint256.NewInt(1)), DAI, USDC, nil)
	maxPricePool.Liquidity = uint256.MustFromBig(big.NewInt(1))

	assert.NoError(t, err)
	p, err = NewPosition(maxPricePool, B100e18Uint256,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing*2)
	assert.NoError(t, err)
	amount0, amount1, err = p.BurnAmountsWithSlippage(slippageTolerance)
	assert.NoError(t, err)
	assert.Equal(t, "0", amount0.Dec())
	assert.Equal(t, "50045084659", amount1.Dec())
}

func TestBurnAmountsWithSlippage(t *testing.T) {
	DAIUSDCPool, poolTickCurrent, tickSpacing := initPool()

	// 0 slippage tolerance
	slippageTolerance := entities.NewPercent(big.NewInt(0), big.NewInt(100))

	// is correct for positions below
	p, err := NewPosition(DAIUSDCPool, B100e18Uint256,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing*2)
	assert.NoError(t, err)
	amount0, amount1, err := p.BurnAmountsWithSlippage(slippageTolerance)
	assert.NoError(t, err)
	assert.Equal(t, "49949961958869841754181", amount0.Dec())
	assert.Equal(t, "0", amount1.Dec())

	// is correct for positions above
	p, err = NewPosition(DAIUSDCPool, B100e18Uint256,
		NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing*2,
		NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing)
	assert.NoError(t, err)
	amount0, amount1, err = p.BurnAmountsWithSlippage(slippageTolerance)
	assert.NoError(t, err)
	assert.Equal(t, "0", amount0.Dec())
	assert.Equal(t, "49970077052", amount1.Dec())

	// is correct for positions within
	p, err = NewPosition(DAIUSDCPool, B100e18Uint256,
		NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing*2,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing*2)
	assert.NoError(t, err)
	amount0, amount1, err = p.BurnAmountsWithSlippage(slippageTolerance)
	assert.NoError(t, err)
	//! 120054069145287995769396 in v3-sdk
	assert.Equal(t, "120054069145287995769397", amount0.Dec())
	assert.Equal(t, "79831926242", amount1.Dec())

	// 0.05% slippage
	slippageTolerance = entities.NewPercent(big.NewInt(5), big.NewInt(10000))

	// is correct for positions below
	p, err = NewPosition(DAIUSDCPool, B100e18Uint256,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing*2)
	assert.NoError(t, err)
	amount0, amount1, err = p.BurnAmountsWithSlippage(slippageTolerance)
	assert.NoError(t, err)
	assert.Equal(t, "49949961958869841754181", amount0.Dec())
	assert.Equal(t, "0", amount1.Dec())

	// is correct for positions above
	p, err = NewPosition(DAIUSDCPool, B100e18Uint256,
		NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing*2,
		NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing)
	assert.NoError(t, err)
	amount0, amount1, err = p.BurnAmountsWithSlippage(slippageTolerance)
	assert.NoError(t, err)
	assert.Equal(t, "0", amount0.Dec())
	assert.Equal(t, "49970077052", amount1.Dec())

	// is correct for positions within
	p, err = NewPosition(DAIUSDCPool, B100e18Uint256,
		NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing*2,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing*2)
	assert.NoError(t, err)
	amount0, amount1, err = p.BurnAmountsWithSlippage(slippageTolerance)
	assert.NoError(t, err)
	// ! 95063440240746211454822 in v3-sdk
	assert.Equal(t, "95063440240746211454823", amount0.Dec())
	assert.Equal(t, "54828800460", amount1.Dec())

	// 5% slippage tolerance
	slippageTolerance = entities.NewPercent(big.NewInt(5), big.NewInt(100))

	// is correct for pool at min price
	// minPricePool, err := NewPool(DAI, USDC, constants.FeeLow, utils.MinSqrtRatio, big.NewInt(0), utils.MinTick, nil)
	minPricePool := NewPoolV3(uint16(constants.FeeLow), int32(utils.MinTick), utils.MinSqrtRatioU256, DAI, USDC, nil)

	assert.NoError(t, err)
	p, err = NewPosition(minPricePool, B100e18Uint256,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing*2)
	assert.NoError(t, err)
	amount0, amount1, err = p.BurnAmountsWithSlippage(slippageTolerance)
	assert.NoError(t, err)
	// ! 49949961958869841738198 in v3-sdk
	assert.Equal(t, "49949961958869841754181", amount0.Dec())
	assert.Equal(t, "0", amount1.Dec())

	// is correct for pool at max price
	// maxPricePool, err := NewPool(DAI, USDC, constants.FeeLow, new(big.Int).Sub(utils.MaxSqrtRatio, big.NewInt(1)), big.NewInt(0), utils.MaxTick-1, nil)
	maxPricePool := NewPoolV3(uint16(constants.FeeLow), int32(utils.MaxTick-1), new(uint256.Int).Sub(utils.MaxSqrtRatioU256, uint256.NewInt(1)), DAI, USDC, nil)
	maxPricePool.Liquidity = uint256.MustFromBig(big.NewInt(1))

	assert.NoError(t, err)
	p, err = NewPosition(maxPricePool, B100e18Uint256,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing*2)
	assert.NoError(t, err)
	amount0, amount1, err = p.BurnAmountsWithSlippage(slippageTolerance)
	assert.NoError(t, err)
	assert.Equal(t, "0", amount0.Dec())
	// ! 50045084660 in v3-sdk
	assert.Equal(t, "50045084659", amount1.Dec())
}

func TestMintAmounts(t *testing.T) {
	DAIUSDCPool, poolTickCurrent, tickSpacing := initPool()

	// is correct for price above
	p, err := NewPosition(DAIUSDCPool, B100e18Uint256,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing*2)
	assert.NoError(t, err)
	amount0, amount1, err := p.MintAmounts()
	assert.NoError(t, err)
	assert.Equal(t, "49949961958869841754182", amount0.Dec())
	assert.Equal(t, "0", amount1.Dec())

	// is correct for price below
	p, err = NewPosition(DAIUSDCPool, B100e18Uint256,
		NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing*2,
		NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing)
	assert.NoError(t, err)
	amount0, amount1, err = p.MintAmounts()
	assert.NoError(t, err)
	assert.Equal(t, "0", amount0.Dec())
	assert.Equal(t, "49970077053", amount1.Dec())

	// is correct for in-range position
	p, err = NewPosition(DAIUSDCPool, B100e18Uint256,
		NearestUsableTick(poolTickCurrent, tickSpacing)-tickSpacing*2,
		NearestUsableTick(poolTickCurrent, tickSpacing)+tickSpacing*2)
	assert.NoError(t, err)
	amount0, amount1, err = p.MintAmounts()
	assert.NoError(t, err)
	// note these are rounded up
	assert.Equal(t, "120054069145287995769397", amount0.Dec())
	assert.Equal(t, "79831926243", amount1.Dec())
}
