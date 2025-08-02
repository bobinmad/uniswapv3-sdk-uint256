package entities

import (
	"slices"
	"sort"

	"github.com/KyberNetwork/int256"
	"github.com/KyberNetwork/uniswapv3-sdk-uint256/utils"
	"github.com/holiman/uint256"
)

type Tick struct {
	Index          int32
	LiquidityGross *uint256.Int
	LiquidityNet   *utils.Int128
}

// // Provides information about ticks
// type TickDataProvider interface {
// 	/**
// 	 * Return information corresponding to a specific tick
// 	 * @param tick the tick to load
// 	 */
// 	GetTick(tick int) (Tick, error)

// 	/**
// 	 * Return the next tick that is initialized within a single word
// 	 * @param tick The current tick
// 	 * @param lte Whether the next tick should be lte the current tick
// 	 * @param tickSpacing The tick spacing of the pool
// 	 */
// 	// NextInitializedTickWithinOneWord(tick int, lte bool, tickSpacing int) (int, bool, error)

// 	// NextInitializedTickIndex return the next tick that is initialized
// 	NextInitializedTickIndex(tick int, lte bool) (int, bool, error)
// }

// наш собственный расширенный TickListDataProvider с блэкджеком и шлюхами
type TicksHandler struct {
	Ticks []Tick

	// performance sake
	TicksLen        int
	SmallestTickIdx int32
	LargestTickIdx  int32
}

func NewTicksHandler() *TicksHandler {
	return &TicksHandler{}
}

// клонирует текущий тикхандлер путём создания глубокой копии.
// используется для создания нового тикхандлера каждой стратегии в многопотоке.
func (h *TicksHandler) Clone() *TicksHandler {
	ticksHandler := NewTicksHandler()

	ticksHandler.SetTicks(h.Ticks)
	return ticksHandler
}

func (h *TicksHandler) SetTicks(ticks []Tick) {
	h.TicksLen = len(ticks)
	h.Ticks = make([]Tick, h.TicksLen)

	// a kinda deep copy of ticks
	for idx, tick := range ticks {
		h.Ticks[idx] = Tick{
			Index:          tick.Index,
			LiquidityGross: tick.LiquidityGross.Clone(),
			LiquidityNet:   tick.LiquidityNet.Clone(),
		}
	}

	h.SmallestTickIdx = h.Ticks[0].Index
	h.LargestTickIdx = h.Ticks[h.TicksLen-1].Index
}

// exports ticks as csv string, debug purpose
// func (h *TicksHandler) ExportTicksAsCSV() string {
// 	var csvString string
// 	for _, tick := range h.Ticks {
// 		csvString += fmt.Sprintf("%d;%s;%s\n", tick.Index, tick.LiquidityGross.Dec(), tick.LiquidityNet.Dec())
// 	}
// 	return csvString
// }

func (h *TicksHandler) GetTick(tick int32) (Tick, error) {
	// tickIndex, _ := h.binarySearchSimple(tick)
	// return h.Ticks[tickIndex], nil

	ft := h.Ticks[sort.Search(h.TicksLen, func(i int) bool { return h.Ticks[i].Index >= tick })]
	if ft.Index == tick {
		return ft, nil
	}
	return ft, ErrTickNotFound
}

// по факту метод не используется
func (h *TicksHandler) NextInitializedTickWithinOneWord(tick int32, lte bool, tickSpacing int) (int32, bool, error) {
	return NextInitializedTickWithinOneWord(h.Ticks, tick, lte, tickSpacing)
}

func (h *TicksHandler) NextInitializedTickIndex(tick int32, lte bool) (int32, bool, error) {
	var initializedTick Tick

	if lte {
		if h.isBelowSmallest(tick) {
			return ZeroValueTickIndex, false, nil
		}

		if h.isAtOrAboveLargest(tick) {
			initializedTick = h.Ticks[h.TicksLen-1]
		} else {
			initializedTick = h.Ticks[h.binarySearch(tick)]
		}
	} else {
		if h.isAtOrAboveLargest(tick) {
			return ZeroValueTickIndex, false, nil
		}

		if h.isBelowSmallest(tick) {
			initializedTick = h.Ticks[0]
		} else {
			initializedTick = h.Ticks[h.binarySearch(tick)+1]
		}
	}

	return initializedTick.Index, !initializedTick.LiquidityGross.IsZero(), nil

	// nextInitializedTick, err := v3entities.NextInitializedTick(h.Ticks, tick, lte)
	// if err != nil {
	// 	return v3entities.ZeroValueTickIndex, v3entities.ZeroValueTickInitialized, err
	// }

	// return nextInitializedTick.Index, !nextInitializedTick.LiquidityGross.IsZero(), nil
}

