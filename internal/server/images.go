package server

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Songxwn/team-nav-go/internal/imgcache"
	"github.com/Songxwn/team-nav-go/internal/store"
)

const maxImageBytes = 2 << 20

var allowedImageTypes = map[string]string{
	"image/png":                "png",
	"image/jpeg":               "jpg",
	"image/gif":                "gif",
	"image/webp":               "webp",
	"image/x-icon":             "ico",
	"image/vnd.microsoft.icon": "ico",
	"image/avif":               "avif",
}

// handleImage serves an uploaded asset from the in-memory cache, falling back
// to SQLite. Responses are immutable so browsers and proxies cache them
// aggressively while ETag revalidation stays available.
func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || len(id) > 64 {
		writeError(w, http.StatusBadRequest, "图片编号不正确")
		return
	}
	entry, hit := s.cache.Get(id)
	if !hit {
		img, data, err := s.store.ImageBytes(r.Context(), id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			s.internalError(w, err)
			return
		}
		etag := `"` + img.SHA256[:24] + `"`
		entry = &imgcache.Entry{ID: img.ID, Mime: img.Mime, Data: data, ETag: etag, Size: int64(len(data))}
		s.cache.Put(entry)
	}
	if inm := r.Header.Get("If-None-Match"); inm != "" && strings.Contains(inm, entry.ETag) {
		w.Header().Set("ETag", entry.ETag)
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h := w.Header()
	h.Set("Content-Type", entry.Mime)
	h.Set("Content-Length", strconv.Itoa(len(entry.Data)))
	h.Set("Cache-Control", "public, max-age=31536000, immutable")
	h.Set("ETag", entry.ETag)
	if hit {
		h.Set("X-Image-Cache", "HIT")
	} else {
		h.Set("X-Image-Cache", "MISS")
	}
	h.Set("Content-Disposition", "inline")
	_, _ = w.Write(entry.Data)
}

// handleAdminUploadImage accepts a single multipart image, de-duplicates it by
// content hash and returns its public URL.
func (s *Server) handleAdminUploadImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
	if err := r.ParseMultipartForm(6 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "上传内容过大或格式不正确")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "请选择要上传的图片")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
	if err != nil {
		s.internalError(w, err)
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusBadRequest, "图片内容为空")
		return
	}
	if len(data) > maxImageBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "图片不能超过 2MB")
		return
	}
	mime := http.DetectContentType(data)
	if _, ok := allowedImageTypes[mime]; !ok {
		writeError(w, http.StatusUnsupportedMediaType, "仅支持 PNG、JPEG、GIF、WebP、AVIF、ICO 格式")
		return
	}
	img, created, err := s.store.SaveImage(r.Context(), header.Filename, mime, newID(), data)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"image":     img,
		"url":       "/api/images/" + img.ID,
		"deduped":   !created,
		"cacheHint": "immutable",
	})
}

func (s *Server) handleAdminImages(w http.ResponseWriter, r *http.Request) {
	images, err := s.store.Images(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"images": images})
}

func (s *Server) handleAdminDeleteImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteImage(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "图片不存在")
			return
		}
		s.internalError(w, err)
		return
	}
	s.cache.Delete(id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAdminBackup(w http.ResponseWriter, r *http.Request) {
	file, err := s.store.Backup(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="team-nav-backup.db"`)
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, file)
}
