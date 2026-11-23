package server

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Songxwn/team-nav-go/internal/auth"
	"github.com/Songxwn/team-nav-go/internal/model"
	"github.com/Songxwn/team-nav-go/internal/store"
)

func (s *Server) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats, err := s.store.Stats(ctx)
	if err != nil {
		s.internalError(w, err)
		return
	}
	cats, err := s.store.Categories(ctx, true)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"stats":      stats,
		"categories": cats,
		"version":    s.version,
	})
}

// ---------------------------------------------------------------------------
// Categories

type categoryInput struct {
	Name        string `json:"name"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
	ParentID    int64  `json:"parentId"`
	Sort        int    `json:"sort"`
	Visible     *bool  `json:"visible"`
}

func (s *Server) handleAdminCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := s.store.Categories(r.Context(), true)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": cats})
}

func (s *Server) handleAdminCreateCategory(w http.ResponseWriter, r *http.Request) {
	var in categoryInput
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 24 {
		writeError(w, http.StatusBadRequest, "分类名称需为 1-24 个字符")
		return
	}
	if utf8.RuneCountInString(in.Description) > 200 {
		writeError(w, http.StatusBadRequest, "分类描述不能超过 200 个字符")
		return
	}
	if err := s.checkParent(r, 0, in.ParentID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	c := &model.Category{
		Name:        in.Name,
		Icon:        in.Icon,
		Description: strings.TrimSpace(in.Description),
		ParentID:    in.ParentID,
		Sort:        in.Sort,
		Visible:     in.Visible == nil || *in.Visible,
	}
	if err := s.store.CreateCategory(r.Context(), c); err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"category": c})
}

func (s *Server) handleAdminUpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "分类编号不正确")
		return
	}
	var in categoryInput
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 24 {
		writeError(w, http.StatusBadRequest, "分类名称需为 1-24 个字符")
		return
	}
	if err := s.checkParent(r, id, in.ParentID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	c, err := s.store.Category(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "分类不存在")
			return
		}
		s.internalError(w, err)
		return
	}
	c.Name = in.Name
	c.Icon = in.Icon
	c.Description = strings.TrimSpace(in.Description)
	c.ParentID = in.ParentID
	c.Sort = in.Sort
	if in.Visible != nil {
		c.Visible = *in.Visible
	}
	if err := s.store.UpdateCategory(r.Context(), c); err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"category": c})
}

func (s *Server) handleAdminDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "分类编号不正确")
		return
	}
	if err := s.store.DeleteCategory(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "分类不存在")
			return
		}
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// checkParent validates that parentID can hold catID without creating a cycle
// or exceeding the three level limit.
func (s *Server) checkParent(r *http.Request, catID, parentID int64) error {
	if parentID == 0 {
		return nil
	}
	ctx := r.Context()
	if parentID == catID {
		return errors.New("不能将分类移动到自己下面")
	}
	parentDepth := 0
	for id := parentID; id != 0; {
		if id == catID {
			return errors.New("不能移动到自己的子分类下面")
		}
		c, err := s.store.Category(ctx, id)
		if err != nil {
			return errors.New("上级分类不存在")
		}
		parentDepth++
		if parentDepth > 8 {
			return errors.New("分类层级异常")
		}
		id = c.ParentID
	}
	cats, err := s.store.Categories(ctx, true)
	if err != nil {
		return errors.New("读取分类失败")
	}
	byParent := make(map[int64][]int64, len(cats))
	for _, c := range cats {
		byParent[c.ParentID] = append(byParent[c.ParentID], c.ID)
	}
	if catID != 0 && parentDepth+s.subtreeHeight(byParent, catID) > 3 {
		return errors.New("最多支持三级分类，请调整上级分类")
	}
	if catID == 0 && parentDepth+1 > 3 {
		return errors.New("最多支持三级分类")
	}
	return nil
}

func (s *Server) subtreeHeight(byParent map[int64][]int64, id int64) int {
	best := 0
	for _, child := range byParent[id] {
		if h := s.subtreeHeight(byParent, child); h > best {
			best = h
		}
	}
	return best + 1
}

// ---------------------------------------------------------------------------
// Cards

type cardInput struct {
	CategoryID  int64    `json:"categoryId"`
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	Description string   `json:"description"`
	IconType    string   `json:"iconType"`
	IconValue   string   `json:"iconValue"`
	Tags        []string `json:"tags"`
	Sort        int      `json:"sort"`
	Status      string   `json:"status"`
}

func (in *cardInput) validate(s *Server, r *http.Request) string {
	in.Name = strings.TrimSpace(in.Name)
	in.URL = strings.TrimSpace(in.URL)
	in.IconValue = strings.TrimSpace(in.IconValue)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 60 {
		return "卡片名称需为 1-60 个字符"
	}
	if in.URL != "" && !validHTTPURL(in.URL) {
		return "网址必须是合法的 http/https 地址"
	}
	if utf8.RuneCountInString(in.Description) > 300 {
		return "描述不能超过 300 个字符"
	}
	switch in.IconType {
	case "text", "emoji", "image", "url":
	case "":
		in.IconType = "text"
	default:
		return "图标类型不正确"
	}
	if (in.IconType == "image" || in.IconType == "url") && !validIconValue(in.IconValue) {
		return "图标地址必须是本站上传图片或 http/https 链接"
	}
	if utf8.RuneCountInString(in.IconValue) > 500 {
		return "图标内容过长"
	}
	if _, err := s.store.Category(r.Context(), in.CategoryID); err != nil {
		return "请选择存在的分类"
	}
	if in.Status == "" {
		in.Status = "approved"
	}
	if in.Status != "approved" && in.Status != "pending" {
		return "卡片状态不正确"
	}
	return ""
}

func (s *Server) handleAdminCards(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.CardFilter{
		Status: q.Get("status"),
		Query:  strings.TrimSpace(q.Get("q")),
	}
	if raw := q.Get("categoryId"); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
			f.CategoryID = id
		}
	}
	cards, err := s.store.Cards(r.Context(), f)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cards": cards})
}

func (s *Server) handleAdminCreateCard(w http.ResponseWriter, r *http.Request) {
	var in cardInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if msg := in.validate(s, r); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	c := &model.Card{
		CategoryID: in.CategoryID, Name: in.Name, URL: in.URL, Description: in.Description,
		IconType: in.IconType, IconValue: in.IconValue, Tags: in.Tags, Sort: in.Sort, Status: in.Status,
	}
	if err := s.store.CreateCard(r.Context(), c); err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"card": c})
}

func (s *Server) handleAdminUpdateCard(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "卡片编号不正确")
		return
	}
	var in cardInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if msg := in.validate(s, r); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	c, err := s.store.Card(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "卡片不存在")
			return
		}
		s.internalError(w, err)
		return
	}
	c.CategoryID, c.Name, c.URL, c.Description = in.CategoryID, in.Name, in.URL, in.Description
	c.IconType, c.IconValue, c.Tags, c.Sort, c.Status = in.IconType, in.IconValue, in.Tags, in.Sort, in.Status
	if err := s.store.UpdateCard(r.Context(), c); err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"card": c})
}

func (s *Server) handleAdminDeleteCard(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "卡片编号不正确")
		return
	}
	if err := s.store.DeleteCard(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "卡片不存在")
			return
		}
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAdminApproveCard(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "卡片编号不正确")
		return
	}
	c, err := s.store.Card(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "卡片不存在")
			return
		}
		s.internalError(w, err)
		return
	}
	c.Status = "approved"
	if err := s.store.UpdateCard(r.Context(), c); err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"card": c})
}

type sortRequest struct {
	Type       string  `json:"type"`
	ParentID   int64   `json:"parentId"`
	CategoryID int64   `json:"categoryId"`
	IDs        []int64 `json:"ids"`
}

func (s *Server) handleAdminSort(w http.ResponseWriter, r *http.Request) {
	var in sortRequest
	if !decodeJSON(w, r, &in) {
		return
	}
	if len(in.IDs) > 500 {
		writeError(w, http.StatusBadRequest, "排序数据过多")
		return
	}
	switch in.Type {
	case "category":
		if err := s.store.ReorderCategories(r.Context(), in.ParentID, in.IDs); err != nil {
			s.internalError(w, err)
			return
		}
	case "card":
		if err := s.store.ReorderCards(r.Context(), in.CategoryID, in.IDs); err != nil {
			s.internalError(w, err)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "排序类型不正确")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------------------------------------------------------------------------
// Notices

type noticeInput struct {
	Title     string `json:"title"`
	Content   string `json:"content"`
	Level     string `json:"level"`
	Link      string `json:"link"`
	Active    *bool  `json:"active"`
	Sort      int    `json:"sort"`
	ExpiresAt int64  `json:"expiresAt"`
}

func (in *noticeInput) validate() string {
	in.Title = strings.TrimSpace(in.Title)
	in.Content = strings.TrimSpace(in.Content)
	if in.Title == "" || utf8.RuneCountInString(in.Title) > 80 {
		return "公告标题需为 1-80 个字符"
	}
	if utf8.RuneCountInString(in.Content) > 2000 {
		return "公告内容不能超过 2000 个字符"
	}
	switch in.Level {
	case "info", "success", "warning", "danger":
	case "":
		in.Level = "info"
	default:
		return "公告级别不正确"
	}
	if in.Link != "" && !validHTTPURL(in.Link) {
		return "公告链接必须是 http/https 地址"
	}
	return ""
}

func (s *Server) handleAdminNotices(w http.ResponseWriter, r *http.Request) {
	notices, err := s.store.Notices(r.Context(), false)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notices": notices})
}

func (s *Server) handleAdminCreateNotice(w http.ResponseWriter, r *http.Request) {
	var in noticeInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	n := &model.Notice{Title: in.Title, Content: in.Content, Level: in.Level, Link: in.Link,
		Active: in.Active == nil || *in.Active, Sort: in.Sort, ExpiresAt: in.ExpiresAt}
	if err := s.store.CreateNotice(r.Context(), n); err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"notice": n})
}

func (s *Server) handleAdminUpdateNotice(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "公告编号不正确")
		return
	}
	var in noticeInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	n := &model.Notice{ID: id, Title: in.Title, Content: in.Content, Level: in.Level, Link: in.Link,
		Active: in.Active == nil || *in.Active, Sort: in.Sort, ExpiresAt: in.ExpiresAt}
	if err := s.store.UpdateNotice(r.Context(), n); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "公告不存在")
			return
		}
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notice": n})
}

func (s *Server) handleAdminDeleteNotice(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "公告编号不正确")
		return
	}
	if err := s.store.DeleteNotice(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "公告不存在")
			return
		}
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------------------------------------------------------------------------
// Users

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,32}$`)