// func (h *TicksHandler) nextInitializedTick(tick int, lte bool) v3entities.Tick {
// 	if lte {
// 		if h.isAtOrAboveLargest(tick) {
// 			return h.Ticks[h.ticksLen-1]
// 		}

// 		return h.Ticks[h.binarySearch(tick)]
// 	} else {
// 		if h.isBelowSmallest(tick) {
// 			return h.Ticks[0]
// 		}

// 		return h.Ticks[h.binarySearch(tick)+1]
// 	}
// }

// актуализирует состояние тиков пула после историчекого события mint
func (h *TicksHandler) UpdateTicksAfterMint(tickLower, tickUpper int32, liquidity *uint256.Int) {
	if key, exist := h.binarySearchSimple(tickLower); exist {
		tick := h.Ticks[key]
		tick.LiquidityGross.Add(tick.LiquidityGross, liquidity)
		tick.LiquidityNet.Add(tick.LiquidityNet, (*int256.Int)(liquidity))
	} else {
		h.Ticks = slices.Insert(h.Ticks, key, Tick{Index: tickLower, LiquidityGross: liquidity.Clone(), LiquidityNet: (*int256.Int)(liquidity).Clone()})
		h.TicksLen++

		if tickLower < h.SmallestTickIdx {
			h.SmallestTickIdx = tickLower
		}
	}

	if key, exist := h.binarySearchSimple(tickUpper); exist {
		tick := h.Ticks[key]
		tick.LiquidityGross.Add(tick.LiquidityGross, liquidity)
		tick.LiquidityNet.Sub(tick.LiquidityNet, (*int256.Int)(liquidity))
	} else {
		h.Ticks = slices.Insert(h.Ticks, key, Tick{Index: tickUpper, LiquidityGross: liquidity.Clone(), LiquidityNet: new(int256.Int).Neg((*int256.Int)(liquidity))})
		h.TicksLen++

		if tickUpper > h.LargestTickIdx {
			h.LargestTickIdx = tickUpper
		}
	}
}

// актуализирует состояние тиков пула после историчекого события burn
func (h *TicksHandler) UpdateTicksAfterBurn(tickLower, tickUpper int32, liquidity *uint256.Int) {
	key, _ := h.binarySearchSimple(tickLower)
	tick := h.Ticks[key]
	tick.LiquidityGross.Sub(tick.LiquidityGross, liquidity)
	tick.LiquidityNet.Sub(tick.LiquidityNet, (*int256.Int)(liquidity))
	h.removeTickIfEmpty(tick, key)

	key, _ = h.binarySearchSimple(tickUpper)
	tick = h.Ticks[key]
	tick.LiquidityGross.Sub(tick.LiquidityGross, liquidity)
	tick.LiquidityNet.Add(tick.LiquidityNet, (*int256.Int)(liquidity))
	h.removeTickIfEmpty(tick, key)
}

// проверяет тик на пустую ликвидность и удаляет в таком случае
func (h *TicksHandler) removeTickIfEmpty(tick Tick, key int) {
	if tick.LiquidityGross.IsZero() && tick.LiquidityNet.IsZero() {
		h.Ticks = slices.Delete(h.Ticks, key, key+1)
		h.TicksLen--

		if h.TicksLen > 0 {
			h.SmallestTickIdx = h.Ticks[0].Index
			h.LargestTickIdx = h.Ticks[h.TicksLen-1].Index
		}
	}
}

func (h *TicksHandler) binarySearchSimple(tick int32) (int, bool) {
	idx := sort.Search(h.TicksLen, func(i int) bool {
		return h.Ticks[i].Index >= tick
	})
	if idx < h.TicksLen && h.Ticks[idx].Index == tick {
		return idx, true
	}
	return idx, false
}

func (h *TicksHandler) binarySearch(tick int32) int {
	start := 0
	end := h.TicksLen - 1

	for start < end {
		mid := (start + end) >> 1
		if h.Ticks[mid].Index <= tick {
			start = mid + 1
		} else {
			end = mid
		}
	}

	if start > 0 && h.Ticks[start-1].Index <= tick {
		return start - 1
	}
	return start
}

func (h *TicksHandler) isBelowSmallest(tick int32) bool {
	return tick < h.SmallestTickIdx
}

func (h *TicksHandler) isAtOrAboveLargest(tick int32) bool {
	return tick >= h.LargestTickIdx
}
