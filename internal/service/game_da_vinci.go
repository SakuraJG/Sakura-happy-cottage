package service

import (
	"crypto/rand"
	"errors"
	"math/big"
	"sort"
)

type DaVinciTileView struct {
	ID       int    `json:"id"`
	Color    string `json:"color"`
	Number   *int   `json:"number,omitempty"`
	Revealed bool   `json:"revealed"`
}

type DaVinciPlayerView struct {
	UID          int64             `json:"uid"`
	Username     string            `json:"username"`
	Tiles        []DaVinciTileView `json:"tiles"`
	HiddenCount  int               `json:"hidden_count"`
	CorrectCount int               `json:"correct_count"`
	Eliminated   bool              `json:"eliminated"`
	Score        int               `json:"score"`
}

type DaVinciLog struct {
	UID  int64  `json:"uid"`
	Text string `json:"text"`
}

type DaVinciView struct {
	Phase       string              `json:"phase"`
	CurrentUID  int64               `json:"current_uid"`
	DrawnTile   *DaVinciTileView    `json:"drawn_tile,omitempty"`
	DeckCount   int                 `json:"deck_count"`
	CanStop     bool                `json:"can_stop"`
	LastCorrect bool                `json:"last_correct"`
	WinnerUID   int64               `json:"winner_uid,omitempty"`
	Players     []DaVinciPlayerView `json:"players"`
	Logs        []DaVinciLog        `json:"logs"`
}

type daVinciTile struct {
	ID       int
	Color    string
	Number   int
	Revealed bool
}

type daVinciState struct {
	Phase         string
	Deck          []daVinciTile
	Hands         map[int64][]daVinciTile
	CurrentUID    int64
	Drawn         *daVinciTile
	CanStop       bool
	LastCorrect   bool
	CorrectCounts map[int64]int
	WinnerUID     int64
	Logs          []DaVinciLog
}

func (m *GameRoomManager) startDaVinciLocked(room *gameRoom) error {
	if len(room.Players) < 2 || len(room.Players) > 4 {
		return errors.New("达芬奇密码需要 2 到 4 名玩家")
	}
	deck := newDaVinciDeck()
	if err := shuffleDaVinciTiles(deck); err != nil {
		return err
	}
	state := &daVinciState{
		Phase: "guessing", Deck: deck, Hands: make(map[int64][]daVinciTile), CorrectCounts: make(map[int64]int), Logs: []DaVinciLog{},
	}
	initialCount := 3
	if len(room.Players) == 2 {
		initialCount = 4
	}
	for _, player := range room.Players {
		state.Hands[player.UID] = append([]daVinciTile(nil), state.Deck[:initialCount]...)
		state.Deck = state.Deck[initialCount:]
		sortDaVinciTiles(state.Hands[player.UID])
	}
	state.CurrentUID = room.Players[(room.CurrentRound-1)%len(room.Players)].UID
	state.addLog(0, "密码牌已经发放")
	room.DaVinci = state
	room.Status = "playing"
	room.RoundComplete = false
	room.WinnerUID = 0
	room.RoundScores = map[int64]int{}
	room.UpdatedAt = nowTime()
	m.beginDaVinciTurnLocked(room)
	m.runDaVinciBotsLocked(room)
	return nil
}

func (m *GameRoomManager) daVinciActionLocked(room *gameRoom, userID int64, action GameRoomAction) error {
	state := room.DaVinci
	if state == nil || room.Status != "playing" || state.CurrentUID != userID {
		return ErrRoomState
	}
	player := roomPlayerByUID(room, userID)
	if player == nil || player.Bot {
		return ErrRoomForbidden
	}
	var err error
	switch action.Type {
	case "guess_tile":
		err = m.applyDaVinciGuessLocked(room, userID, action.TargetUID, action.TileID, action.Number)
	case "stop_guessing":
		err = m.applyDaVinciStopLocked(room, userID)
	default:
		err = errors.New("猜测操作无效")
	}
	if err != nil {
		return err
	}
	m.runDaVinciBotsLocked(room)
	return nil
}

func (m *GameRoomManager) beginDaVinciTurnLocked(room *gameRoom) {
	state := room.DaVinci
	state.CanStop = false
	state.LastCorrect = false
	state.Drawn = nil
	if len(state.Deck) > 0 {
		drawn := state.Deck[0]
		state.Deck = state.Deck[1:]
		state.Drawn = &drawn
		state.addLog(state.CurrentUID, "摸取了一张密码牌")
	}
}

