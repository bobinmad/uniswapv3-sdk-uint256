package entities

import (
	"slices"

	"github.com/bobinmad/uniswapv3-sdk-uint256/utils"
	"github.com/holiman/uint256"
	"github.com/vuquang23/int256"
)

type Tick struct {
	Index          int32
	LiquidityGross *uint256.Int
	LiquidityNet   *utils.Int128
}

// наш собственный расширенный TickListDataProvider с блэкджеком и шлюхами
type TicksHandler struct {
	Ticks           []Tick
	TicksLen        int
	SmallestTickIdx int32
	LargestTickIdx  int32

	// lastResultIdx кэширует slice-индекс последнего тика из NextInitializedTickIndex.
	// Используется двояко:
	//   1. GetTick: если Ticks[lastResultIdx].Index совпадает — binarySearch не нужен.
	//   2. NextInitializedTickIndex: sequential hint — внутри одного свапа тики
	//      пересекаются по одному (±1 шаг), поэтому проверяем соседний элемент
	//      перед запуском полного binary search (O(1) vs O(log N)).
	// Хранится как int (не pointer) — нет GC write barrier при каждом присваивании.
	// -1 означает «кэш невалиден» (при инициализации или удалении самого hint tick).
	lastResultIdx int

	// Direct-mapped cache для exact tick lookup исторических Mint/Burn.
	// 16 ячеек покрывают несколько одновременно живущих пар границ, а hash lookup
	// не сканирует последовательно все slots при промахе. При insert/delete
	// закэшированные slice-индексы автоматически сдвигаются.
	cacheTicks     [16]int32
	cacheIdx       [16]int32
	cacheValidMask uint16
}

func NewTicksHandler() *TicksHandler {
	return &TicksHandler{}
}

// клонирует текущий тикхандлер путём создания глубокой копии.
// используется для создания нового тикхандлера каждой стратегии в многопотоке.
func (h *TicksHandler) Clone() *TicksHandler {
	ticksHandler := NewTicksHandler()

	ticksHandler.CloneTicks(h.Ticks)
	return ticksHandler
}

func (h *TicksHandler) SetTicks(ticks []Tick) {
	h.TicksLen = len(ticks)
	h.Ticks = make([]Tick, h.TicksLen)

	for idx, tick := range ticks {
		h.Ticks[idx] = Tick{
			Index:          tick.Index,
			LiquidityGross: tick.LiquidityGross,
			LiquidityNet:   tick.LiquidityNet,
		}
	}

	h.SmallestTickIdx = h.Ticks[0].Index
	h.LargestTickIdx = h.Ticks[h.TicksLen-1].Index
	h.lastResultIdx = -1
	h.cacheValidMask = 0
}

func (h *TicksHandler) CloneTicks(ticks []Tick) {
	h.TicksLen = len(ticks)
	h.Ticks = make([]Tick, h.TicksLen)

	for idx, tick := range ticks {
		h.Ticks[idx] = Tick{
			Index:          tick.Index,
			LiquidityGross: tick.LiquidityGross.Clone(),
			LiquidityNet:   tick.LiquidityNet.Clone(),
		}
	}

	h.SmallestTickIdx = h.Ticks[0].Index
	h.LargestTickIdx = h.Ticks[h.TicksLen-1].Index
	h.lastResultIdx = -1
	h.cacheValidMask = 0
}

func (h *TicksHandler) GetTick(tick int32) (Tick, error) {
	// Быстрый путь: если последний вызов NextInitializedTickIndex вернул именно этот тик,
	// возвращаем его без binary search. lastResultIdx — int, нет GC write barrier.
	if i := h.lastResultIdx; i >= 0 && h.Ticks[i].Index == tick {
		return h.Ticks[i], nil
	}
	i := h.binarySearch(tick)
	if h.Ticks[i].Index == tick {
		return h.Ticks[i], nil
	}
	return EmptyTick, ErrTickNotFound
}

