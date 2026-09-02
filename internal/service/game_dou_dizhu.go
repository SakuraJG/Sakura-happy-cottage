package service

import (
	"crypto/rand"
	"errors"
	"math/big"
	"sort"
	"time"
)

type PlayingCard struct {
	ID    int    `json:"id"`
	Rank  string `json:"rank"`
	Value int    `json:"value"`
	Suit  string `json:"suit"`
}

type DouDizhuPlayerView struct {
	UID       int64         `json:"uid"`
	Username  string        `json:"username"`
	Role      string        `json:"role,omitempty"`
	HandCount int           `json:"hand_count"`
	Hand      []PlayingCard `json:"hand,omitempty"`
	Bid       int           `json:"bid"`
	Score     int           `json:"score"`
}

type DouDizhuPlayView struct {
	UID      int64         `json:"uid"`
	Cards    []PlayingCard `json:"cards"`
	Kind     string        `json:"kind"`
	KindName string        `json:"kind_name"`
}

type DouDizhuLog struct {
	UID  int64  `json:"uid"`
	Text string `json:"text"`
}

type DouDizhuView struct {
	Phase           string               `json:"phase"`
	CurrentUID      int64                `json:"current_uid"`
	TurnSecondsLeft int                  `json:"turn_seconds_left"`
	LandlordUID     int64                `json:"landlord_uid,omitempty"`
	HighestBid      int                  `json:"highest_bid"`
	HighestUID      int64                `json:"highest_uid,omitempty"`
	BottomCards     []PlayingCard        `json:"bottom_cards"`
	LastPlay        *DouDizhuPlayView    `json:"last_play,omitempty"`
	Multiplier      int                  `json:"multiplier"`
	WinnerSide      string               `json:"winner_side,omitempty"`
	Players         []DouDizhuPlayerView `json:"players"`
	Logs            []DouDizhuLog        `json:"logs"`
	AllowedBids     []int                `json:"allowed_bids"`
	CanPass         bool                 `json:"can_pass"`
}

type douDizhuCombo struct {
	Kind string
	Main int
	Size int
}

type douDizhuPlay struct {
	UID   int64
	Cards []PlayingCard
	Combo douDizhuCombo
}

type douDizhuState struct {
	Phase        string
	Hands        map[int64][]PlayingCard
	Bottom       []PlayingCard
	CurrentUID   int64
	BidStartUID  int64
	BidCount     int
	Bids         map[int64]int
	HighestBid   int
	HighestUID   int64
	LandlordUID  int64
	LastPlay     *douDizhuPlay
	TrickLeader  int64
	PassCount    int
	Multiplier   int
	PlayCounts   map[int64]int
	WinnerSide   string
	Logs         []DouDizhuLog
	TurnDeadline time.Time
}

const douDizhuTurnDuration = 30 * time.Second

func (m *GameRoomManager) startDouDizhuLocked(room *gameRoom) error {
	if len(room.Players) != 3 {
		return errors.New("斗地主需要正好 3 名玩家")
	}
	deck := newDouDizhuDeck()
	if err := shufflePlayingCards(deck); err != nil {
		return err
	}
	state := &douDizhuState{
		Phase: "bidding", Hands: make(map[int64][]PlayingCard), Bottom: append([]PlayingCard(nil), deck[51:]...),
		Bids: make(map[int64]int), Multiplier: 1, PlayCounts: make(map[int64]int), Logs: []DouDizhuLog{},
	}
	for index, player := range room.Players {
		hand := append([]PlayingCard(nil), deck[index*17:(index+1)*17]...)
		sortPlayingCards(hand)
		state.Hands[player.UID] = hand
	}
	starter := room.Players[(room.CurrentRound-1)%len(room.Players)].UID
	setDouDizhuTurnLocked(state, starter)
	state.BidStartUID = starter
	state.addLog(0, "发牌完成，开始叫分")
	room.DouDizhu = state
	room.Status = "playing"
	room.RoundComplete = false
	room.WinnerUID = 0
	room.RoundScores = map[int64]int{}
	room.UpdatedAt = nowTime()
	m.runDouDizhuBotsLocked(room)
	return nil
}