type userInput struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}

func validPassword(pw string) bool {
	n := len(pw)
	return n >= 8 && n <= 72
}

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.Users(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func (s *Server) handleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	var in userInput
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if !usernamePattern.MatchString(in.Username) {
		writeError(w, http.StatusBadRequest, "用户名需为 3-32 位字母、数字或 _ . -")
		return
	}
	if !validPassword(in.Password) {
		writeError(w, http.StatusBadRequest, "密码长度需为 8-72 位")
		return
	}
	if in.Role == "" {
		in.Role = "member"
	}
	if in.Role != "admin" && in.Role != "member" {
		writeError(w, http.StatusBadRequest, "角色不正确")
		return
	}
	if in.DisplayName == "" {
		in.DisplayName = in.Username
	}
	if utf8.RuneCountInString(in.DisplayName) > 32 {
		writeError(w, http.StatusBadRequest, "显示名称不能超过 32 个字符")
		return
	}
	user, err := s.store.CreateUser(r.Context(), in.Username, in.Password, in.DisplayName, in.Role)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeError(w, http.StatusConflict, "用户名已存在")
			return
		}
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": user})
}

func (s *Server) handleAdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "用户编号不正确")
		return
	}
	var in userInput
	if !decodeJSON(w, r, &in) {
		return
	}
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if in.Role != "admin" && in.Role != "member" {
		writeError(w, http.StatusBadRequest, "角色不正确")
		return
	}
	if utf8.RuneCountInString(in.DisplayName) > 32 {
		writeError(w, http.StatusBadRequest, "显示名称不能超过 32 个字符")
		return
	}
	users, err := s.store.Users(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	var target, admins int
	for _, u := range users {
		if u.ID == id {
			target++
		}
		if u.Role == "admin" {
			admins++
		}
	}
	if target == 0 {
		writeError(w, http.StatusNotFound, "用户不存在")
		return
	}
	if in.Role == "member" && admins <= 1 {
		for _, u := range users {
			if u.ID == id && u.Role == "admin" {
				writeError(w, http.StatusBadRequest, "至少需要保留一名管理员")
				return
			}
		}
	}
	u := &model.User{ID: id, DisplayName: in.DisplayName, Role: in.Role}
	if err := s.store.UpdateUser(r.Context(), u); err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "用户编号不正确")
		return
	}
	if me := s.currentUser(r); me != nil && me.ID == id {
		writeError(w, http.StatusBadRequest, "不能删除当前登录的账号")
		return
	}
	users, err := s.store.Users(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	admins := 0
	for _, u := range users {
		if u.Role == "admin" {
			admins++
		}
	}
	for _, u := range users {
		if u.ID == id && u.Role == "admin" && admins <= 1 {
			writeError(w, http.StatusBadRequest, "至少需要保留一名管理员")
			return
		}
	}
	if err := s.store.DeleteUser(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "用户不存在")
			return
		}
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAdminResetPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "用户编号不正确")
		return
	}
	target, err := s.store.UserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "用户不存在")
			return
		}
		s.internalError(w, err)
		return
	}
	if target.IsExternal() {
		writeError(w, http.StatusBadRequest, "外部账号请在身份提供方修改密码")
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if !validPassword(in.Password) {
		writeError(w, http.StatusBadRequest, "密码长度需为 8-72 位")
		return
	}
	if err := s.store.SetPassword(r.Context(), id, in.Password); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "用户不存在")
			return
		}
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAdminOwnPassword(w http.ResponseWriter, r *http.Request) {
	me := s.currentUser(r)
	if me == nil {
		writeError(w, http.StatusUnauthorized, "请先登录")
		return
	}
	if me.IsExternal() {
		writeError(w, http.StatusBadRequest, "外部账号请在身份提供方修改密码")
		return
	}
	var in struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if !validPassword(in.NewPassword) {
		writeError(w, http.StatusBadRequest, "新密码长度需为 8-72 位")
		return
	}
	_, hash, err := s.store.UserByUsername(r.Context(), me.Username)
	if err != nil || !auth.CheckPassword(hash, in.CurrentPassword) {
		writeError(w, http.StatusBadRequest, "当前密码不正确")
		return
	}
	if err := s.store.SetPassword(r.Context(), me.ID, in.NewPassword); err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------------------------------------------------------------------------
// Settings

var editableSettings = map[string]bool{
	"siteName":       true,
	"siteSubtitle":   true,
	"siteLogo":       true,
	"footerText":     true,
	"welcomeText":    true,
	"allowApply":     true,
	"showClicks":     true,
	"noticeInterval": true,
	"sessionTTL":     true,
	"accent":         true,
	"language":       true,

	"ldapEnabled":         true,
	"ldapAddr":            true,
	"ldapStartTLS":        true,
	"ldapSkipVerify":      true,
	"ldapBindDN":          true,
	"ldapBindPassword":    true,
	"ldapBaseDN":          true,
	"ldapUserFilter":      true,
	"ldapUserDN":          true,
	"ldapUsernameAttr":    true,
	"ldapDisplayNameAttr": true,
	"ldapAdminGroup":      true,
	"ldapDefaultRole":     true,
	"ldapAutoCreate":      true,

	"oidcEnabled":          true,
	"oidcName":             true,
	"oidcIssuer":           true,
	"oidcClientID":         true,
	"oidcClientSecret":     true,
	"oidcRedirectURL":      true,
	"oidcScopes":           true,
	"oidcUsernameClaim":    true,
	"oidcDisplayNameClaim": true,
	"oidcGroupsClaim":      true,
	"oidcAdminGroup":       true,
	"oidcDefaultRole":      true,
	"oidcAutoCreate":       true,
}

// secretMask is returned instead of stored secrets. Submitting it unchanged
// keeps the current value; submitting an empty string clears it.
const secretMask = "********"

var secretSettings = map[string]bool{
	"ldapBindPassword": true,
	"oidcClientSecret": true,
}

func adminVisibleSettings(all map[string]string) map[string]string {
	out := make(map[string]string, len(all))
	for k, v := range all {
		out[k] = v
	}
	for key := range secretSettings {
		if out[key] != "" {
			out[key] = secretMask
		}
	}
	return out
}

func (s *Server) handleAdminSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.Settings(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": adminVisibleSettings(settings)})
}

func (s *Server) handleAdminUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var in map[string]string
	if !decodeJSON(w, r, &in) {
		return
	}
	authTouched := false
	for k, v := range in {
		if !editableSettings[k] {
			delete(in, k)
			continue
		}
		if secretSettings[k] && v == secretMask {
			delete(in, k)
			continue
		}
		if utf8.RuneCountInString(v) > 300 {
			writeError(w, http.StatusBadRequest, "设置内容过长")
			return
		}
		if strings.HasPrefix(k, "ldap") || strings.HasPrefix(k, "oidc") {
			authTouched = true
		}
		switch k {
		case "allowApply", "showClicks",
			"ldapEnabled", "ldapStartTLS", "ldapSkipVerify", "ldapAutoCreate",
			"oidcEnabled", "oidcAutoCreate":
			if v != "0" && v != "1" {
				writeError(w, http.StatusBadRequest, "开关取值不正确")
				return
			}
		case "noticeInterval":
			n, err := strconv.Atoi(v)
			if err != nil || n < 3 || n > 120 {
				writeError(w, http.StatusBadRequest, "公告切换间隔需为 3-120 秒")
				return
			}
		case "sessionTTL":
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > 365 {
				writeError(w, http.StatusBadRequest, "会话有效期需为 1-365 天")
				return
			}
		case "accent":
			switch v {
			case "indigo", "emerald", "rose", "amber", "ocean":
			default:
				writeError(w, http.StatusBadRequest, "主题色不正确")
				return
			}
		case "language":
			if v != "zh" && v != "en" {
				writeError(w, http.StatusBadRequest, "语言取值不正确")
				return
			}
		case "ldapDefaultRole", "oidcDefaultRole":
			if v != "member" && v != "admin" {
				writeError(w, http.StatusBadRequest, "角色不正确")
				return
			}
		case "siteLogo":
			if !validIconValue(v) {
				writeError(w, http.StatusBadRequest, "LOGO 地址不正确")
				return
			}
		}
	}
	if authTouched {
		merged, err := s.store.Settings(r.Context())
		if err != nil {
			s.internalError(w, err)
			return
		}
		for k, v := range in {
			merged[k] = v
		}
		if err := auth.LDAPConfigFromSettings(merged).Validate(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := auth.OIDCConfigFromSettings(merged).Validate(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if err := s.store.UpdateSettings(r.Context(), in); err != nil {
		s.internalError(w, err)
		return
	}
	settings, err := s.store.Settings(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": adminVisibleSettings(settings)})
}
