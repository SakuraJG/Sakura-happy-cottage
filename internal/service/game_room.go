package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
	"sakura-happy-cottage/internal/domain"
)

var (
	ErrRoomNotFound    = errors.New("房间不存在或已过期")
	ErrRoomForbidden   = errors.New("没有操作这个房间的权限")
	ErrRoomPassword    = errors.New("房间密码错误")
	ErrRoomFull        = errors.New("房间人数已满")
	ErrRoomAlreadyOpen = errors.New("你已经在另一个房间中")
	ErrRoomState       = errors.New("当前房间状态不允许此操作")
)

var roomColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type CreateGameRoomInput struct {
	GameID       string
	Name         string
	MaxPlayers   int
	Rounds       int
	RoundSeconds int
	Password     string
	RecordStats  bool
}

type GameRoomPlayerView struct {
	UID      int64  `json:"uid"`
	Username string `json:"username"`
	Ready    bool   `json:"ready"`
	Host     bool   `json:"host,omitempty"`
	Bot      bool   `json:"bot,omitempty"`
}

type GamePoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type GameStroke struct {
	ID      int64       `json:"id"`
	Color   string      `json:"color"`
	Width   float64     `json:"width"`
	Tool    string      `json:"tool,omitempty"`
	Opacity float64     `json:"opacity,omitempty"`
	Points  []GamePoint `json:"points"`
}