func (m *GameRoomManager) douDizhuActionLocked(room *gameRoom, userID int64, action GameRoomAction) error {
	state := room.DouDizhu
	if state == nil || room.Status != "playing" || state.CurrentUID != userID {
		return ErrRoomState
	}
	player := roomPlayerByUID(room, userID)
	if player == nil || player.Bot {
		return ErrRoomForbidden
	}
	var err error
	switch state.Phase {
	case "bidding":
		if action.Type != "bid" {
			return errors.New("当前正在叫分")
		}
		err = m.applyDouDizhuBidLocked(room, userID, action.Bid)
	case "playing":
		switch action.Type {
		case "play_cards":
			err = m.applyDouDizhuPlayLocked(room, userID, action.Cards)
		case "pass":
			err = m.applyDouDizhuPassLocked(room, userID)
		default:
			err = errors.New("出牌操作无效")
		}
	default:
		err = ErrRoomState
	}
	if err != nil {
		return err
	}
	m.runDouDizhuBotsLocked(room)
	return nil
}

func (m *GameRoomManager) applyDouDizhuBidLocked(room *gameRoom, userID int64, bid int) error {
	state := room.DouDizhu
	if bid < 0 || bid > 3 || (bid != 0 && bid <= state.HighestBid) {
		return errors.New("叫分必须高于当前分数，也可以选择不叫")
	}
	state.Bids[userID] = bid
	state.BidCount++
	if bid > state.HighestBid {
		state.HighestBid = bid
		state.HighestUID = userID
	}
	state.addLog(userID, bidText(bid))
	if bid == 3 || state.BidCount >= len(room.Players) {
		if state.HighestUID == 0 {
			state.HighestUID = state.BidStartUID
			state.HighestBid = 1
			state.Bids[state.HighestUID] = 1
			state.addLog(state.HighestUID, "无人叫分，首位玩家以 1 分成为地主")
		}
		state.LandlordUID = state.HighestUID
		state.Hands[state.LandlordUID] = append(state.Hands[state.LandlordUID], state.Bottom...)
		sortPlayingCards(state.Hands[state.LandlordUID])
		state.TrickLeader = state.LandlordUID
		state.Phase = "playing"
		setDouDizhuTurnLocked(state, state.LandlordUID)
		state.addLog(state.LandlordUID, "成为地主，获得三张底牌")
		return nil
	}
	setDouDizhuTurnLocked(state, nextRoomPlayerUID(room, userID))
	return nil
}

func (m *GameRoomManager) applyDouDizhuPlayLocked(room *gameRoom, userID int64, ids []int) error {
	state := room.DouDizhu
	if len(ids) == 0 {
		return errors.New("请选择要出的牌")
	}
	cards, ok := selectPlayingCards(state.Hands[userID], ids)
	if !ok {
		return errors.New("所选手牌无效")
	}
	combo, ok := classifyDouDizhu(cards)
	if !ok {
		return errors.New("这些牌不构成合法牌型")
	}
	if state.LastPlay != nil && state.LastPlay.UID != userID && !beatsDouDizhu(combo, state.LastPlay.Combo) {
		return errors.New("所选牌型无法压过上一手")
	}
	state.Hands[userID] = removePlayingCards(state.Hands[userID], ids)
	state.LastPlay = &douDizhuPlay{UID: userID, Cards: cards, Combo: combo}
	state.TrickLeader = userID
	state.PassCount = 0
	state.PlayCounts[userID]++
	if combo.Kind == "bomb" || combo.Kind == "rocket" {
		state.Multiplier *= 2
	}
	state.addLog(userID, "打出"+douDizhuKindName(combo.Kind))
	if len(state.Hands[userID]) == 0 {
		m.completeDouDizhuLocked(room, userID)
		return nil
	}
	setDouDizhuTurnLocked(state, nextRoomPlayerUID(room, userID))
	return nil
}

func (m *GameRoomManager) applyDouDizhuPassLocked(room *gameRoom, userID int64) error {
	state := room.DouDizhu
	if state.LastPlay == nil || state.LastPlay.UID == userID {
		return errors.New("当前轮次必须出牌")
	}
	state.PassCount++
	state.addLog(userID, "选择不出")
	if state.PassCount >= 2 {
		leader := state.TrickLeader
		if roomPlayerByUID(room, leader) == nil {
			leader = userID
		}
		state.LastPlay = nil
		state.PassCount = 0
		state.TrickLeader = leader
		setDouDizhuTurnLocked(state, leader)
		state.addLog(0, "新一轮出牌")
	} else {
		setDouDizhuTurnLocked(state, nextRoomPlayerUID(room, userID))
	}
	return nil
}

