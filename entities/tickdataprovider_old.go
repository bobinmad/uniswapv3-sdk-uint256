package entities

// import (
// 	"irq/common/uniconst"
// 	"slices"

// 	"github.com/KyberNetwork/int256"
// 	kyberEntities "github.com/KyberNetwork/uniswapv3-sdk-uint256/entities"
// 	"github.com/holiman/uint256"
// )

// // наш собственный расширенный TickListDataProvider с блэкджеком и шлюхами
// type TicksHandler struct {
// 	Ticks               []kyberEntities.Tick
// 	TickSpacing         int
// 	InitializedTicksMap map[int32]int
// 	TicksLen            int
// }

// func NewTicksHandler() *TicksHandler {
// 	return &TicksHandler{
// 		InitializedTicksMap: make(map[int32]int),
// 	}
// }

// func (h *TicksHandler) GetTick(tick int32) (kyberEntities.Tick, error) {
// 	return kyberEntities.GetTick(h.Ticks, int(tick))
// }

// type Tick struct {
// 	Index          int32
// 	LiquidityGross *uint256.Int
// 	LiquidityNet   *int256.Int
// }

// func (h *TicksHandler) GetTicks() []Tick {
// 	ticks := make([]Tick, len(h.Ticks))
// 	for i, tick := range h.Ticks {
// 		ticks[i] = Tick{
// 			Index:          int32(tick.Index),
// 			LiquidityGross: tick.LiquidityGross,
// 			LiquidityNet:   tick.LiquidityNet,
// 		}
// 	}

// 	return ticks
// }

// func (h *TicksHandler) NextInitializedTickWithinOneWord(tick int32, lte bool, tickSpacing int) (int32, bool, error) {
// 	idx, initialized, err := kyberEntities.NextInitializedTickWithinOneWord(h.Ticks, int(tick), lte, tickSpacing)
// 	return int32(idx), initialized, err
// }

// func (h *TicksHandler) NextInitializedTickIndex(tick int32, lte bool) (int32, bool, error) {
// 	idx, initialized, err := kyberEntities.NextInitializedTickIndex(h.Ticks, int(tick), lte)
// 	return int32(idx), initialized, err
// }

// func (h *TicksHandler) Clone() *TicksHandler {
// 	return &TicksHandler{
// 		Ticks:               slices.Clone(h.Ticks),
// 		TickSpacing:         h.TickSpacing,
// 		InitializedTicksMap: h.InitializedTicksMap,
// 		TicksLen:            h.TicksLen,
// 	}
// }

// func (h *TicksHandler) SetTicks(ticks []Tick) {
// 	h.Ticks = make([]kyberEntities.Tick, len(ticks))
// 	for i, tick := range ticks {
// 		h.Ticks[i] = kyberEntities.Tick{
// 			Index:          int(tick.Index),
// 			LiquidityGross: tick.LiquidityGross,
// 			LiquidityNet:   tick.LiquidityNet,
// 		}
// 	}
// 	h.TicksLen = len(ticks)
// 	h.buildInitializedTicksMap()
// }

// // актуализирует состояние тиков пула после историчекого события mint
// func (h *TicksHandler) UpdateTicksAfterMint(tickLower, tickUpper int32, liquidity *uint256.Int) {
// 	liquidityInt256 := int256.MustFromBig(liquidity.ToBig())

// 	// if ltIndex, exist := h.InitializedTicksMap[tickLower]; exist {
// 	// 	h.Ticks[ltIndex].LiquidityGross = new(uint256.Int).Add(h.Ticks[ltIndex].LiquidityGross, liquidity)
// 	// 	h.Ticks[ltIndex].LiquidityNet = new(int256.Int).Add(h.Ticks[ltIndex].LiquidityNet, liquidityInt256)
// 	// } else {
// 	// 	var (
// 	// 		foundPosition     bool
// 	// 		prevIndex         int
// 	// 		lowerTickToAppend = v3Entities.Tick{Index: tickLower, LiquidityGross: liquidity, LiquidityNet: liquidityInt256}
// 	// 	)

// 	// 	for key, tick := range h.Ticks {
// 	// 		if tickLower > prevIndex && tickLower < tick.Index {
// 	// 			// нашли подходящий индекс для вставки нижнего тика в массив
// 	// 			h.Ticks = slices.Insert(h.Ticks, key, lowerTickToAppend)
// 	// 			foundPosition = true
// 	// 			break
// 	// 		}

// 	// 		prevIndex = tick.Index
// 	// 	}

