package main

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"strings"

	"gorm.io/gorm"
	"sakura-happy-cottage/internal/domain"
)

func (a *App) handleListMemos(w http.ResponseWriter, r *http.Request, id identity) {
	memos, err := a.memos.List(r.Context(), id.User.ID, r.URL.Query().Get("status"), r.URL.Query().Get("q"))
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if memos == nil {
		memos = []domain.Memo{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"memos": memos, "count": len(memos)})
}

func (a *App) handleCreateMemo(w http.ResponseWriter, r *http.Request, id identity) {
	r.Body = http.MaxBytesReader(w, r.Body, a.cfg.Storage.MaxUploadBytes+(1<<20))
	if err := r.ParseMultipartForm(a.cfg.Storage.MaxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "提交内容无效或附件总大小超过限制")
		return
	}
	files := r.MultipartForm.File["attachments"]
	var total int64
	for _, file := range files {
		total += file.Size
	}
	if total > a.cfg.Storage.MaxUploadBytes {
		writeError(w, http.StatusBadRequest, "附件总大小超过限制")
		return
	}
	memo, err := a.memos.Create(r.Context(), id.User.ID, r.FormValue("title"), r.FormValue("description"), r.FormValue("status"), files)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, memo)
}

func (a *App) handleUpdateMemo(w http.ResponseWriter, r *http.Request, id identity) {
	memoID, err := pathID(r, "id")
	if err != nil {
		handleServiceError(w, err)
		return
	}
	var input struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}
	if err := decodeJSON(r, &input); err != nil {
		handleServiceError(w, err)
		return
	}
	memo, err := a.memos.Update(r.Context(), id.User.ID, memoID, input.Title, input.Description, input.Status)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, memo)
}

func (a *App) handleDeleteMemo(w http.ResponseWriter, r *http.Request, id identity) {
	memoID, err := pathID(r, "id")
	if err != nil {
		handleServiceError(w, err)
		return
	}
	if err := a.memos.Delete(r.Context(), id.User.ID, memoID); err != nil {
		handleServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleAttachment(w http.ResponseWriter, r *http.Request, id identity) {
	attachmentID, err := pathID(r, "id")
	if err != nil {
		handleServiceError(w, err)
		return
	}
	attachment, file, err := a.memos.Attachment(r.Context(), id.User.ID, attachmentID)
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, os.ErrNotExist) {
		writeError(w, http.StatusNotFound, "附件不存在")
		return
	}
	if err != nil {
		writeInternalError(w, err)
		return
	}
	defer file.Close()
	disposition := "attachment"
	if r.URL.Query().Get("inline") == "1" && safeInlineImage(attachment.ContentType) {
		disposition = "inline"
	}
	w.Header().Set("Content-Type", attachment.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": attachment.OriginalName}))
	http.ServeContent(w, r, attachment.OriginalName, attachment.CreatedAt, file)
}

func safeInlineImage(contentType string) bool {
	switch strings.ToLower(contentType) {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}