func (m *GameRoomManager) applyDaVinciGuessLocked(room *gameRoom, userID, targetUID int64, tileID, number int) error {
	state := room.DaVinci
	if targetUID == userID || number < 0 || number > 11 {
		return errors.New("请选择对手的暗牌并猜测 0 到 11")
	}
	tiles := state.Hands[targetUID]
	index := daVinciTileIndex(tiles, tileID)
	if index < 0 || tiles[index].Revealed {
		return errors.New("目标密码牌无效")
	}
	if tiles[index].Number == number {
		tiles[index].Revealed = true
		state.Hands[targetUID] = tiles
		state.CorrectCounts[userID]++
		state.CanStop = true
		state.LastCorrect = true
		state.addLog(userID, "猜中了一张 "+tiles[index].Color+" 色密码牌")
		if m.daVinciWinnerLocked(room, userID) {
			m.completeDaVinciLocked(room, userID)
		}
		return nil
	}
	state.LastCorrect = false
	state.addLog(userID, "猜测错误，回合结束")
	if state.Drawn != nil {
		drawn := *state.Drawn
		drawn.Revealed = true
		state.Hands[userID] = append(state.Hands[userID], drawn)
		sortDaVinciTiles(state.Hands[userID])
		state.Drawn = nil
	}
	m.advanceDaVinciTurnLocked(room)
	return nil
}

func (m *GameRoomManager) applyDaVinciStopLocked(room *gameRoom, userID int64) error {
	state := room.DaVinci
	if !state.CanStop {
		return errors.New("至少猜中一次后才能主动结束回合")
	}
	if state.Drawn != nil {
		state.Hands[userID] = append(state.Hands[userID], *state.Drawn)
		sortDaVinciTiles(state.Hands[userID])
		state.Drawn = nil
	}
	state.addLog(userID, "保留摸到的牌并结束回合")
	m.advanceDaVinciTurnLocked(room)
	return nil
}

func (m *GameRoomManager) advanceDaVinciTurnLocked(room *gameRoom) {
	state := room.DaVinci
	for offset := 1; offset <= len(room.Players); offset++ {
		candidate := nextRoomPlayerUID(room, state.CurrentUID)
		state.CurrentUID = candidate
		if daVinciHiddenCount(state.Hands[candidate]) > 0 {
			break
		}
	}
	m.beginDaVinciTurnLocked(room)
}

func (m *GameRoomManager) daVinciWinnerLocked(room *gameRoom, actorUID int64) bool {
	state := room.DaVinci
	for _, player := range room.Players {
		if player.UID != actorUID && daVinciHiddenCount(state.Hands[player.UID]) > 0 {
			return false
		}
	}
	return true
}

func (m *GameRoomManager) completeDaVinciLocked(room *gameRoom, winnerUID int64) {
	state := room.DaVinci
	if state.Drawn != nil {
		state.Hands[winnerUID] = append(state.Hands[winnerUID], *state.Drawn)
		sortDaVinciTiles(state.Hands[winnerUID])
		state.Drawn = nil
	}
	state.Phase = "round_complete"
	state.CurrentUID = 0
	state.WinnerUID = winnerUID
	state.addLog(winnerUID, "破解了所有对手的密码")
	for _, player := range room.Players {
		score := state.CorrectCounts[player.UID] * 10
		if player.UID == winnerUID {
			score += 100
		}
		room.RoundScores[player.UID] = score
		room.TotalScores[player.UID] += score
	}
	room.Status = "round_complete"
	room.RoundComplete = true
	room.WinnerUID = winnerUID
	room.UpdatedAt = nowTime()
}

func (m *GameRoomManager) runDaVinciBotsLocked(room *gameRoom) {
	for steps := 0; steps < 32 && room.Status == "playing"; steps++ {
		state := room.DaVinci
		player := roomPlayerByUID(room, state.CurrentUID)
		if player == nil || !player.Bot {
			return
		}
		if state.CanStop && steps%2 == 1 {
			_ = m.applyDaVinciStopLocked(room, player.UID)
			continue
		}
		targetUID, tileID, number, ok := daVinciBotGuess(room, state, player.UID)
		if !ok {
			return
		}
		_ = m.applyDaVinciGuessLocked(room, player.UID, targetUID, tileID, number)
	}
}