// 	// 	if !foundPosition {
// 	// 		if len(h.Ticks) > 0 && tickLower < h.Ticks[0].Index {
// 	// 			// ниже нижнего
// 	// 			h.Ticks = slices.Insert(h.Ticks, 0, lowerTickToAppend)
// 	// 		} else {
// 	// 			// выше верхнего
// 	// 			h.Ticks = append(h.Ticks, lowerTickToAppend)
// 	// 		}
// 	// 	}
// 	// }

// 	// if utIndex, exist := h.InitializedTicksMap[tickUpper]; exist {
// 	// 	h.Ticks[utIndex].LiquidityGross = new(uint256.Int).Add(h.Ticks[utIndex].LiquidityGross, liquidity)
// 	// 	h.Ticks[utIndex].LiquidityNet = new(int256.Int).Sub(h.Ticks[utIndex].LiquidityNet, liquidityInt256)
// 	// } else {
// 	// 	var (
// 	// 		prevIndex         int
// 	// 		upperTickToAppend = v3Entities.Tick{Index: tickUpper, LiquidityGross: liquidity, LiquidityNet: new(int256.Int).Mul(liquidityInt256, int256.NewInt(-1))}
// 	// 	)

// 	// 	for key, tick := range h.Ticks {
// 	// 		if tickUpper > prevIndex && tickUpper < tick.Index {
// 	// 			// нашли подходящий индекс для вставки верхнего тика в массив, то делаем вставку и выходим
// 	// 			h.Ticks = slices.Insert(h.Ticks, key, upperTickToAppend)
// 	// 		}

// 	// 		prevIndex = tick.Index
// 	// 	}

// 	// 	// если не определилось, куда вставлять - то в конец
// 	// 	h.Ticks = append(h.Ticks, upperTickToAppend)
// 	// }

// 	// OLD working
// 	var lowerTickExist, upperTickExist bool
// 	for key, tick := range h.Ticks {
// 		if !lowerTickExist && int32(tick.Index) == tickLower {
// 			h.Ticks[key].LiquidityGross = new(uint256.Int).Add(tick.LiquidityGross, liquidity)
// 			h.Ticks[key].LiquidityNet = new(int256.Int).Add(tick.LiquidityNet, liquidityInt256)
// 			lowerTickExist = true
// 		} else if int32(tick.Index) == tickUpper {
// 			h.Ticks[key].LiquidityGross = new(uint256.Int).Add(tick.LiquidityGross, liquidity)
// 			h.Ticks[key].LiquidityNet = new(int256.Int).Sub(tick.LiquidityNet, liquidityInt256)
// 			upperTickExist = true

// 			// если оба тика уже инициализированы и обновлены, просто выходим
// 			if lowerTickExist {
// 				return
// 			}
// 		}
// 	}

// 	// если нижний тик ещё не был инициализирован
// 	if !lowerTickExist {
// 		var (
// 			foundPosition     bool
// 			prevIndex         int32
// 			lowerTickToAppend = kyberEntities.Tick{Index: int(tickLower), LiquidityGross: liquidity, LiquidityNet: liquidityInt256}
// 		)

// 		for key, tick := range h.Ticks {
// 			if tickLower > prevIndex && tickLower < int32(tick.Index) {
// 				// нашли подходящий индекс для вставки нижнего тика в массив
// 				h.Ticks = slices.Insert(h.Ticks, key, lowerTickToAppend)
// 				foundPosition = true
// 				break
// 			}

// 			prevIndex = int32(tick.Index)
// 		}

// 		if !foundPosition {
// 			if len(h.Ticks) > 0 && tickLower < int32(h.Ticks[0].Index) {
// 				// ниже нижнего
// 				h.Ticks = slices.Insert(h.Ticks, 0, lowerTickToAppend)
// 			} else {
// 				// выше верхнего
// 				h.Ticks = append(h.Ticks, lowerTickToAppend)
// 			}
// 		}

// 		// h.Ticks = append(h.Ticks, v3Entities.Tick{Index: tickLower, LiquidityGross: liquidityUint256, LiquidityNet: liquidityInt256})
// 	}

// 	// если верхний тик ещё не был инициализирован
// 	if !upperTickExist {
// 		var (
// 			prevIndex         int32
// 			upperTickToAppend = kyberEntities.Tick{Index: int(tickUpper), LiquidityGross: liquidity, LiquidityNet: new(int256.Int).Mul(liquidityInt256, int256.NewInt(-1))}
// 		)

// 		for key, tick := range h.Ticks {
// 			if tickUpper > prevIndex && tickUpper < int32(tick.Index) {
// 				// нашли подходящий индекс для вставки верхнего тика в массив, то делаем вставку и выходим
// 				h.Ticks = slices.Insert(h.Ticks, key, upperTickToAppend)
// 				h.buildInitializedTicksMap()
// 				return
// 			}

