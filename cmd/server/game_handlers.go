package main

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"gorm.io/gorm"
	"sakura-happy-cottage/internal/domain"
	"sakura-happy-cottage/internal/service"
)

func (a *App) handleGameStats(w http.ResponseWriter, r *http.Request, id identity) {
	stats, err := a.games.Stats(r.Context(), id.User.ID, r.URL.Query().Get("game"))
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stats": stats})
}

func (a *App) handleRecordGameMatch(w http.ResponseWriter, r *http.Request, id identity) {
	var match domain.GameMatch
	if err := decodeJSON(r, &match); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	stats, err := a.games.Record(r.Context(), id.User.ID, match)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"stats": stats})
}

func (a *App) handleGameSession(w http.ResponseWriter, r *http.Request, id identity) {
	detail, err := a.games.Session(r.Context(), id.User.ID, r.URL.Query().Get("game"), r.PathValue("code"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeError(w, http.StatusNotFound, "对局不存在")
		return
	}
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session": detail})
}

func (a *App) handleCreateGameRoom(w http.ResponseWriter, r *http.Request, id identity) {
	var input struct {
		GameID       string `json:"game_id"`
		Name         string `json:"name"`
		MaxPlayers   int    `json:"max_players"`
		Rounds       int    `json:"rounds"`
		RoundSeconds int    `json:"round_seconds"`
		Password     string `json:"password"`
		RecordStats  *bool  `json:"record_stats"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	recordStats := true
	if input.RecordStats != nil {
		recordStats = *input.RecordStats
	}
	room, err := a.rooms.Create(id.User, service.CreateGameRoomInput{
		GameID: input.GameID, Name: input.Name, MaxPlayers: input.MaxPlayers, Rounds: input.Rounds,
		RoundSeconds: input.RoundSeconds, Password: input.Password, RecordStats: recordStats,
	})
	if err != nil {
		handleGameRoomError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"room": room})
}

func (a *App) handleGameRoomAction(w http.ResponseWriter, r *http.Request, id identity) {
	var action service.GameRoomAction
	if err := decodeJSON(r, &action); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	room, err := a.rooms.Action(id.User.ID, r.PathValue("code"), action)
	writeGameRoomResult(w, room, err)
}

func (a *App) handleJoinGameRoom(w http.ResponseWriter, r *http.Request, id identity) {
	var input struct {
		Code     string `json:"code"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	room, err := a.rooms.Join(id.User, input.Code, input.Password)
	if err != nil {
		handleGameRoomError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"room": room})
}

func (a *App) handleGetGameRoom(w http.ResponseWriter, r *http.Request, id identity) {
	room, err := a.rooms.View(id.User.ID, r.PathValue("code"))
	if err != nil {
		handleGameRoomError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"room": room})
}

func (a *App) handleCurrentGameRoom(w http.ResponseWriter, _ *http.Request, id identity) {
	room, err := a.rooms.Current(id.User.ID)
	if err != nil {
		handleGameRoomError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"room": room})
}

func (a *App) handleLeaveGameRoom(w http.ResponseWriter, r *http.Request, id identity) {
	if err := a.rooms.Leave(id.User.ID, r.PathValue("code")); err != nil {
		handleGameRoomError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleInviteGameRoom(w http.ResponseWriter, r *http.Request, id identity) {
	var input struct {
		UID int64 `json:"uid"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	target, err := a.users.FindByID(r.Context(), input.UID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "用户不存在")
			return
		}
		writeInternalError(w, err)
		return
	}
	if err := a.rooms.Invite(id.User.ID, id.User.Username, target, r.PathValue("code")); err != nil {
		handleGameRoomError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "邀请已发送"})
}

func (a *App) handleGameInvitations(w http.ResponseWriter, _ *http.Request, id identity) {
	writeJSON(w, http.StatusOK, map[string]any{"invitations": a.rooms.Invitations(id.User.ID)})
}

func (a *App) handleReadyGameRoom(w http.ResponseWriter, r *http.Request, id identity) {
	var input struct {
		Ready bool `json:"ready"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	room, err := a.rooms.Ready(id.User.ID, r.PathValue("code"), input.Ready)
	writeGameRoomResult(w, room, err)
}

func (a *App) handleAddPracticePlayer(w http.ResponseWriter, r *http.Request, id identity) {
	room, err := a.rooms.AddPracticePlayer(id.User.ID, r.PathValue("code"))
	writeGameRoomResult(w, room, err)
}

func (a *App) handleStartGameRoom(w http.ResponseWriter, r *http.Request, id identity) {
	room, err := a.rooms.Start(id.User.ID, r.PathValue("code"))
	writeGameRoomResult(w, room, err)
}

func (a *App) handleGameRoomStroke(w http.ResponseWriter, r *http.Request, id identity) {
	var stroke service.GameStroke
	if err := decodeJSON(r, &stroke); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	room, err := a.rooms.AddStroke(id.User.ID, r.PathValue("code"), stroke)
	writeGameRoomResult(w, room, err)
}

func (a *App) handleUndoGameRoomStroke(w http.ResponseWriter, r *http.Request, id identity) {
	room, err := a.rooms.UndoStroke(id.User.ID, r.PathValue("code"))
	writeGameRoomResult(w, room, err)
}

func (a *App) handleClearGameRoomStrokes(w http.ResponseWriter, r *http.Request, id identity) {
	room, err := a.rooms.ClearStrokes(id.User.ID, r.PathValue("code"))
	writeGameRoomResult(w, room, err)
}

func (a *App) handleGameRoomMessage(w http.ResponseWriter, r *http.Request, id identity) {
	var input struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	room, err := a.rooms.Message(id.User, r.PathValue("code"), input.Text)
	writeGameRoomResult(w, room, err)
}

func (a *App) handleFinishGameRoomRound(w http.ResponseWriter, r *http.Request, id identity) {
	room, err := a.rooms.FinishRound(id.User.ID, r.PathValue("code"))
	writeGameRoomResult(w, room, err)
}

func (a *App) handleLockGameRoomDrawing(w http.ResponseWriter, r *http.Request, id identity) {
	room, err := a.rooms.LockDrawing(id.User.ID, r.PathValue("code"))
	writeGameRoomResult(w, room, err)
}

func (a *App) handleNextGameRoomRound(w http.ResponseWriter, r *http.Request, id identity) {
	room, err := a.rooms.NextRound(id.User.ID, r.PathValue("code"))
	writeGameRoomResult(w, room, err)
}

func (a *App) handleGameRoomEvents(w http.ResponseWriter, r *http.Request, id identity) {
	events, cancel, err := a.rooms.Subscribe(id.User.ID, r.PathValue("code"))
	if err != nil {
		handleGameRoomError(w, err)
		return
	}
	defer cancel()
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "服务器不支持房间事件")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	writeRoomEvent(w, flusher, "room")
	keepAlive := time.NewTicker(20 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-events:
			writeRoomEvent(w, flusher, "room")
		case <-keepAlive.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeRoomEvent(w http.ResponseWriter, flusher http.Flusher, event string) {
	_, _ = fmt.Fprintf(w, "event: %s\ndata: {}\n\n", event)
	flusher.Flush()
}

func writeGameRoomResult(w http.ResponseWriter, room service.GameRoomView, err error) {
	if err != nil {
		handleGameRoomError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"room": room})
}

func handleGameRoomError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrRoomNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrRoomForbidden), errors.Is(err, service.ErrRoomPassword):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrRoomFull), errors.Is(err, service.ErrRoomAlreadyOpen), errors.Is(err, service.ErrRoomState):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}