func (h *TicksHandler) NextInitializedTickIndex(tick int32, lte bool) (int32, bool, error) {
	var i int

	if lte {
		if h.isBelowSmallest(tick) {
			return ZeroValueTickIndex, false, ErrBelowSmallest
		}
		if h.isAtOrAboveLargest(tick) {
			i = h.TicksLen - 1
		} else {
			// Sequential hint: внутри одного свапа тики пересекаются по одному (lte=true → индекс -=1).
			// Проверяем Ticks[last-1] перед полным binary search — O(1) вместо O(log N).
			hint := h.lastResultIdx - 1
			if hint >= 0 && h.Ticks[hint].Index <= tick && h.Ticks[h.lastResultIdx].Index > tick {
				i = hint
			} else {
				i = h.binarySearch(tick)
			}
		}
	} else {
		if h.isAtOrAboveLargest(tick) {
			return ZeroValueTickIndex, false, ErrAtOrAboveLargest
		}
		if h.isBelowSmallest(tick) {
			i = 0
		} else {
			// Sequential hint для lte=false: тики пересекаются по одному (индекс +=1).
			hint := h.lastResultIdx + 1
			if h.lastResultIdx >= 0 && hint < h.TicksLen &&
				h.Ticks[h.lastResultIdx].Index <= tick && h.Ticks[hint].Index > tick {
				i = hint
			} else {
				i = h.binarySearch(tick) + 1
			}
		}
	}

	// Кэшируем slice-индекс: используется в GetTick и как hint для следующего вызова.
	// int, не pointer — нет GC write barrier.
	h.lastResultIdx = i
	t := &h.Ticks[i]
	return t.Index, !t.LiquidityGross.IsZero(), nil
}

// NextInitializedTickWithinOneWord exactly mirrors TickBitmap.nextInitializedTickWithinOneWord.
// Even when a bitmap word contains no initialized ticks, Uniswap V3 Core stops at
// that word's boundary and performs a separate swap step. Skipping directly to a
// distant initialized tick changes per-step rounding of amountIn, amountOut and fee.
func (h *TicksHandler) NextInitializedTickWithinOneWord(tick int32, lte bool, tickSpacing uint16) (int32, bool, error) {
	if tickSpacing == 0 {
		return 0, false, utils.ErrInvariant
	}

	spacing := int32(tickSpacing)
	compressed := tick / spacing
	// Solidity's signed division truncates toward zero, followed by an explicit
	// decrement for negative non-multiples. This is floor(tick / tickSpacing).
	if tick < 0 && tick%spacing != 0 {
		compressed--
	}

	if lte {
		wordPos := compressed >> 8
		minimum := (wordPos << 8) * spacing
		if h.TicksLen == 0 || tick < h.SmallestTickIdx {
			return minimum, false, nil
		}

		i := h.binarySearch(tick)
		for i >= 0 && h.Ticks[i].Index >= minimum {
			candidate := &h.Ticks[i]
			if !candidate.LiquidityGross.IsZero() {
				h.lastResultIdx = i
				return candidate.Index, true, nil
			}
			i--
		}
		return minimum, false, nil
	}

	wordPos := (compressed + 1) >> 8
	// Core operates on compressed ticks and then multiplies the selected bit
	// index by tickSpacing. Therefore the empty-word boundary is the last
	// aligned compressed tick in the word, not the uncompressed tick just
	// below the next word (the latter is a common SDK approximation).
	maximum := (((wordPos + 1) << 8) - 1) * spacing
	if h.TicksLen == 0 || tick >= h.LargestTickIdx {
		return maximum, false, nil
	}

	i := 0
	if tick >= h.SmallestTickIdx {
		i = h.binarySearch(tick) + 1
	}
	for i < h.TicksLen && h.Ticks[i].Index <= maximum {
		candidate := &h.Ticks[i]
		if !candidate.LiquidityGross.IsZero() {
			h.lastResultIdx = i
			return candidate.Index, true, nil
		}
		i++
	}
	return maximum, false, nil
}