// 			prevIndex = int32(tick.Index)
// 		}

// 		// если не определилось, куда вставлять - то в конец
// 		h.Ticks = append(h.Ticks, upperTickToAppend)

// 		// h.Ticks = append(h.Ticks, v3Entities.Tick{Index: tickUpper, LiquidityGross: liquidityUint256, LiquidityNet: new(int256.Int).Mul(liquidityInt256, int256.NewInt(-1))})
// 	}

// 	h.buildInitializedTicksMap()

// 	// если инициализировались новые тики, требуется пересортировка
// 	// if !lowerTickExist || !upperTickExist {
// 	// 	h.reOrderTicks()
// 	// }
// }

// // актуализирует состояние тиков пула после историчекого события burn
// func (h *TicksHandler) UpdateTicksAfterBurn(tickLower, tickUpper int32, liquidity *uint256.Int) {
// 	liquidityInt256 := int256.MustFromBig(liquidity.ToBig())

// 	ltIndex := h.InitializedTicksMap[int32(tickLower)]
// 	h.Ticks[ltIndex].LiquidityGross = new(uint256.Int).Sub(h.Ticks[ltIndex].LiquidityGross, liquidity)
// 	h.Ticks[ltIndex].LiquidityNet = new(int256.Int).Sub(h.Ticks[ltIndex].LiquidityNet, liquidityInt256)

// 	utIndex := h.InitializedTicksMap[int32(tickUpper)]
// 	h.Ticks[utIndex].LiquidityGross = new(uint256.Int).Sub(h.Ticks[utIndex].LiquidityGross, liquidity)
// 	h.Ticks[utIndex].LiquidityNet = new(int256.Int).Add(h.Ticks[utIndex].LiquidityNet, liquidityInt256)

// 	// var lowerTickFound bool
// 	// for key, tick := range h.Ticks {
// 	// 	if !lowerTickFound && tick.Index == tickLower {
// 	// 		h.Ticks[key].LiquidityGross = new(uint256.Int).Sub(tick.LiquidityGross, liquidity)
// 	// 		h.Ticks[key].LiquidityNet = new(int256.Int).Sub(tick.LiquidityNet, liquidityInt256)

// 	// 		lowerTickFound = true
// 	// 	} else if tick.Index == tickUpper {
// 	// 		h.Ticks[key].LiquidityGross.Sub(tick.LiquidityGross, liquidity)
// 	// 		h.Ticks[key].LiquidityNet.Add(tick.LiquidityNet, liquidityInt256)

// 	// 		// если нашли и обновили оба тика, поиск прекращаем
// 	// 		if lowerTickFound {
// 	// 			break
// 	// 		}
// 	// 	}
// 	// }

// 	removedCount = 0
// 	if h.Ticks, removedCount = removeEmptyTicksResursive(h.Ticks); removedCount > 0 {
// 		h.buildInitializedTicksMap()
// 	}
// 	// h.reOrderTicks() // по идее, после удаления ликвидности пересортировка не требуется
// }

// var removedCount uint

// // рекурсивно удаляем пустые тики, если таковые имеются
// func removeEmptyTicksResursive(ticks []kyberEntities.Tick) ([]kyberEntities.Tick, uint) {
// 	for key, tick := range ticks {
// 		if tick.LiquidityGross.Cmp(uniconst.UI256Zero) == 0 && tick.LiquidityNet.Cmp(uniconst.I256Zero) == 0 {
// 			removedCount++
// 			return removeEmptyTicksResursive(append(ticks[:key], ticks[key+1:]...))
// 		}
// 	}
// 	return ticks, removedCount
// }

// func (h *TicksHandler) buildInitializedTicksMap() {
// 	clear(h.InitializedTicksMap)
// 	for key, tick := range h.Ticks {
// 		h.InitializedTicksMap[int32(tick.Index)] = key
// 	}
// }

// // // удаляет тик по индексу его массива (слайса)
// // func removeTickByKey(slice []v3Entities.Tick, s int) []v3Entities.Tick {
// // 	return append(slice[:s], slice[s+1:]...)
// // }

// // // переупорядочиваем (сортируем) внутренний массив тиков. он нам необходим в сортированном виде во многих местах.
// // func (h *TicksHandler) reOrderTicks() {
// // 	slices.SortFunc(h.Ticks, func(a, b v3Entities.Tick) int {
// // 		return cmp.Compare(a.Index, b.Index)
// // 	})
// // }