func (m *GameRoomManager) completeDouDizhuLocked(room *gameRoom, winnerUID int64) {
	state := room.DouDizhu
	landlordWon := winnerUID == state.LandlordUID
	state.WinnerSide = "farmers"
	if landlordWon {
		state.WinnerSide = "landlord"
	}
	spring := false
	if landlordWon {
		spring = true
		for _, player := range room.Players {
			if player.UID != state.LandlordUID && state.PlayCounts[player.UID] > 0 {
				spring = false
			}
		}
	} else if state.PlayCounts[state.LandlordUID] <= 1 {
		spring = true
	}
	if spring {
		state.Multiplier *= 2
		state.addLog(0, "触发春天，倍率翻倍")
	}
	base := max(1, state.HighestBid) * state.Multiplier
	for _, player := range room.Players {
		score := base
		if player.UID == state.LandlordUID {
			score = base * 2
			if !landlordWon {
				score = -score
			}
		} else if landlordWon {
			score = -score
		}
		room.RoundScores[player.UID] = score
		room.TotalScores[player.UID] += score
	}
	state.Phase = "round_complete"
	state.CurrentUID = 0
	state.TurnDeadline = time.Time{}
	room.Status = "round_complete"
	room.RoundComplete = true
	room.WinnerUID = winnerUID
	room.UpdatedAt = nowTime()
	state.addLog(winnerUID, "本局获胜")
}

func (m *GameRoomManager) runDouDizhuBotsLocked(room *gameRoom) {
	for steps := 0; steps < 24 && room.Status == "playing"; steps++ {
		state := room.DouDizhu
		player := roomPlayerByUID(room, state.CurrentUID)
		if player == nil || !player.Bot {
			return
		}
		if state.Phase == "bidding" {
			bid := douDizhuBotBid(state.Hands[player.UID])
			if bid <= state.HighestBid {
				bid = 0
			}
			_ = m.applyDouDizhuBidLocked(room, player.UID, bid)
			continue
		}
		if state.Phase != "playing" {
			return
		}
		cards := douDizhuBotPlay(state.Hands[player.UID], state.LastPlay, player.UID)
		if len(cards) == 0 {
			_ = m.applyDouDizhuPassLocked(room, player.UID)
			continue
		}
		ids := make([]int, len(cards))
		for i := range cards {
			ids[i] = cards[i].ID
		}
		_ = m.applyDouDizhuPlayLocked(room, player.UID, ids)
	}
}

func (m *GameRoomManager) douDizhuViewLocked(room *gameRoom, userID int64) *DouDizhuView {
	state := room.DouDizhu
	if state == nil {
		return nil
	}
	view := &DouDizhuView{
		Phase: state.Phase, CurrentUID: state.CurrentUID, TurnSecondsLeft: douDizhuSecondsLeft(state), LandlordUID: state.LandlordUID,
		HighestBid: state.HighestBid, HighestUID: state.HighestUID, Multiplier: state.Multiplier,
		WinnerSide: state.WinnerSide, Players: []DouDizhuPlayerView{}, Logs: append([]DouDizhuLog(nil), state.Logs...),
		BottomCards: []PlayingCard{}, AllowedBids: []int{}, CanPass: state.LastPlay != nil && state.LastPlay.UID != userID,
	}
	if state.LandlordUID != 0 {
		view.BottomCards = append([]PlayingCard(nil), state.Bottom...)
	}
	if state.LastPlay != nil {
		view.LastPlay = &DouDizhuPlayView{UID: state.LastPlay.UID, Cards: append([]PlayingCard(nil), state.LastPlay.Cards...), Kind: state.LastPlay.Combo.Kind, KindName: douDizhuKindName(state.LastPlay.Combo.Kind)}
	}
	if state.Phase == "bidding" && state.CurrentUID == userID {
		view.AllowedBids = []int{0}
		for bid := state.HighestBid + 1; bid <= 3; bid++ {
			view.AllowedBids = append(view.AllowedBids, bid)
		}
	}
	for _, player := range room.Players {
		item := DouDizhuPlayerView{UID: player.UID, Username: player.Username, HandCount: len(state.Hands[player.UID]), Bid: state.Bids[player.UID], Score: room.TotalScores[player.UID]}
		if state.LandlordUID != 0 {
			item.Role = "farmer"
			if player.UID == state.LandlordUID {
				item.Role = "landlord"
			}
		}
		if player.UID == userID || room.RoundComplete {
			item.Hand = append([]PlayingCard(nil), state.Hands[player.UID]...)
		}
		view.Players = append(view.Players, item)
	}
	return view
}

func setDouDizhuTurnLocked(state *douDizhuState, uid int64) {
	state.CurrentUID = uid
	if uid == 0 || state.Phase == "round_complete" {
		state.TurnDeadline = time.Time{}
		return
	}
	state.TurnDeadline = time.Now().Add(douDizhuTurnDuration)
}

func douDizhuSecondsLeft(state *douDizhuState) int {
	if state.CurrentUID == 0 || state.TurnDeadline.IsZero() {
		return 0
	}
	remaining := time.Until(state.TurnDeadline)
	if remaining <= 0 {
		return 0
	}
	return int((remaining + time.Second - 1) / time.Second)
}

