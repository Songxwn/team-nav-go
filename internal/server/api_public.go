package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Songxwn/team-nav-go/internal/model"
	"github.com/Songxwn/team-nav-go/internal/store"
)

type bootstrapResponse struct {
	Site       map[string]string `json:"site"`
	Categories []*model.Category `json:"categories"`
	Cards      []model.Card      `json:"cards"`
	Notices    []model.Notice    `json:"notices"`
	User       *model.User       `json:"user,omitempty"`
	Stats      publicStats       `json:"stats"`
	Version    string            `json:"version"`
}

type publicStats struct {
	Cards      int   `json:"cards"`
	Categories int   `json:"categories"`
	Clicks     int64 `json:"clicks"`
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	site, err := s.store.Settings(ctx)
	if err != nil {
		s.internalError(w, err)
		return
	}
	cats, err := s.store.Categories(ctx, false)
	if err != nil {
		s.internalError(w, err)
		return
	}
	tree := buildTree(cats)
	visible := map[int64]bool{}
	for _, c := range cats {
		visible[c.ID] = true
	}
	cards, err := s.store.Cards(ctx, store.CardFilter{Status: "approved"})
	if err != nil {
		s.internalError(w, err)
		return
	}
	filtered := make([]model.Card, 0, len(cards))
	stats := publicStats{Categories: len(tree)}
	for _, c := range cards {
		if !visible[c.CategoryID] {
			continue
		}
		filtered = append(filtered, c)
		stats.Clicks += c.Clicks
	}
	stats.Cards = len(filtered)
	notices, err := s.store.Notices(ctx, true)
	if err != nil {
		s.internalError(w, err)
		return
	}
	resp := bootstrapResponse{
		Site:       site,
		Categories: tree,
		Cards:      filtered,
		Notices:    notices,
		User:       s.currentUser(r),
		Stats:      stats,
		Version:    s.version,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := s.store.Categories(r.Context(), false)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": buildTree(cats)})
}

func (s *Server) handleCards(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.CardFilter{
		Status: "approved",
		Query:  strings.TrimSpace(q.Get("q")),
		Limit:  500,
	}
	if raw := q.Get("categoryId"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "分类参数不正确")
			return
		}
		f.CategoryID = id
	}
	cards, err := s.store.Cards(r.Context(), f)
	if err != nil {
		s.internalError(w, err)
		return
	}
	visible := s.visibleCategories(r.Context())
	out := make([]model.Card, 0, len(cards))
	for _, c := range cards {
		if visible[c.CategoryID] {
			out = append(out, c)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"cards": out})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	term := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(term)) < 1 {
		writeJSON(w, http.StatusOK, map[string]any{"cards": []model.Card{}, "categories": []*model.Category{}})
		return
	}
	ctx := r.Context()
	visible := s.visibleCategories(ctx)
	cards, err := s.store.Cards(ctx, store.CardFilter{Status: "approved", Query: term, Limit: 50})
	if err != nil {
		s.internalError(w, err)
		return
	}
	outCards := make([]model.Card, 0, len(cards))
	for _, c := range cards {
		if visible[c.CategoryID] {
			outCards = append(outCards, c)
		}
	}
	cats, err := s.store.Categories(ctx, false)
	if err != nil {
		s.internalError(w, err)
		return
	}
	lower := strings.ToLower(term)
	outCats := make([]model.Category, 0)
	for _, c := range cats {
		if strings.Contains(strings.ToLower(c.Name), lower) || strings.Contains(strings.ToLower(c.Description), lower) {
			outCats = append(outCats, c)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"cards": outCards, "categories": outCats})
}

func (s *Server) handleClick(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "卡片编号不正确")
		return
	}
	if err := s.store.IncrementClick(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "卡片不存在")
			return
		}
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type applyRequest struct {
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	Description string   `json:"description"`
	CategoryID  int64    `json:"categoryId"`
	Tags        []string `json:"tags"`
}

func (s *Server) handleApply(w http.ResponseWriter, r *http.Request) {
	if !s.limiter.allow("apply:"+s.clientIP(r), 0.05, 3) {
		writeError(w, http.StatusTooManyRequests, "提交太频繁了，请稍后再试")
		return
	}
	ctx := r.Context()
	site, err := s.store.Settings(ctx)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if site["allowApply"] == "0" {
		writeError(w, http.StatusForbidden, "管理员已关闭卡片申请")
		return
	}
	var req applyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.URL = strings.TrimSpace(req.URL)
	if req.Name == "" || len([]rune(req.Name)) > 40 {
		writeError(w, http.StatusBadRequest, "请填写 1-40 个字符的名称")
		return
	}
	if !validHTTPURL(req.URL) {
		writeError(w, http.StatusBadRequest, "请填写合法的 http/https 地址")
		return
	}
	if !s.visibleCategories(ctx)[req.CategoryID] {
		writeError(w, http.StatusBadRequest, "请选择有效的分类")
		return
	}
	card := &model.Card{
		CategoryID:  req.CategoryID,
		Name:        req.Name,
		URL:         req.URL,
		Description: req.Description,
		IconType:    "text",
		Status:      "pending",
		Tags:        req.Tags,
	}
	if err := s.store.CreateCard(ctx, card); err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "message": "提交成功，等待管理员审核"})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.limiter.allow("login:"+s.clientIP(r), 0.1, 6) {
		writeError(w, http.StatusTooManyRequests, "尝试次数过多，请一分钟后再试")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	user, token, err := s.auth.Login(r.Context(), strings.TrimSpace(req.Username), req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "用户名或密码不正确")
		return
	}
	s.auth.SetCookie(w, token)
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.auth.Logout(r.Context(), w, r)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"user": s.currentUser(r)})
}

func (s *Server) visibleCategories(ctx context.Context) map[int64]bool {
	cats, err := s.store.Categories(ctx, false)
	if err != nil {
		return map[int64]bool{}
	}
	out := make(map[int64]bool, len(cats))
	for _, c := range cats {
		out[c.ID] = true
	}
	return out
}

func buildTree(cats []model.Category) []*model.Category {
	nodes := make(map[int64]*model.Category, len(cats))
	roots := make([]*model.Category, 0, 8)
	for i := range cats {
		c := cats[i]
		c.Children = []*model.Category{}
		node := &c
		nodes[c.ID] = node
		if c.ParentID == 0 {
			roots = append(roots, node)
			continue
		}
		if parent, ok := nodes[c.ParentID]; ok {
			parent.Children = append(parent.Children, node)
		}
	}
	return roots
}

func validHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// validIconValue keeps icon references constrained to local uploads or remote
// images over http(s).
func validIconValue(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return true
	}
	if strings.HasPrefix(v, "/api/images/") || strings.HasPrefix(v, "/assets/") {
		return true
	}
	return validHTTPURL(v)
}