// актуализирует состояние тиков пула после историчекого события mint
func (h *TicksHandler) UpdateTicksAfterMint(tickLower, tickUpper int32, liquidity *uint256.Int) {
	liquidityI256 := (*int256.Int)(liquidity)

	tick, lowerSliceKey, lowerExists := h.tickWithSliceKey(tickLower)
	if lowerExists {
		tick.LiquidityGross.Add(tick.LiquidityGross, liquidity)
		tick.LiquidityNet.Add(tick.LiquidityNet, liquidityI256)
	} else {
		h.Ticks = slices.Insert(h.Ticks, lowerSliceKey, Tick{Index: tickLower, LiquidityGross: liquidity.Clone(), LiquidityNet: liquidityI256.Clone()})
		h.TicksLen++
		h.shiftIndicesAfterInsert(int32(lowerSliceKey))
		if tickLower < h.SmallestTickIdx {
			h.SmallestTickIdx = tickLower
		}
	}

	// tickUpper > tickLower, поэтому после lower lookup/insert верхнюю границу
	// можно искать только правее lowerSliceKey.
	if tick, sliceKey, exist := h.tickWithSliceKeyFrom(tickUpper, lowerSliceKey+1); exist {
		tick.LiquidityGross.Add(tick.LiquidityGross, liquidity)
		tick.LiquidityNet.Sub(tick.LiquidityNet, liquidityI256)
	} else {
		h.Ticks = slices.Insert(h.Ticks, sliceKey, Tick{Index: tickUpper, LiquidityGross: liquidity.Clone(), LiquidityNet: new(int256.Int).Neg(liquidityI256)})
		h.TicksLen++
		h.shiftIndicesAfterInsert(int32(sliceKey))
		if tickUpper > h.LargestTickIdx {
			h.LargestTickIdx = tickUpper
		}
	}
}

// актуализирует состояние тиков пула после историчекого события burn
func (h *TicksHandler) UpdateTicksAfterBurn(tickLower, tickUpper int32, liquidity *uint256.Int) {
	liquidityI256 := (*int256.Int)(liquidity)

	tick, sliceKey, _ := h.tickWithSliceKey(tickLower)
	tick.LiquidityGross.Sub(tick.LiquidityGross, liquidity)
	tick.LiquidityNet.Sub(tick.LiquidityNet, liquidityI256)
	h.removeTickIfEmpty(tick, sliceKey)

	// Если lower был удалён, upper сдвинулся в lower sliceKey; если остался —
	// включение одного заведомо меньшего элемента стоит дешевле полного поиска.
	tick, sliceKey, _ = h.tickWithSliceKeyFrom(tickUpper, sliceKey)
	tick.LiquidityGross.Sub(tick.LiquidityGross, liquidity)
	tick.LiquidityNet.Add(tick.LiquidityNet, liquidityI256)
	h.removeTickIfEmpty(tick, sliceKey)
}

// проверяет тик на пустую ликвидность и удаляет в таком случае
func (h *TicksHandler) removeTickIfEmpty(tick *Tick, sliceKey int) {
	if tick.LiquidityGross.IsZero() && tick.LiquidityNet.IsZero() {
		h.Ticks = slices.Delete(h.Ticks, sliceKey, sliceKey+1)
		h.TicksLen--
		h.shiftIndicesAfterDelete(int32(sliceKey))

		if h.TicksLen > 0 {
			h.SmallestTickIdx = h.Ticks[0].Index
			h.LargestTickIdx = h.Ticks[h.TicksLen-1].Index
		}
	}
}

// shiftIndicesAfterInsert корректирует кэшированные индексы после slices.Insert(at).
// Все индексы >= at сдвигаются на +1; sequential-hint lastResultIdx тоже обновляется.
//
//go:nosplit
func (h *TicksHandler) shiftIndicesAfterInsert(at int32) {
	mask := h.cacheValidMask
	for i := uint16(0); i < 16; i++ {
		if mask&(1<<i) != 0 && h.cacheIdx[i] >= at {
			h.cacheIdx[i]++
		}
	}
	if h.lastResultIdx >= int(at) {
		h.lastResultIdx++
	}
}