func newDouDizhuDeck() []PlayingCard {
	suits := []string{"spade", "heart", "club", "diamond"}
	ranks := []string{"3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A", "2"}
	deck := make([]PlayingCard, 0, 54)
	id := 1
	for value, rank := range ranks {
		for _, suit := range suits {
			deck = append(deck, PlayingCard{ID: id, Rank: rank, Value: value + 3, Suit: suit})
			id++
		}
	}
	deck = append(deck, PlayingCard{ID: id, Rank: "小王", Value: 16, Suit: "joker"})
	deck = append(deck, PlayingCard{ID: id + 1, Rank: "大王", Value: 17, Suit: "joker"})
	return deck
}

func shufflePlayingCards(cards []PlayingCard) error {
	for i := len(cards) - 1; i > 0; i-- {
		value, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return err
		}
		j := int(value.Int64())
		cards[i], cards[j] = cards[j], cards[i]
	}
	return nil
}

func sortPlayingCards(cards []PlayingCard) {
	sort.Slice(cards, func(i, j int) bool {
		if cards[i].Value == cards[j].Value {
			return cards[i].ID < cards[j].ID
		}
		return cards[i].Value < cards[j].Value
	})
}

func selectPlayingCards(hand []PlayingCard, ids []int) ([]PlayingCard, bool) {
	wanted := make(map[int]bool, len(ids))
	for _, id := range ids {
		if wanted[id] {
			return nil, false
		}
		wanted[id] = true
	}
	selected := make([]PlayingCard, 0, len(ids))
	for _, card := range hand {
		if wanted[card.ID] {
			selected = append(selected, card)
			delete(wanted, card.ID)
		}
	}
	if len(wanted) != 0 {
		return nil, false
	}
	sortPlayingCards(selected)
	return selected, true
}

func removePlayingCards(hand []PlayingCard, ids []int) []PlayingCard {
	removed := make(map[int]bool, len(ids))
	for _, id := range ids {
		removed[id] = true
	}
	result := make([]PlayingCard, 0, len(hand)-len(ids))
	for _, card := range hand {
		if !removed[card.ID] {
			result = append(result, card)
		}
	}
	return result
}

func classifyDouDizhu(cards []PlayingCard) (douDizhuCombo, bool) {
	counts := make(map[int]int)
	for _, card := range cards {
		counts[card.Value]++
	}
	values := make([]int, 0, len(counts))
	for value := range counts {
		values = append(values, value)
	}
	sort.Ints(values)
	n := len(cards)
	if n == 2 && counts[16] == 1 && counts[17] == 1 {
		return douDizhuCombo{Kind: "rocket", Main: 17, Size: 2}, true
	}
	if len(values) == 1 {
		kind := map[int]string{1: "single", 2: "pair", 3: "triple", 4: "bomb"}[n]
		if kind != "" {
			return douDizhuCombo{Kind: kind, Main: values[0], Size: n}, true
		}
	}
	if n == 4 {
		for _, value := range values {
			if counts[value] == 3 {
				return douDizhuCombo{Kind: "triple_single", Main: value, Size: n}, true
			}
		}
	}
	if n == 5 {
		triple, pair := 0, 0
		for _, value := range values {
			if counts[value] == 3 {
				triple = value
			} else if counts[value] == 2 {
				pair = value
			}
		}
		if triple != 0 && pair != 0 {
			return douDizhuCombo{Kind: "triple_pair", Main: triple, Size: n}, true
		}
	}
	if n >= 5 && len(values) == n && consecutiveValues(values) && values[len(values)-1] <= 14 {
		return douDizhuCombo{Kind: "straight", Main: values[len(values)-1], Size: n}, true
	}
	if n >= 6 && n%2 == 0 && len(values) == n/2 && consecutiveValues(values) && values[len(values)-1] <= 14 {
		allPairs := true
		for _, value := range values {
			allPairs = allPairs && counts[value] == 2
		}
		if allPairs {
			return douDizhuCombo{Kind: "pair_straight", Main: values[len(values)-1], Size: n}, true
		}
	}
	if n == 6 || n == 8 {
		for _, value := range values {
			if counts[value] != 4 {
				continue
			}
			if n == 6 {
				return douDizhuCombo{Kind: "four_two_single", Main: value, Size: n}, true
			}
			pairs := 0
			for _, other := range values {
				if other != value && counts[other] == 2 {
					pairs++
				}
			}
			if pairs == 2 {
				return douDizhuCombo{Kind: "four_two_pair", Main: value, Size: n}, true
			}
		}
	}
	for triples := 2; triples <= 6; triples++ {
		for start := 3; start+triples-1 <= 14; start++ {
			valid := true
			remaining := make(map[int]int, len(counts))
			for value, count := range counts {
				remaining[value] = count
			}
			for value := start; value < start+triples; value++ {
				if remaining[value] < 3 {
					valid = false
					break
				}
				remaining[value] -= 3
			}
			if !valid {
				continue
			}
			left := 0
			pairWings := 0
			pairOnly := true
			for _, count := range remaining {
				left += count
				if count > 0 {
					if count != 2 {
						pairOnly = false
					}
					pairWings++
				}
			}
			main := start + triples - 1
			switch {
			case left == 0 && n == triples*3:
				return douDizhuCombo{Kind: "airplane", Main: main, Size: n}, true
			case left == triples && n == triples*4:
				return douDizhuCombo{Kind: "airplane_single", Main: main, Size: n}, true
			case left == triples*2 && pairOnly && pairWings == triples && n == triples*5:
				return douDizhuCombo{Kind: "airplane_pair", Main: main, Size: n}, true
			}
		}
	}
	return douDizhuCombo{}, false
}