type GameRoomMessage struct {
	ID        int64     `json:"id"`
	AuthorUID int64     `json:"author_uid"`
	Author    string    `json:"author"`
	Text      string    `json:"text"`
	Kind      string    `json:"kind,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type GameRoomView struct {
	Code              string               `json:"code"`
	Name              string               `json:"name"`
	GameID            string               `json:"gameID"`
	HostUID           int64                `json:"hostUID"`
	MaxPlayers        int                  `json:"maxPlayers"`
	Rounds            int                  `json:"rounds"`
	RoundSeconds      int                  `json:"roundSeconds"`
	PasswordProtected bool                 `json:"passwordProtected"`
	RecordStats       bool                 `json:"recordStats"`
	Status            string               `json:"status"`
	CurrentRound      int                  `json:"currentRound"`
	DrawerUID         int64                `json:"drawerUID"`
	SecondsLeft       int                  `json:"secondsLeft"`
	Word              string               `json:"word,omitempty"`
	WordCategory      string               `json:"wordCategory,omitempty"`
	WordHint          string               `json:"wordHint,omitempty"`
	WordLength        int                  `json:"wordLength,omitempty"`
	RoundComplete     bool                 `json:"roundComplete"`
	WinnerUID         int64                `json:"winnerUID,omitempty"`
	RoundScore        int                  `json:"roundScore"`
	TotalScore        int                  `json:"totalScore"`
	DrawingLocked     bool                 `json:"drawingLocked"`
	Players           []GameRoomPlayerView `json:"players"`
	Strokes           []GameStroke         `json:"strokes"`
	Messages          []GameRoomMessage    `json:"messages"`
	DouDizhu          *DouDizhuView        `json:"douDizhu,omitempty"`
	DaVinci           *DaVinciView         `json:"daVinci,omitempty"`
}

type GameInvitationView struct {
	Code              string    `json:"code"`
	RoomName          string    `json:"room_name"`
	FromUID           int64     `json:"from_uid"`
	FromUsername      string    `json:"from_username"`
	PasswordProtected bool      `json:"password_protected"`
	CreatedAt         time.Time `json:"created_at"`
	GameID            string    `json:"game_id"`
}

type gameRoomPlayer struct {
	UID      int64
	Username string
	Ready    bool
	Host     bool
	Bot      bool
}

type gameRoom struct {
	Code             string
	Name             string
	GameID           string
	HostUID          int64
	MaxPlayers       int
	Rounds           int
	RoundSeconds     int
	PasswordHash     string
	RecordStats      bool
	Status           string
	CurrentRound     int
	DrawerUID        int64
	Word             drawingWord
	RoundStartedAt   time.Time
	RoundComplete    bool
	DrawingLocked    bool
	SecondsLeftAtEnd int
	WinnerUID        int64
	RoundScores      map[int64]int
	TotalScores      map[int64]int
	Players          []gameRoomPlayer
	Strokes          []GameStroke
	Messages         []GameRoomMessage
	NextStrokeID     int64
	NextMessageID    int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DouDizhu         *douDizhuState
	DaVinci          *daVinciState
}

type GameRoomAction struct {
	Type      string `json:"type"`
	Bid       int    `json:"bid,omitempty"`
	Cards     []int  `json:"cards,omitempty"`
	TargetUID int64  `json:"target_uid,omitempty"`
	TileID    int    `json:"tile_id,omitempty"`
	Number    int    `json:"number,omitempty"`
}

type gameInvitation struct {
	Code         string
	FromUID      int64
	FromUsername string
	CreatedAt    time.Time
}

type GameRoomManager struct {
	mu          sync.Mutex
	rooms       map[string]*gameRoom
	invitations map[int64]map[string]gameInvitation
	watchers    map[string]map[chan struct{}]struct{}
	bcryptCost  int
}

func NewGameRoomManager(bcryptCost int) *GameRoomManager {
	return &GameRoomManager{
		rooms: make(map[string]*gameRoom), invitations: make(map[int64]map[string]gameInvitation),
		watchers: make(map[string]map[chan struct{}]struct{}), bcryptCost: bcryptCost,
	}
}

func (m *GameRoomManager) Create(owner domain.User, input CreateGameRoomInput) (GameRoomView, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.GameID = strings.TrimSpace(input.GameID)
	if input.GameID == "" {
		input.GameID = DrawingGuessGame
	}
	if err := validateGameRoomInput(input); err != nil {
		return GameRoomView{}, err
	}
	passwordHash := ""
	if input.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), m.bcryptCost)
		if err != nil {
			return GameRoomView{}, err
		}
		passwordHash = string(hash)
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.purgeLocked(now)
	if existing := m.roomForUserLocked(owner.ID); existing != nil {
		return GameRoomView{}, ErrRoomAlreadyOpen
	}
	code, err := m.uniqueCodeLocked()
	if err != nil {
		return GameRoomView{}, err
	}
	room := &gameRoom{
		Code: code, Name: input.Name, HostUID: owner.ID, MaxPlayers: input.MaxPlayers,
		GameID: input.GameID,
		Rounds: input.Rounds, RoundSeconds: input.RoundSeconds, PasswordHash: passwordHash,
		RecordStats: input.RecordStats,
		Status:      "waiting", Players: []gameRoomPlayer{{UID: owner.ID, Username: displayName(owner), Host: true}},
		Strokes: []GameStroke{}, Messages: []GameRoomMessage{}, RoundScores: map[int64]int{}, TotalScores: map[int64]int{},
		CreatedAt: now, UpdatedAt: now,
	}
	m.rooms[code] = room
	return m.viewLocked(room, owner.ID), nil
}

func (m *GameRoomManager) Join(user domain.User, code, password string) (GameRoomView, error) {
	code = normalizeRoomCode(code)
	m.mu.Lock()
	defer m.mu.Unlock()
	room, err := m.roomLocked(code)
	if err != nil {
		return GameRoomView{}, err
	}
	if playerIndex(room, user.ID) >= 0 {
		return m.viewLocked(room, user.ID), nil
	}
	if existing := m.roomForUserLocked(user.ID); existing != nil {
		return GameRoomView{}, ErrRoomAlreadyOpen
	}
	if room.Status != "waiting" {
		return GameRoomView{}, ErrRoomState
	}
	if len(room.Players) >= room.MaxPlayers {
		return GameRoomView{}, ErrRoomFull
	}
	invited := m.invitations[user.ID] != nil
	if invited {
		_, invited = m.invitations[user.ID][code]
	}
	if room.PasswordHash != "" && !invited && bcrypt.CompareHashAndPassword([]byte(room.PasswordHash), []byte(password)) != nil {
		return GameRoomView{}, ErrRoomPassword
	}
	room.Players = append(room.Players, gameRoomPlayer{UID: user.ID, Username: displayName(user)})
	room.UpdatedAt = time.Now()
	if invited {
		delete(m.invitations[user.ID], code)
	}
	m.notifyLocked(code)
	return m.viewLocked(room, user.ID), nil
}

func (m *GameRoomManager) View(userID int64, code string) (GameRoomView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	room, err := m.roomLocked(normalizeRoomCode(code))
	if err != nil {
		return GameRoomView{}, err
	}
	if playerIndex(room, userID) < 0 {
		return GameRoomView{}, ErrRoomForbidden
	}
	m.finishExpiredRoundLocked(room)
	return m.viewLocked(room, userID), nil
}

func (m *GameRoomManager) Current(userID int64) (GameRoomView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	room := m.roomForUserLocked(userID)
	if room == nil {
		return GameRoomView{}, ErrRoomNotFound
	}
	m.finishExpiredRoundLocked(room)
	return m.viewLocked(room, userID), nil
}

func (m *GameRoomManager) Invite(actorID int64, actorName string, target domain.User, code string) error {
	code = normalizeRoomCode(code)
	m.mu.Lock()
	defer m.mu.Unlock()
	room, err := m.roomLocked(code)
	if err != nil {
		return err
	}
	if playerIndex(room, actorID) < 0 {
		return ErrRoomForbidden
	}
	if room.Status != "waiting" || target.ID == actorID || playerIndex(room, target.ID) >= 0 {
		return ErrRoomState
	}
	if m.invitations[target.ID] == nil {
		m.invitations[target.ID] = make(map[string]gameInvitation)
	}
	m.invitations[target.ID][code] = gameInvitation{Code: code, FromUID: actorID, FromUsername: actorName, CreatedAt: time.Now()}
	return nil
}

func (m *GameRoomManager) Invitations(userID int64) []GameInvitationView {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.purgeLocked(time.Now())
	items := make([]GameInvitationView, 0, len(m.invitations[userID]))
	for code, invitation := range m.invitations[userID] {
		room := m.rooms[code]
		if room == nil || room.Status != "waiting" {
			delete(m.invitations[userID], code)
			continue
		}
		items = append(items, GameInvitationView{
			Code: code, RoomName: room.Name, FromUID: invitation.FromUID, FromUsername: invitation.FromUsername,
			PasswordProtected: room.PasswordHash != "", CreatedAt: invitation.CreatedAt, GameID: room.GameID,
		})
	}
	return items
}

func (m *GameRoomManager) Ready(userID int64, code string, ready bool) (GameRoomView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	room, err := m.roomLocked(normalizeRoomCode(code))
	if err != nil {
		return GameRoomView{}, err
	}
	index := playerIndex(room, userID)
	if index < 0 {
		return GameRoomView{}, ErrRoomForbidden
	}
	if room.Status != "waiting" || room.Players[index].Bot {
		return GameRoomView{}, ErrRoomState
	}
	room.Players[index].Ready = ready
	room.UpdatedAt = time.Now()
	m.notifyLocked(room.Code)
	return m.viewLocked(room, userID), nil
}

func (m *GameRoomManager) AddPracticePlayer(userID int64, code string) (GameRoomView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	room, err := m.roomLocked(normalizeRoomCode(code))
	if err != nil {
		return GameRoomView{}, err
	}
	if room.HostUID != userID {
		return GameRoomView{}, ErrRoomForbidden
	}
	if room.Status != "waiting" || len(room.Players) >= room.MaxPlayers {
		return GameRoomView{}, ErrRoomState
	}
	botCount := 0
	for _, player := range room.Players {
		if player.Bot {
			botCount++
		}
	}
	botNames := map[string]string{DrawingGuessGame: "小屋画伴", DouDizhuGame: "小屋牌友", DaVinciGame: "密码助手"}
	name := botNames[room.GameID]
	if botCount > 0 {
		name = fmt.Sprintf("%s %d", name, botCount+1)
	}
	room.Players = append(room.Players, gameRoomPlayer{UID: int64(-1 - botCount), Username: name, Ready: true, Bot: true})
	room.UpdatedAt = time.Now()
	m.notifyLocked(room.Code)
	return m.viewLocked(room, userID), nil
}

func (m *GameRoomManager) Start(userID int64, code string) (GameRoomView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	room, err := m.roomLocked(normalizeRoomCode(code))
	if err != nil {
		return GameRoomView{}, err
	}
	if room.HostUID != userID {
		return GameRoomView{}, ErrRoomForbidden
	}
	if room.Status != "waiting" || len(room.Players) < 2 || !allPlayersReady(room) {
		return GameRoomView{}, ErrRoomState
	}
	room.CurrentRound = 1
	var startErr error
	switch room.GameID {
	case DrawingGuessGame:
		room.DrawerUID = firstRealPlayer(room)
		startErr = m.beginRoundLocked(room)
	case DouDizhuGame:
		startErr = m.startDouDizhuLocked(room)
	case DaVinciGame:
		startErr = m.startDaVinciLocked(room)
	default:
		startErr = errors.New("小游戏不存在")
	}
	if startErr != nil {
		return GameRoomView{}, startErr
	}
	m.notifyLocked(room.Code)
	return m.viewLocked(room, userID), nil
}

func (m *GameRoomManager) Action(userID int64, code string, action GameRoomAction) (GameRoomView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	room, err := m.roomLocked(normalizeRoomCode(code))
	if err != nil {
		return GameRoomView{}, err
	}
	if playerIndex(room, userID) < 0 {
		return GameRoomView{}, ErrRoomForbidden
	}
	switch room.GameID {
	case DouDizhuGame:
		err = m.douDizhuActionLocked(room, userID, action)
	case DaVinciGame:
		err = m.daVinciActionLocked(room, userID, action)
	default:
		err = errors.New("当前游戏不支持这个操作")
	}
	if err != nil {
		return GameRoomView{}, err
	}
	room.UpdatedAt = time.Now()
	m.notifyLocked(room.Code)
	return m.viewLocked(room, userID), nil
}

func (m *GameRoomManager) AddStroke(userID int64, code string, stroke GameStroke) (GameRoomView, error) {
	if stroke.Tool == "" {
		stroke.Tool = "pen"
	}
	if stroke.Opacity == 0 {
		stroke.Opacity = 1
	}
	if err := validateGameStroke(stroke); err != nil {
		return GameRoomView{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	room, err := m.drawingRoomLocked(userID, code)
	if err != nil {
		return GameRoomView{}, err
	}
	if room.DrawerUID != userID {
		return GameRoomView{}, ErrRoomForbidden
	}
	if room.DrawingLocked {
		return GameRoomView{}, ErrRoomState
	}
	room.NextStrokeID++
	stroke.ID = room.NextStrokeID
	room.Strokes = append(room.Strokes, stroke)
	room.UpdatedAt = time.Now()
	m.notifyLocked(room.Code)
	return m.viewLocked(room, userID), nil
}

func (m *GameRoomManager) UndoStroke(userID int64, code string) (GameRoomView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	room, err := m.drawingRoomLocked(userID, code)
	if err != nil {
		return GameRoomView{}, err
	}
	if room.DrawerUID != userID {
		return GameRoomView{}, ErrRoomForbidden
	}
	if room.DrawingLocked {
		return GameRoomView{}, ErrRoomState
	}
	if len(room.Strokes) > 0 {
		room.Strokes = room.Strokes[:len(room.Strokes)-1]
	}
	m.notifyLocked(room.Code)
	return m.viewLocked(room, userID), nil
}

func (m *GameRoomManager) ClearStrokes(userID int64, code string) (GameRoomView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	room, err := m.drawingRoomLocked(userID, code)
	if err != nil {
		return GameRoomView{}, err
	}
	if room.DrawerUID != userID {
		return GameRoomView{}, ErrRoomForbidden
	}
	if room.DrawingLocked {
		return GameRoomView{}, ErrRoomState
	}
	room.Strokes = []GameStroke{}
	m.notifyLocked(room.Code)
	return m.viewLocked(room, userID), nil
}

func (m *GameRoomManager) LockDrawing(userID int64, code string) (GameRoomView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	room, err := m.drawingRoomLocked(userID, code)
	if err != nil {
		return GameRoomView{}, err
	}
	if room.DrawerUID != userID {
		return GameRoomView{}, ErrRoomForbidden
	}
	if !room.DrawingLocked {
		room.DrawingLocked = true
		room.UpdatedAt = time.Now()
		m.appendMessageLocked(room, 0, "系统", "画手已完成作画，画板已锁定", "system")
		m.notifyLocked(room.Code)
	}
	return m.viewLocked(room, userID), nil
}

func (m *GameRoomManager) Message(user domain.User, code, text string) (GameRoomView, error) {
	text = strings.TrimSpace(text)
	if text == "" || utf8.RuneCountInString(text) > 60 {
		return GameRoomView{}, errors.New("消息应为 1 到 60 个字符")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	room, err := m.drawingRoomLocked(user.ID, code)
	if err != nil {
		return GameRoomView{}, err
	}
	kind := ""
	visibleText := text
	if user.ID != room.DrawerUID && room.Word.matches(text) {
		kind = "correct"
		visibleText = "猜中了：" + room.Word.Word
		m.completeRoundLocked(room, user.ID, "correct")
	}
	m.appendMessageLocked(room, user.ID, displayName(user), visibleText, kind)
	m.notifyLocked(room.Code)
	return m.viewLocked(room, user.ID), nil
}

func (m *GameRoomManager) FinishRound(userID int64, code string) (GameRoomView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	room, err := m.drawingRoomLocked(userID, code)
	if err != nil {
		return GameRoomView{}, err
	}
	if room.DrawerUID != userID && room.HostUID != userID {
		return GameRoomView{}, ErrRoomForbidden
	}
	reason := "complete"
	if m.secondsLeftLocked(room) <= 0 {
		reason = "timeout"
	}
	m.completeRoundLocked(room, 0, reason)
	m.notifyLocked(room.Code)
	return m.viewLocked(room, userID), nil
}

func (m *GameRoomManager) NextRound(userID int64, code string) (GameRoomView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	room, err := m.roomLocked(normalizeRoomCode(code))
	if err != nil {
		return GameRoomView{}, err
	}
	if room.HostUID != userID {
		return GameRoomView{}, ErrRoomForbidden
	}
	if room.Status != "round_complete" || room.CurrentRound >= room.Rounds {
		return GameRoomView{}, ErrRoomState
	}
	room.CurrentRound++
	var startErr error
	switch room.GameID {
	case DrawingGuessGame:
		room.DrawerUID = nextRealPlayer(room, room.DrawerUID)
		startErr = m.beginRoundLocked(room)
	case DouDizhuGame:
		startErr = m.startDouDizhuLocked(room)
	case DaVinciGame:
		startErr = m.startDaVinciLocked(room)
	default:
		startErr = errors.New("小游戏不存在")
	}
	if startErr != nil {
		return GameRoomView{}, startErr
	}
	m.notifyLocked(room.Code)
	return m.viewLocked(room, userID), nil
}

func (m *GameRoomManager) Leave(userID int64, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	room, err := m.roomLocked(normalizeRoomCode(code))
	if err != nil {
		return err
	}
	index := playerIndex(room, userID)
	if index < 0 {
		return ErrRoomForbidden
	}
	leftPlayer := room.Players[index]
	wasDrawer := room.DrawerUID == userID
	wasPlayingCardGame := room.Status == "playing" && (room.GameID == DouDizhuGame || room.GameID == DaVinciGame)
	room.Players = append(room.Players[:index], room.Players[index+1:]...)
	if countRealPlayers(room) == 0 {
		delete(m.rooms, room.Code)
		m.notifyLocked(room.Code)
		return nil
	}
	if room.HostUID == userID {
		room.HostUID = firstRealPlayer(room)
		for i := range room.Players {
			room.Players[i].Host = room.Players[i].UID == room.HostUID
		}
	}
	if wasDrawer && room.Status == "playing" {
		m.completeRoundLocked(room, 0, "complete")
	} else if wasPlayingCardGame {
		winnerUID := firstRealPlayer(room)
		if room.GameID == DouDizhuGame {
			m.completeDouDizhuLocked(room, winnerUID)
		} else {
			m.completeDaVinciLocked(room, winnerUID)
		}
	}
	m.appendMessageLocked(room, 0, "系统", leftPlayer.Username+" 已退出房间", "system")
	room.UpdatedAt = time.Now()
	m.notifyLocked(room.Code)
	return nil
}

func (m *GameRoomManager) Subscribe(userID int64, code string) (<-chan struct{}, func(), error) {
	code = normalizeRoomCode(code)
	m.mu.Lock()
	room, err := m.roomLocked(code)
	if err != nil {
		m.mu.Unlock()
		return nil, nil, err
	}
	if playerIndex(room, userID) < 0 {
		m.mu.Unlock()
		return nil, nil, ErrRoomForbidden
	}
	channel := make(chan struct{}, 1)
	if m.watchers[code] == nil {
		m.watchers[code] = make(map[chan struct{}]struct{})
	}
	m.watchers[code][channel] = struct{}{}
	m.mu.Unlock()
	cancel := func() {
		m.mu.Lock()
		delete(m.watchers[code], channel)
		m.mu.Unlock()
	}
	return channel, cancel, nil
}

func validateGameRoomInput(input CreateGameRoomInput) error {
	if !validGameKey(input.GameID) {
		return errors.New("小游戏不存在")
	}
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 24 {
		return errors.New("房间名称应为 1 到 24 个字符")
	}
	validPlayers := false
	switch input.GameID {
	case DrawingGuessGame:
		validPlayers = input.MaxPlayers >= 2 && input.MaxPlayers <= 8
	case DouDizhuGame:
		validPlayers = input.MaxPlayers == 3
	case DaVinciGame:
		validPlayers = input.MaxPlayers >= 2 && input.MaxPlayers <= 4
	}
	if !validPlayers {
		return errors.New("房间人数无效")
	}
	if input.Rounds != 1 && input.Rounds != 3 && input.Rounds != 5 && input.Rounds != 7 {
		return errors.New("回合数量无效")
	}
	if input.RoundSeconds != 30 && input.RoundSeconds != 45 && input.RoundSeconds != 60 && input.RoundSeconds != 90 && input.RoundSeconds != 120 && input.RoundSeconds != 180 {
		return errors.New("回合时间无效")
	}
	if len(input.Password) > 72 || utf8.RuneCountInString(input.Password) > 16 {
		return errors.New("房间密码不能超过 16 个字符")
	}
	return nil
}

func validateGameStroke(stroke GameStroke) error {
	if !roomColorPattern.MatchString(stroke.Color) || stroke.Width < 1 || stroke.Width > 48 || len(stroke.Points) < 2 || len(stroke.Points) > 2000 {
		return errors.New("画笔数据无效")
	}
	if stroke.Tool != "pen" && stroke.Tool != "marker" && stroke.Tool != "eraser" {
		return errors.New("画笔工具无效")
	}
	if stroke.Opacity < 0.1 || stroke.Opacity > 1 {
		return errors.New("画笔透明度无效")
	}
	for _, point := range stroke.Points {
		if math.IsNaN(point.X) || math.IsNaN(point.Y) || math.IsInf(point.X, 0) || math.IsInf(point.Y, 0) || point.X < 0 || point.X > 1 || point.Y < 0 || point.Y > 1 {
			return errors.New("画笔坐标无效")
		}
	}
	return nil
}

func (m *GameRoomManager) roomLocked(code string) (*gameRoom, error) {
	m.purgeLocked(time.Now())
	room := m.rooms[code]
	if room == nil {
		return nil, ErrRoomNotFound
	}
	return room, nil
}

func (m *GameRoomManager) playingRoomLocked(userID int64, code string) (*gameRoom, error) {
	room, err := m.roomLocked(normalizeRoomCode(code))
	if err != nil {
		return nil, err
	}
	if playerIndex(room, userID) < 0 {
		return nil, ErrRoomForbidden
	}
	m.finishExpiredRoundLocked(room)
	if room.Status != "playing" || room.RoundComplete {
		return nil, ErrRoomState
	}
	return room, nil
}

func (m *GameRoomManager) drawingRoomLocked(userID int64, code string) (*gameRoom, error) {
	room, err := m.playingRoomLocked(userID, code)
	if err != nil {
		return nil, err
	}
	if room.GameID != DrawingGuessGame {
		return nil, ErrRoomState
	}
	return room, nil
}

func (m *GameRoomManager) beginRoundLocked(room *gameRoom) error {
	word, err := randomDrawingWord()
	if err != nil {
		return err
	}
	room.Word = word
	room.Status = "playing"
	room.RoundStartedAt = time.Now()
	room.RoundComplete = false
	room.DrawingLocked = false
	room.SecondsLeftAtEnd = 0
	room.WinnerUID = 0
	room.RoundScores = map[int64]int{}
	room.Strokes = []GameStroke{}
	room.Messages = []GameRoomMessage{}
	room.NextStrokeID = 0
	room.NextMessageID = 0
	room.UpdatedAt = time.Now()
	m.appendMessageLocked(room, 0, "系统", "新回合已经开始", "system")
	return nil
}

func (m *GameRoomManager) completeRoundLocked(room *gameRoom, winnerUID int64, reason string) {
	if room.RoundComplete {
		return
	}
	remaining := m.secondsLeftLocked(room)
	room.Status = "round_complete"
	room.RoundComplete = true
	room.DrawingLocked = true
	room.SecondsLeftAtEnd = remaining
	room.WinnerUID = winnerUID
	room.RoundScores = make(map[int64]int)
	if room.TotalScores == nil {
		room.TotalScores = make(map[int64]int)
	}
	for _, player := range room.Players {
		if player.Bot {
			continue
		}
		score := 10
		if winnerUID != 0 {
			score = 20
			if player.UID == winnerUID {
				score = 100 + remaining*10
			} else if player.UID == room.DrawerUID {
				score = 60 + remaining*5
			}
		} else if player.UID == room.DrawerUID {
			score = 30
		}
		room.RoundScores[player.UID] = score
		room.TotalScores[player.UID] += score
	}
	room.UpdatedAt = time.Now()
	if reason == "correct" {
		m.appendMessageLocked(room, 0, "系统", "有人猜中了本回合题目", "system")
	} else if reason == "timeout" {
		m.appendMessageLocked(room, 0, "系统", "本回合时间到", "system")
	}
}

func (m *GameRoomManager) finishExpiredRoundLocked(room *gameRoom) {
	if room.GameID == DrawingGuessGame && room.Status == "playing" && m.secondsLeftLocked(room) <= 0 {
		m.completeRoundLocked(room, 0, "timeout")
		m.notifyLocked(room.Code)
	}
}

func (m *GameRoomManager) appendMessageLocked(room *gameRoom, uid int64, author, text, kind string) {
	room.NextMessageID++
	room.Messages = append(room.Messages, GameRoomMessage{ID: room.NextMessageID, AuthorUID: uid, Author: author, Text: text, Kind: kind, CreatedAt: time.Now()})
	if len(room.Messages) > 100 {
		room.Messages = room.Messages[len(room.Messages)-100:]
	}
}

func (m *GameRoomManager) viewLocked(room *gameRoom, userID int64) GameRoomView {
	players := make([]GameRoomPlayerView, 0, len(room.Players))
	for _, player := range room.Players {
		players = append(players, GameRoomPlayerView{UID: player.UID, Username: player.Username, Ready: player.Ready, Host: player.Host, Bot: player.Bot})
	}
	view := GameRoomView{
		Code: room.Code, Name: room.Name, GameID: room.GameID, HostUID: room.HostUID,
		MaxPlayers: room.MaxPlayers, Rounds: room.Rounds, RoundSeconds: room.RoundSeconds,
		PasswordProtected: room.PasswordHash != "", RecordStats: room.RecordStats, Status: room.Status, CurrentRound: room.CurrentRound,
		DrawerUID: room.DrawerUID, SecondsLeft: m.secondsLeftLocked(room), RoundComplete: room.RoundComplete,
		WinnerUID: room.WinnerUID, RoundScore: room.RoundScores[userID], TotalScore: room.TotalScores[userID], DrawingLocked: room.DrawingLocked, Players: players,
		Strokes: append([]GameStroke(nil), room.Strokes...), Messages: append([]GameRoomMessage(nil), room.Messages...),
	}
	if room.GameID == DouDizhuGame {
		view.DouDizhu = m.douDizhuViewLocked(room, userID)
	}
	if room.GameID == DaVinciGame {
		view.DaVinci = m.daVinciViewLocked(room, userID)
	}
	if room.GameID == DrawingGuessGame && room.CurrentRound > 0 {
		if userID == room.DrawerUID || room.RoundComplete {
			view.Word = room.Word.Word
			view.WordCategory = room.Word.Category
		} else {
			view.WordHint = room.Word.Hint
			view.WordLength = utf8.RuneCountInString(room.Word.Word)
		}
	}
	return view
}

func (m *GameRoomManager) secondsLeftLocked(room *gameRoom) int {
	if room.RoundComplete {
		return room.SecondsLeftAtEnd
	}
	if room.Status != "playing" || room.RoundStartedAt.IsZero() {
		return 0
	}
	return max(0, room.RoundSeconds-int(time.Since(room.RoundStartedAt).Seconds()))
}

func roomPlayerByUID(room *gameRoom, uid int64) *gameRoomPlayer {
	for index := range room.Players {
		if room.Players[index].UID == uid {
			return &room.Players[index]
		}
	}
	return nil
}

func nextRoomPlayerUID(room *gameRoom, current int64) int64 {
	index := playerIndex(room, current)
	if index < 0 || len(room.Players) == 0 {
		return 0
	}
	return room.Players[(index+1)%len(room.Players)].UID
}

func nowTime() time.Time { return time.Now() }

func (m *GameRoomManager) roomForUserLocked(userID int64) *gameRoom {
	for _, room := range m.rooms {
		if playerIndex(room, userID) >= 0 {
			return room
		}
	}
	return nil
}

func (m *GameRoomManager) uniqueCodeLocked() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	for attempt := 0; attempt < 20; attempt++ {
		buffer := make([]byte, 6)
		for i := range buffer {
			index, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
			if err != nil {
				return "", err
			}
			buffer[i] = alphabet[index.Int64()]
		}
		code := string(buffer)
		if m.rooms[code] == nil {
			return code, nil
		}
	}
	return "", fmt.Errorf("generate unique room code")
}

func (m *GameRoomManager) notifyLocked(code string) {
	for channel := range m.watchers[code] {
		select {
		case channel <- struct{}{}:
		default:
		}
	}
}

func (m *GameRoomManager) purgeLocked(now time.Time) {
	for code, room := range m.rooms {
		if now.Sub(room.UpdatedAt) > 12*time.Hour {
			delete(m.rooms, code)
			m.notifyLocked(code)
		}
	}
}

func playerIndex(room *gameRoom, userID int64) int {
	for index, player := range room.Players {
		if player.UID == userID {
			return index
		}
	}
	return -1
}

func allPlayersReady(room *gameRoom) bool {
	for _, player := range room.Players {
		if !player.Ready {
			return false
		}
	}
	return true
}

func firstRealPlayer(room *gameRoom) int64 {
	for _, player := range room.Players {
		if !player.Bot {
			return player.UID
		}
	}
	return 0
}

func nextRealPlayer(room *gameRoom, current int64) int64 {
	start := playerIndex(room, current)
	for offset := 1; offset <= len(room.Players); offset++ {
		player := room.Players[(start+offset)%len(room.Players)]
		if !player.Bot {
			return player.UID
		}
	}
	return current
}

func countRealPlayers(room *gameRoom) int {
	count := 0
	for _, player := range room.Players {
		if !player.Bot {
			count++
		}
	}
	return count
}

func displayName(user domain.User) string {
	if strings.TrimSpace(user.Username) != "" {
		return user.Username
	}
	return user.Account
}

func normalizeRoomCode(code string) string { return strings.ToUpper(strings.TrimSpace(code)) }