// shiftIndicesAfterDelete корректирует кэшированные индексы после slices.Delete(at).
// Удалённая ячейка инвалидируется; индексы > at сдвигаются на -1.
//
//go:nosplit
func (h *TicksHandler) shiftIndicesAfterDelete(at int32) {
	mask := h.cacheValidMask
	for i := uint16(0); i < 16; i++ {
		if mask&(1<<i) != 0 {
			if h.cacheIdx[i] == at {
				mask &^= 1 << i // удалённая ячейка — инвалидируем
			} else if h.cacheIdx[i] > at {
				h.cacheIdx[i]--
			}
		}
	}
	h.cacheValidMask = mask
	if h.lastResultIdx == int(at) {
		h.lastResultIdx = -1
	} else if h.lastResultIdx > int(at) {
		h.lastResultIdx--
	}
}

// tickWithSliceKey возвращает указатель на тик и индекс при найденном, иначе (nil, insertionIdx, false).
//
// Direct-mapped cache для типичного rebalance-паттерна Mint(L,U) → Burn(L,U).
// Sequential-hint lastResultIdx отдельно корректируется только при фактическом
// insert/delete; обычный lookup его не сбрасывает.
//
//go:nosplit
func (h *TicksHandler) tickWithSliceKey(tick int32) (*Tick, int, bool) {
	return h.tickWithSliceKeyFrom(tick, 0)
}

// tickWithSliceKeyFrom эквивалентен tickWithSliceKey, но caller может передать
// доказанную нижнюю границу slice-индекса. Используется для tickUpper после
// уже найденного tickLower.
//
//go:nosplit
func (h *TicksHandler) tickWithSliceKeyFrom(tick int32, minIdx int) (*Tick, int, bool) {
	if h.TicksLen == 0 {
		return nil, 0, false
	}

	// Multiplicative hashing использует старшие биты произведения: это важно,
	// поскольку реальные tick values обычно кратны tickSpacing.
	slot := uint16((uint32(tick) * 0x9e3779b1) >> 28)
	slotMask := uint16(1) << slot
	if h.cacheValidMask&slotMask != 0 && h.cacheTicks[slot] == tick {
		idx := int(h.cacheIdx[slot])
		if idx >= minIdx && idx < h.TicksLen && h.Ticks[idx].Index == tick {
			return &h.Ticks[idx], idx, true
		}
		h.cacheValidMask &^= slotMask
	}

	// ВАЖНО: lastResultIdx больше не сбрасываем здесь.
	// shiftIndicesAfterInsert/Delete сами корректно поддерживают индекс при insert/delete.
	// Если Mint попал в exist'ующий тик (без insert) — sequential hint следующего swap'а валиден.
	if minIdx < 0 {
		minIdx = 0
	} else if minIdx > h.TicksLen {
		minIdx = h.TicksLen
	}
	lo, hi := minIdx, h.TicksLen
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if h.Ticks[mid].Index <= tick {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	idx := lo - 1
	if idx >= minIdx && h.Ticks[idx].Index == tick {
		h.cacheTicks[slot] = tick
		h.cacheIdx[slot] = int32(idx)
		h.cacheValidMask |= slotMask
		return &h.Ticks[idx], idx, true
	}
	return nil, lo, false
}

// binarySearch возвращает наибольший индекс i, при котором Ticks[i].Index <= tick.
// Если все Ticks[i].Index > tick, возвращает 0 (для совместимости со старой семантикой).
//
// Обычный lo/hi upper_bound оказался быстрее step-based варианта на профиле
// исторических Mint/Burn: меньше зависимых операций в теле цикла.
//
//go:nosplit
func (h *TicksHandler) binarySearch(tick int32) int {
	ticks := h.Ticks
	lo, hi := 0, h.TicksLen
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if ticks[mid].Index <= tick {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo == 0 {
		return 0
	}
	return lo - 1
}

func (h *TicksHandler) isBelowSmallest(tick int32) bool {
	return tick < h.SmallestTickIdx
}

func (h *TicksHandler) isAtOrAboveLargest(tick int32) bool {
	return tick >= h.LargestTickIdx
}
