package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"sakura-happy-cottage/internal/config"
	"sakura-happy-cottage/internal/domain"
	"sakura-happy-cottage/internal/repository"
)

type MemoService struct {
	repo *repository.MemoRepository
	cfg  config.Config
}

func NewMemoService(repo *repository.MemoRepository, cfg config.Config) *MemoService {
	return &MemoService{repo: repo, cfg: cfg}
}
func (s *MemoService) List(ctx context.Context, userID int64, status, search string) ([]domain.Memo, error) {
	return s.repo.List(ctx, userID, status, strings.TrimSpace(search))
}
func (s *MemoService) Create(ctx context.Context, userID int64, title, description, status string, files []*multipart.FileHeader) (domain.Memo, error) {
	if err := validateMemo(title, description, status); err != nil {
		return domain.Memo{}, err
	}
	memo := domain.Memo{UserID: userID, Title: strings.TrimSpace(title), Description: strings.TrimSpace(description), Status: status}
	if err := s.repo.Create(ctx, &memo); err != nil {
		return domain.Memo{}, err
	}
	attachments, paths, err := s.saveFiles(files, memo.ID)
	if err != nil {
		_, _ = s.repo.Delete(ctx, memo.ID, userID)
		return domain.Memo{}, err
	}
	if err := s.repo.AddAttachments(ctx, attachments); err != nil {
		removeFiles(paths)
		_, _ = s.repo.Delete(ctx, memo.ID, userID)
		return domain.Memo{}, err
	}
	return s.repo.FindOwned(ctx, memo.ID, userID)
}
func (s *MemoService) Update(ctx context.Context, userID, id int64, title, description, status string) (domain.Memo, error) {
	if err := validateMemo(title, description, status); err != nil {
		return domain.Memo{}, err
	}
	memo, err := s.repo.FindOwned(ctx, id, userID)
	if err != nil {
		return domain.Memo{}, err
	}
	memo.Title = strings.TrimSpace(title)
	memo.Description = strings.TrimSpace(description)
	if memo.Status != status {
		memo.Status = status
		if status == "done" {
			now := time.Now()
			memo.CompletedAt = &now
		} else {
			memo.CompletedAt = nil
		}
	}
	if err := s.repo.Save(ctx, &memo); err != nil {
		return domain.Memo{}, err
	}
	return s.repo.FindOwned(ctx, id, userID)
}
func (s *MemoService) Delete(ctx context.Context, userID, id int64) error {
	memo, err := s.repo.FindOwned(ctx, id, userID)
	if err != nil {
		return err
	}
	deleted, err := s.repo.Delete(ctx, id, userID)
	if err != nil {
		return err
	}
	if !deleted {
		return gorm.ErrRecordNotFound
	}
	for _, a := range memo.Attachments {
		_ = os.Remove(filepath.Join(s.cfg.Storage.UploadDir, a.StoredName))
	}
	return nil
}
func (s *MemoService) Attachment(ctx context.Context, userID, id int64) (domain.Attachment, *os.File, error) {
	attachment, err := s.repo.AttachmentOwned(ctx, id, userID)
	if err != nil {
		return attachment, nil, err
	}
	file, err := os.Open(filepath.Join(s.cfg.Storage.UploadDir, attachment.StoredName))
	return attachment, file, err
}
func (s *MemoService) saveFiles(files []*multipart.FileHeader, memoID int64) ([]domain.Attachment, []string, error) {
	items := make([]domain.Attachment, 0, len(files))
	paths := make([]string, 0, len(files))
	for _, header := range files {
		name, err := uniqueName(header.Filename)
		if err != nil {
			removeFiles(paths)
			return nil, nil, err
		}
		target := filepath.Join(s.cfg.Storage.UploadDir, name)
		if err := saveUpload(header, target); err != nil {
			removeFiles(paths)
			return nil, nil, err
		}
		paths = append(paths, target)
		contentType := header.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		items = append(items, domain.Attachment{MemoID: memoID, OriginalName: filepath.Base(header.Filename), StoredName: name, ContentType: contentType, Size: header.Size})
	}
	return items, paths, nil
}
func validateMemo(title, description, status string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return errors.New("标题不能为空")
	}
	if utf8.RuneCountInString(title) > 120 {
		return errors.New("标题不能超过 120 个字符")
	}
	if utf8.RuneCountInString(description) > 5000 {
		return errors.New("具体描述不能超过 5000 个字符")
	}
	if status != "open" && status != "done" {
		return errors.New("状态无效")
	}
	return nil
}
func uniqueName(original string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(filepath.Base(original)))
	if len(ext) > 12 {
		ext = ""
	}
	return hex.EncodeToString(buf) + ext, nil
}
func saveUpload(header *multipart.FileHeader, target string) error {
	source, err := header.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()
	if copyErr != nil {
		_ = os.Remove(target)
		return copyErr
	}
	return closeErr
}
func removeFiles(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}