func beatsDouDizhu(next, previous douDizhuCombo) bool {
	if next.Kind == "rocket" {
		return previous.Kind != "rocket"
	}
	if previous.Kind == "rocket" {
		return false
	}
	if next.Kind == "bomb" && previous.Kind != "bomb" {
		return true
	}
	if next.Kind != previous.Kind || next.Size != previous.Size {
		return false
	}
	return next.Main > previous.Main
}

func consecutiveValues(values []int) bool {
	for i := 1; i < len(values); i++ {
		if values[i] != values[i-1]+1 {
			return false
		}
	}
	return true
}

func douDizhuKindName(kind string) string {
	return map[string]string{
		"single": "单张", "pair": "对子", "triple": "三张", "triple_single": "三带一", "triple_pair": "三带二",
		"straight": "顺子", "pair_straight": "连对", "airplane": "飞机", "airplane_single": "飞机带单",
		"airplane_pair": "飞机带对", "four_two_single": "四带二", "four_two_pair": "四带两对", "bomb": "炸弹", "rocket": "王炸",
	}[kind]
}

func douDizhuBotBid(hand []PlayingCard) int {
	score := 0
	counts := make(map[int]int)
	for _, card := range hand {
		counts[card.Value]++
		if card.Value >= 16 {
			score += 3
		} else if card.Value == 15 {
			score += 2
		} else if card.Value == 14 {
			score++
		}
	}
	for _, count := range counts {
		if count == 4 {
			score += 4
		}
	}
	if score >= 10 {
		return 3
	}
	if score >= 7 {
		return 2
	}
	if score >= 4 {
		return 1
	}
	return 0
}

func douDizhuBotPlay(hand []PlayingCard, last *douDizhuPlay, uid int64) []PlayingCard {
	if combo, ok := classifyDouDizhu(hand); ok && (last == nil || last.UID == uid || beatsDouDizhu(combo, last.Combo)) {
		return append([]PlayingCard(nil), hand...)
	}
	if last == nil || last.UID == uid {
		return []PlayingCard{hand[0]}
	}
	counts := make(map[int][]PlayingCard)
	for _, card := range hand {
		counts[card.Value] = append(counts[card.Value], card)
	}
	need := map[string]int{"single": 1, "pair": 2, "triple": 3, "bomb": 4}[last.Combo.Kind]
	if need > 0 {
		for value := last.Combo.Main + 1; value <= 17; value++ {
			if len(counts[value]) == need {
				return append([]PlayingCard(nil), counts[value]...)
			}
		}
	}
	if last.Combo.Kind != "bomb" && last.Combo.Kind != "rocket" {
		for value := 3; value <= 15; value++ {
			if len(counts[value]) == 4 {
				return append([]PlayingCard(nil), counts[value]...)
			}
		}
	}
	if last.Combo.Kind != "rocket" && len(counts[16]) == 1 && len(counts[17]) == 1 {
		return []PlayingCard{counts[16][0], counts[17][0]}
	}
	return nil
}

func (state *douDizhuState) addLog(uid int64, text string) {
	state.Logs = append(state.Logs, DouDizhuLog{UID: uid, Text: text})
	if len(state.Logs) > 40 {
		state.Logs = state.Logs[len(state.Logs)-40:]
	}
}

func bidText(bid int) string {
	if bid == 0 {
		return "不叫"
	}
	return []string{"", "叫 1 分", "叫 2 分", "叫 3 分"}[bid]
}