func (m *GameRoomManager) daVinciViewLocked(room *gameRoom, userID int64) *DaVinciView {
	state := room.DaVinci
	if state == nil {
		return nil
	}
	view := &DaVinciView{
		Phase: state.Phase, CurrentUID: state.CurrentUID, DeckCount: len(state.Deck),
		CanStop: state.CanStop && state.CurrentUID == userID, LastCorrect: state.LastCorrect,
		WinnerUID: state.WinnerUID, Players: []DaVinciPlayerView{}, Logs: append([]DaVinciLog(nil), state.Logs...),
	}
	if state.Drawn != nil && state.CurrentUID == userID {
		drawn := daVinciTileView(*state.Drawn, true)
		view.DrawnTile = &drawn
	}
	for _, player := range room.Players {
		tiles := state.Hands[player.UID]
		item := DaVinciPlayerView{
			UID: player.UID, Username: player.Username, Tiles: make([]DaVinciTileView, 0, len(tiles)),
			HiddenCount: daVinciHiddenCount(tiles), CorrectCount: state.CorrectCounts[player.UID],
			Eliminated: daVinciHiddenCount(tiles) == 0, Score: room.TotalScores[player.UID],
		}
		for _, tile := range tiles {
			item.Tiles = append(item.Tiles, daVinciTileView(tile, player.UID == userID || room.RoundComplete || tile.Revealed))
		}
		view.Players = append(view.Players, item)
	}
	return view
}

func newDaVinciDeck() []daVinciTile {
	deck := make([]daVinciTile, 0, 24)
	id := 1
	for number := 0; number <= 11; number++ {
		for _, color := range []string{"black", "white"} {
			deck = append(deck, daVinciTile{ID: id, Color: color, Number: number})
			id++
		}
	}
	return deck
}

func shuffleDaVinciTiles(tiles []daVinciTile) error {
	for i := len(tiles) - 1; i > 0; i-- {
		value, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return err
		}
		j := int(value.Int64())
		tiles[i], tiles[j] = tiles[j], tiles[i]
	}
	return nil
}

func sortDaVinciTiles(tiles []daVinciTile) {
	sort.Slice(tiles, func(i, j int) bool {
		if tiles[i].Number == tiles[j].Number {
			return tiles[i].Color == "black"
		}
		return tiles[i].Number < tiles[j].Number
	})
}

func daVinciTileView(tile daVinciTile, showNumber bool) DaVinciTileView {
	view := DaVinciTileView{ID: tile.ID, Color: tile.Color, Revealed: tile.Revealed}
	if showNumber {
		number := tile.Number
		view.Number = &number
	}
	return view
}

func daVinciTileIndex(tiles []daVinciTile, id int) int {
	for index := range tiles {
		if tiles[index].ID == id {
			return index
		}
	}
	return -1
}

func daVinciHiddenCount(tiles []daVinciTile) int {
	count := 0
	for _, tile := range tiles {
		if !tile.Revealed {
			count++
		}
	}
	return count
}

func daVinciBotGuess(room *gameRoom, state *daVinciState, botUID int64) (int64, int, int, bool) {
	for _, player := range room.Players {
		if player.UID == botUID {
			continue
		}
		for index, tile := range state.Hands[player.UID] {
			if tile.Revealed {
				continue
			}
			low, high := 0, 11
			for left := index - 1; left >= 0; left-- {
				if state.Hands[player.UID][left].Revealed {
					low = state.Hands[player.UID][left].Number
					break
				}
			}
			for right := index + 1; right < len(state.Hands[player.UID]); right++ {
				if state.Hands[player.UID][right].Revealed {
					high = state.Hands[player.UID][right].Number
					break
				}
			}
			return player.UID, tile.ID, (low + high) / 2, true
		}
	}
	return 0, 0, 0, false
}

func (state *daVinciState) addLog(uid int64, text string) {
	state.Logs = append(state.Logs, DaVinciLog{UID: uid, Text: text})
	if len(state.Logs) > 40 {
		state.Logs = state.Logs[len(state.Logs)-40:]
	}
}
