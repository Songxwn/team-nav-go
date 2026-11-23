/* global fetch, FormData, localStorage */
(function () {
  "use strict";

  var $ = function (sel, root) { return (root || document).querySelector(sel); };
  var $$ = function (sel, root) { return Array.prototype.slice.call((root || document).querySelectorAll(sel)); };

  var state = {
    user: null,
    version: "",
    stats: {},
    categories: [],
    cards: [],
    notices: [],
    users: [],
    images: [],
    settings: {},
    view: "overview",
    cardFilters: { q: "", status: "", categoryId: "" },
    confirmAction: null,
    viewTitles: {
      overview: ["概览", "概览"],
      cards: ["卡片管理", "卡片管理"],
      categories: ["分类管理", "分类管理"],
      notices: ["通知公告", "通知公告"],
      users: ["用户与权限", "用户与权限"],
      images: ["图片库", "图片库"],
      settings: ["系统设置", "系统设置"]
    }
  };

  // -------------------------------------------------------------------------
  // Helpers

  function esc(value) {
    return String(value == null ? "" : value)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#39;");
  }

  function icon(name, size) {
    return '<svg class="icon"' + (size ? ' style="width:' + size + "px;height:" + size + 'px"' : "") +
      '><use href="#i-' + name + '"/></svg>';
  }

  function toast(message, kind) {
    var node = document.createElement("div");
    node.className = "toast " + (kind || "");
    node.innerHTML = '<span class="toast-dot"></span><span></span>';
    node.lastElementChild.textContent = message;
    $("#toasts").appendChild(node);
    window.setTimeout(function () {
      node.classList.add("out");
      window.setTimeout(function () { node.remove(); }, 260);
    }, 3400);
  }

  async function api(path, options) {
    var opts = options || {};
    var headers = Object.assign({ Accept: "application/json" }, opts.headers || {});
    if (opts.body && !(opts.body instanceof FormData)) headers["Content-Type"] = "application/json";
    var res = await fetch(path, Object.assign({}, opts, { headers: headers }));
    if (res.status === 401) {
      window.location.href = "/?login=1";
      throw new Error("需要登录");
    }
    var data = await res.json().catch(function () { return {}; });
    if (!res.ok) throw new Error(data.error || "请求失败（" + res.status + "）");
    return data;
  }

  function formatDate(sec) {
    if (!sec) return "—";
    var d = new Date(sec * 1000);
    return d.getFullYear() + "-" + pad(d.getMonth() + 1) + "-" + pad(d.getDate());
  }

  function formatDateTime(sec) {
    if (!sec) return "—";
    var d = new Date(sec * 1000);
    return formatDate(sec) + " " + pad(d.getHours()) + ":" + pad(d.getMinutes());
  }

  function pad(n) { return n < 10 ? "0" + n : String(n); }

  function formatBytes(bytes) {
    if (!bytes) return "0 B";
    if (bytes < 1024) return bytes + " B";
    if (bytes < 1048576) return (bytes / 1024).toFixed(1) + " KB";
    return (bytes / 1048576).toFixed(1) + " MB";
  }

  function toneFor(text) {
    var palette = ["#6366f1", "#8b5cf6", "#0ea5e9", "#14b8a6", "#f59e0b", "#ef4444", "#ec4899", "#10b981", "#0284c7", "#7c3aed"];
    var hash = 0;
    var str = String(text || "");
    for (var i = 0; i < str.length; i++) hash = (hash * 31 + str.charCodeAt(i)) >>> 0;
    return palette[hash % palette.length];
  }

  function categoryMap() {
    var map = new Map();
    state.categories.forEach(function (cat) { map.set(cat.id, cat); });
    return map;
  }

  function orderedCategories() {
    var map = categoryMap();
    var byParent = new Map();
    state.categories.forEach(function (cat) {
      if (!byParent.has(cat.parentId)) byParent.set(cat.parentId, []);
      byParent.get(cat.parentId).push(cat);
    });
    byParent.forEach(function (list) {
      list.sort(function (a, b) { return a.sort - b.sort || a.id - b.id; });
    });
    var out = [];
    function walk(parent, depth) {
      (byParent.get(parent) || []).forEach(function (cat) {
        cat._depth = depth;
        out.push(cat);
        walk(cat.id, depth + 1);
      });
    }
    walk(0, 0);
    return out;
  }

  function pathOf(id) {
    var map = categoryMap();
    var node = map.get(id);
    if (!node) return "未分类";
    var parts = [node.name];
    var parent = map.get(node.parentId);
    while (parent) {
      parts.unshift(parent.name);
      parent = map.get(parent.parentId);
    }
    return parts.join(" / ");
  }

  function isImageIcon(card) {
    return (card.iconType === "image" || card.iconType === "url") && card.iconValue;
  }

  function cardIcon(card) {
    if (isImageIcon(card)) {
      return '<span class="cell-icon" style="--tone:' + toneFor(card.name) + '"><img src="' +
        esc(card.iconValue) + '" alt=""></span>';
    }
    var content = card.iconType === "emoji" && card.iconValue ? card.iconValue :
      (card.iconType === "text" && card.iconValue ? card.iconValue :
        (Array.from(card.name || "?")[0] || "?"));
    return '<span class="cell-icon" style="background:linear-gradient(140deg,' + toneFor(card.name) +
      ",#0f172a)\">" + esc(content) + "</span>";
  }

  function iconPreview(node, card) {
    if (!node) return;
    if (isImageIcon(card)) {
      node.innerHTML = '<img src="' + esc(card.iconValue) + '" alt="">';
      node.style.background = "linear-gradient(140deg," + toneFor(card.name) + ",#0f172a)";
      return;
    }
    var value = card.iconType === "emoji" && card.iconValue ? card.iconValue :
      (card.iconType === "text" && card.iconValue ? card.iconValue : (Array.from(card.name || "?")[0] || "?"));
    node.textContent = value;
    node.style.background = "linear-gradient(140deg," + toneFor(card.name) + ",#0f172a)";
  }

  function openDialog(id) {
    var dialog = document.getElementById(id);
    if (dialog) dialog.showModal();
  }

  function closeDialog(id) {
    var dialog = document.getElementById(id);
    if (dialog) dialog.close();
  }

  function formError(id, message) {
    var node = $("#" + id);
    node.textContent = message || "";
    node.classList.toggle("show", Boolean(message));
  }

  function confirmAction(title, text, action) {
    $("#confirmTitle").textContent = title;
    $("#confirmText").textContent = text;
    state.confirmAction = action;
    openDialog("confirmDialog");
  }

  // -------------------------------------------------------------------------
  // Init

  async function init() {
    bindEvents();
    try {
      var me = await api("/api/auth/me");
      if (!me.user || me.user.role !== "admin") {
        window.location.href = "/?login=1";
        return;
      }
      state.user = me.user;
      $("#adminName").textContent = me.user.displayName || me.user.username;
      $("#adminRole").textContent = me.user.role;
      $("#adminAvatar").textContent = (me.user.displayName || me.user.username).slice(0, 1).toUpperCase();
      await loadAll();
    } catch (err) {
      toast(err.message, "error");
    }
  }

  async function loadAll() {
    var results = await Promise.all([
      api("/api/admin/overview"),
      api("/api/admin/cards"),
      api("/api/admin/notices"),
      api("/api/admin/users"),
      api("/api/admin/images"),
      api("/api/admin/settings")
    ]);
    state.stats = results[0].stats || {};
    state.categories = results[0].categories || [];
    state.version = results[0].version || "dev";
    state.cards = results[1].cards || [];
    state.notices = results[2].notices || [];
    state.users = results[3].users || [];
    state.images = results[4].images || [];
    state.settings = results[5].settings || {};
    applySiteChrome();
    renderAll();
  }

  function applySiteChrome() {
    var settings = state.settings;
    document.documentElement.setAttribute("data-accent", settings.accent || "indigo");
    if (settings.siteName) {
      document.title = "管理后台 · " + settings.siteName;
      $$(".admin-side .brand-text strong").forEach(function (node) { node.textContent = settings.siteName; });
    }
    $("#appVersion").textContent = state.version;
    fillSettingsForm();
  }

  function renderAll() {
    renderStats();
    renderOverview();
    renderCards();
    renderCategories();
    renderCategorySelects();
    renderNotices();
    renderUsers();
    renderImages();
    renderPendingBadge();
  }

  function renderPendingBadge() {
    var pending = state.stats.pending || 0;
    var badge = $("#pendingBadge");
    badge.textContent = pending;
    badge.hidden = !pending;
  }

  // -------------------------------------------------------------------------
  // Overview

  function renderStats() {
    var stats = state.stats;
    var items = [
      { label: "服务卡片", value: stats.cards || 0, sub: (stats.pending || 0) + " 个待审核", icon: "card" },
      { label: "分类", value: stats.categories || 0, sub: "最多三级结构", icon: "folder" },
      { label: "累计点击", value: formatCount(stats.totalClicks || 0), sub: "来自全部卡片", icon: "bolt" },
      { label: "今日访问", value: formatCount(stats.viewsToday || 0), sub: "首页打开次数", icon: "layout" },
      { label: "用户", value: stats.users || 0, sub: "含管理员账号", icon: "users" },
      { label: "数据库", value: formatBytes(stats.databaseSize || 0), sub: (stats.images || 0) + " 张图片", icon: "image" }
    ];
    $("#statGrid").innerHTML = items.map(function (item) {
      return '<div class="stat-card"><span class="stat-label">' + icon(item.icon, 14) + esc(item.label) +
        '</span><strong class="stat-value">' + esc(item.value) + '</strong><span class="stat-sub">' +
        esc(item.sub) + "</span></div>";
    }).join("");
  }

  function formatCount(n) {
    if (n >= 10000) return (n / 10000).toFixed(1).replace(/\.0$/, "") + "w";
    if (n >= 1000) return (n / 1000).toFixed(1).replace(/\.0$/, "") + "k";
    return String(n);
  }

  function renderOverview() {
    var daily = state.stats.dailyViews || [];
    var max = Math.max.apply(null, daily.map(function (d) { return d.views; }).concat([1]));
    $("#viewsChart").innerHTML = daily.map(function (day) {
      var height = Math.max(4, Math.round((day.views / max) * 100));
      var label = day.day.slice(5).replace("-", "/");
      return '<div class="chart-col"><span class="bar-value">' + day.views + '</span>' +
        '<div class="chart-bar-wrap"><div class="chart-bar" style="height:' + height + '%"></div></div>' +
        '<span class="bar-label">' + label + "</span></div>";
    }).join("") || '<div class="image-empty">还没有访问数据</div>';

    var top = state.stats.topCards || [];
    $("#topCards").innerHTML = top.length ? top.map(function (card, index) {
      return '<div class="rank-row"><span class="rank-index">' + (index + 1) + "</span>" +
        cardIcon(card) + '<div class="rank-main"><strong>' + esc(card.name) + "</strong><span>" +
        esc(pathOf(card.categoryId)) + '</span></div><span class="rank-value">' + formatCount(card.clicks) +
        "</span></div>";
    }).join("") : '<div class="image-empty">暂无点击数据</div>';

    var recent = state.stats.recentCards || [];
    $("#recentCards").innerHTML = recent.length ? recent.map(function (card) {
      return "<tr><td><div class=\"cell-main\">" + cardIcon(card) + "<div><strong>" + esc(card.name) +
        "</strong><span>" + esc(card.url || "—") + "</span></div></div></td><td>" + esc(pathOf(card.categoryId)) +
        '</td><td><span class="badge-status ' + (card.status === "approved" ? "ok" : "pending") + '">' +
        (card.status === "approved" ? "已上线" : "待审核") + "</span></td><td>" + formatDate(card.createdAt) + "</td></tr>";
    }).join("") : '<tr><td colspan="4"><div class="image-empty">还没有卡片</div></td></tr>';
  }

  // -------------------------------------------------------------------------
  // Cards

  function renderCards() {
    var filters = state.cardFilters;
    var term = filters.q.toLowerCase();
    var cards = state.cards.filter(function (card) {
      if (filters.status && card.status !== filters.status) return false;
      if (filters.categoryId && String(card.categoryId) !== String(filters.categoryId)) return false;
      if (!term) return true;
      return [card.name, card.description, card.url, (card.tags || []).join(" ")].join(" ").toLowerCase().indexOf(term) >= 0;
    });

    $("#cardTable").innerHTML = cards.length ? cards.map(function (card) {
      var siblings = state.cards.filter(function (c) { return c.categoryId === card.categoryId; })
        .sort(function (a, b) { return a.sort - b.sort || a.id - b.id; });
      var index = siblings.findIndex(function (c) { return c.id === card.id; });
      var tags = (card.tags || []).map(function (tag) { return '<span class="tag">' + esc(tag) + "</span>"; }).join("");
      return "<tr><td><div class=\"row-actions\">" +
        '<button class="mini-btn" type="button" data-move-card="' + card.id + '" data-delta="-1" ' +
        (index <= 0 ? "disabled" : "") + ' title="上移">' + icon("up", 14) + "</button>" +
        '<button class="mini-btn" type="button" data-move-card="' + card.id + '" data-delta="1" ' +
        (index >= siblings.length - 1 ? "disabled" : "") + ' title="下移">' + icon("down", 14) + "</button>" +
        "</div></td><td><div class=\"cell-main\">" + cardIcon(card) + "<div><strong>" + esc(card.name) +
        "</strong><span>" + esc(card.url || "未设置链接") + "</span></div></div></td><td>" +
        esc(pathOf(card.categoryId)) + '</td><td><div class="tag-list">' + (tags || '<span class="tag">—</span>') +
        '</div></td><td><span class="badge-status ' + (card.status === "approved" ? "ok" : "pending") + '">' +
        (card.status === "approved" ? "已上线" : "待审核") + "</span></td><td>" + formatCount(card.clicks) +
        '</td><td><div class="row-actions">' +
        (card.status === "pending" ? '<button class="mini-btn approve" type="button" data-approve="' + card.id +
          '" title="审核通过">' + icon("check", 15) + "</button>" : "") +
        '<button class="mini-btn" type="button" data-edit-card="' + card.id + '" title="编辑">' + icon("edit", 15) + "</button>" +
        '<button class="mini-btn danger" type="button" data-delete-card="' + card.id + '" title="删除">' + icon("trash", 15) + "</button>" +
        "</div></td></tr>";
    }).join("") : '<tr><td colspan="7"><div class="image-empty">没有匹配的卡片</div></td></tr>';

    $$("[data-edit-card]").forEach(function (button) {
      button.addEventListener("click", function () { openCardDialog(findCard(Number(button.dataset.editCard))); });
    });
    $$("[data-delete-card]").forEach(function (button) {
      button.addEventListener("click", function () {
        var card = findCard(Number(button.dataset.deleteCard));
        if (!card) return;
        confirmAction("删除卡片？", "「" + card.name + "」将被永久删除。", async function () {
          await api("/api/admin/cards/" + card.id, { method: "DELETE" });
          state.cards = state.cards.filter(function (c) { return c.id !== card.id; });
          toast("卡片已删除", "success");
          await refreshStats();
          renderCards();
          renderOverview();
        });
      });
    });
    $$("[data-approve]").forEach(function (button) {
      button.addEventListener("click", async function () {
        var card = findCard(Number(button.dataset.approve));
        if (!card) return;
        try {
          var data = await api("/api/admin/cards/" + card.id + "/approve", { method: "POST", body: "{}" });
          Object.assign(card, data.card);
          toast("已通过审核", "success");
          await refreshStats();
          renderCards();
          renderOverview();
        } catch (err) { toast(err.message, "error"); }
      });
    });
    $$("[data-move-card]").forEach(function (button) {
      button.addEventListener("click", async function () {
        var card = findCard(Number(button.dataset.moveCard));
        if (card) await moveCard(card, Number(button.dataset.delta));
      });
    });
  }

  function findCard(id) {
    return state.cards.find(function (card) { return card.id === id; });
  }

  async function moveCard(card, delta) {
    var siblings = state.cards.filter(function (c) { return c.categoryId === card.categoryId; })
      .sort(function (a, b) { return a.sort - b.sort || a.id - b.id; });
    var index = siblings.findIndex(function (c) { return c.id === card.id; });
    var target = index + delta;
    if (target < 0 || target >= siblings.length) return;
    var tmp = siblings[index];
    siblings[index] = siblings[target];
    siblings[target] = tmp;
    try {
      await api("/api/admin/sort", {
        method: "POST",
        body: JSON.stringify({ type: "card", categoryId: card.categoryId, ids: siblings.map(function (c) { return c.id; }) })
      });
      await loadCards();
    } catch (err) { toast(err.message, "error"); }
  }

  async function loadCards() {
    var data = await api("/api/admin/cards");
    state.cards = data.cards || [];
    renderCards();
  }

  async function refreshStats() {
    var data = await api("/api/admin/overview");
    state.stats = data.stats || {};
    state.categories = data.categories || [];
    renderStats();
    renderPendingBadge();
    renderCategorySelects();
    renderOverview();
  }

  function openCardDialog(card) {
    formError("cardError", "");
    $("#cardId").value = card ? card.id : "";
    $("#cardDialogTitle").textContent = card ? "编辑卡片" : "新建卡片";
    $("#cardName").value = card ? card.name : "";
    $("#cardURL").value = card ? card.url : "";
    $("#cardDesc").value = card ? card.description : "";
    $("#cardTags").value = card ? (card.tags || []).join(", ") : "";
    $("#cardIconType").value = card ? card.iconType : "emoji";
    $("#cardIconValue").value = card ? card.iconValue : "🚀";
    $("#cardSort").value = card ? card.sort : 0;
    $("#cardStatus").value = card ? card.status : "approved";
    renderCategorySelects();
    $("#cardCategory").value = card ? card.categoryId : firstLeafCategoryID();
    updateCardIconPreview();
    openDialog("cardDialog");
  }

  function firstLeafCategoryID() {
    var leaves = state.categories.filter(function (cat) {
      return !state.categories.some(function (other) { return other.parentId === cat.id; });
    }).sort(function (a, b) { return a.sort - b.sort || a.id - b.id; });
    return leaves.length ? leaves[0].id : (state.categories[0] || {}).id;
  }

  function updateCardIconPreview() {
    iconPreview($("#cardIconPreview"), {
      name: $("#cardName").value || "?",
      iconType: $("#cardIconType").value,
      iconValue: $("#cardIconValue").value
    });
  }

  async function submitCard(event) {
    event.preventDefault();
    var id = $("#cardId").value;
    var tags = $("#cardTags").value.split(/[,，]/).map(function (tag) { return tag.trim(); }).filter(Boolean).slice(0, 6);
    var payload = {
      name: $("#cardName").value.trim(),
      url: $("#cardURL").value.trim(),
      categoryId: Number($("#cardCategory").value),
      description: $("#cardDesc").value.trim(),
      iconType: $("#cardIconType").value,
      iconValue: $("#cardIconValue").value.trim(),
      tags: tags,
      sort: Number($("#cardSort").value) || 0,
      status: $("#cardStatus").value
    };
    if (!payload.name || !payload.categoryId) {
      formError("cardError", "请填写名称并选择分类");
      return;
    }
    var button = $("#cardSubmit");
    button.disabled = true;
    try {
      var data = await api("/api/admin/cards" + (id ? "/" + id : ""), {
        method: id ? "PUT" : "POST",
        body: JSON.stringify(payload)
      });
      if (id) {
        var index = state.cards.findIndex(function (c) { return c.id === Number(id); });
        if (index >= 0) state.cards[index] = data.card;
      } else {
        state.cards.push(data.card);
      }
      closeDialog("cardDialog");
      toast(id ? "卡片已更新" : "卡片已创建", "success");
      await refreshStats();
      renderCards();
    } catch (err) {
      formError("cardError", err.message);
    } finally {
      button.disabled = false;
    }
  }

  function renderCategorySelects() {
    var ordered = orderedCategories();
    var options = ordered.map(function (cat) {
      var prefix = cat._depth ? "— ".repeat(cat._depth) : "";
      return '<option value="' + cat.id + '">' + esc(prefix + cat.name) + "</option>";
    }).join("");
    var cardSelect = $("#cardCategory");
    if (cardSelect) {
      var current = cardSelect.value;
      cardSelect.innerHTML = options;
      if (current) cardSelect.value = current;
    }
    var filter = $("#cardCategoryFilter");
    if (filter) {
      var currentFilter = state.cardFilters.categoryId;
      filter.innerHTML = '<option value="">全部分类</option>' + options;
      filter.value = currentFilter || "";
    }
    var parent = $("#categoryParent");
    if (parent) {
      var currentParent = parent.value;
      parent.innerHTML = '<option value="0">顶级分类</option>' + ordered.filter(function (cat) {
        return cat._depth < 2;
      }).map(function (cat) {
        var prefix = cat._depth ? "— ".repeat(cat._depth) : "";
        return '<option value="' + cat.id + '">' + esc(prefix + cat.name) + "</option>";
      }).join("");
      parent.value = currentParent || "0";
    }
  }

  // -------------------------------------------------------------------------
  // Categories

  function renderCategories() {
    var ordered = orderedCategories();
    $("#categoryTable").innerHTML = ordered.length ? ordered.map(function (cat) {
      var siblings = state.categories.filter(function (c) { return c.parentId === cat.parentId; })
        .sort(function (a, b) { return a.sort - b.sort || a.id - b.id; });
      var index = siblings.findIndex(function (c) { return c.id === cat.id; });
      var prefix = cat._depth ? "— ".repeat(cat._depth) : "";
      return "<tr><td><div class=\"row-actions\">" +
        '<button class="mini-btn" type="button" data-move-cat="' + cat.id + '" data-delta="-1" ' +
        (index <= 0 ? "disabled" : "") + ">" + icon("up", 14) + "</button>" +
        '<button class="mini-btn" type="button" data-move-cat="' + cat.id + '" data-delta="1" ' +
        (index >= siblings.length - 1 ? "disabled" : "") + ">" + icon("down", 14) + "</button>" +
        "</div></td><td><strong>" + esc(prefix + (cat.icon ? cat.icon + " " : "") + cat.name) + "</strong></td><td>" +
        (cat.description ? esc(cat.description) : '<span style="color:var(--faint)">—</span>') + "</td><td>" +
        ("第 " + (cat._depth + 1) + " 级") + "</td><td>" + (cat.cardCount || 0) + "</td><td>" +
        '<span class="badge-status ' + (cat.visible ? "ok" : "hidden") + '">' + (cat.visible ? "可见" : "隐藏") +
        '</span></td><td><div class="row-actions">' +
        '<button class="mini-btn" type="button" data-edit-cat="' + cat.id + '" title="编辑">' + icon("edit", 15) + "</button>" +
        '<button class="mini-btn danger" type="button" data-delete-cat="' + cat.id + '" title="删除">' + icon("trash", 15) + "</button>" +
        "</div></td></tr>";
    }).join("") : '<tr><td colspan="7"><div class="image-empty">还没有分类</div></td></tr>';

    $$("[data-edit-cat]").forEach(function (button) {
      button.addEventListener("click", function () { openCategoryDialog(findCategory(Number(button.dataset.editCat))); });
    });
    $$("[data-delete-cat]").forEach(function (button) {
      button.addEventListener("click", function () {
        var cat = findCategory(Number(button.dataset.deleteCat));
        if (!cat) return;
        confirmAction("删除分类？", "「" + cat.name + "」及其子分类、卡片都会被删除。", async function () {
          await api("/api/admin/categories/" + cat.id, { method: "DELETE" });
          toast("分类已删除", "success");
          await loadAll();
        });
      });
    });
    $$("[data-move-cat]").forEach(function (button) {
      button.addEventListener("click", function () {
        var cat = findCategory(Number(button.dataset.moveCat));
        if (cat) moveCategory(cat, Number(button.dataset.delta));
      });
    });
  }

  function findCategory(id) {
    return state.categories.find(function (cat) { return cat.id === id; });
  }

  async function moveCategory(cat, delta) {
    var siblings = state.categories.filter(function (c) { return c.parentId === cat.parentId; })
      .sort(function (a, b) { return a.sort - b.sort || a.id - b.id; });
    var index = siblings.findIndex(function (c) { return c.id === cat.id; });
    var target = index + delta;
    if (target < 0 || target >= siblings.length) return;
    var tmp = siblings[index];
    siblings[index] = siblings[target];
    siblings[target] = tmp;
    try {
      await api("/api/admin/sort", {
        method: "POST",
        body: JSON.stringify({ type: "category", parentId: cat.parentId, ids: siblings.map(function (c) { return c.id; }) })
      });
      await loadAll();
    } catch (err) { toast(err.message, "error"); }
  }

  function openCategoryDialog(cat) {
    formError("categoryError", "");
    renderCategorySelects();
    $("#categoryId").value = cat ? cat.id : "";
    $("#categoryDialogTitle").textContent = cat ? "编辑分类" : "新建分类";
    $("#categoryName").value = cat ? cat.name : "";
    $("#categoryIcon").value = cat ? cat.icon : "📁";
    $("#categoryDesc").value = cat ? cat.description : "";
    $("#categorySort").value = cat ? cat.sort : 0;
    $("#categoryVisible").checked = cat ? cat.visible : true;
    var parent = $("#categoryParent");
    parent.value = cat ? String(cat.parentId) : "0";
    $$("option", parent).forEach(function (option) { option.disabled = false; });
    if (cat) {
      var descendants = new Set();
      (function collect(id) {
        state.categories.forEach(function (c) {
          if (c.parentId === id) { descendants.add(c.id); collect(c.id); }
        });
      })(cat.id);
      descendants.add(cat.id);
      $$("option", parent).forEach(function (option) {
        option.disabled = descendants.has(Number(option.value));
      });
    }
    openDialog("categoryDialog");
  }

  async function submitCategory(event) {
    event.preventDefault();
    var id = $("#categoryId").value;
    var payload = {
      name: $("#categoryName").value.trim(),
      icon: $("#categoryIcon").value.trim(),
      description: $("#categoryDesc").value.trim(),
      parentId: Number($("#categoryParent").value) || 0,
      sort: Number($("#categorySort").value) || 0,
      visible: $("#categoryVisible").checked
    };
    if (!payload.name) {
      formError("categoryError", "请填写分类名称");
      return;
    }
    var button = $("#categorySubmit");
    button.disabled = true;
    try {
      await api("/api/admin/categories" + (id ? "/" + id : ""), {
        method: id ? "PUT" : "POST",
        body: JSON.stringify(payload)
      });
      closeDialog("categoryDialog");
      toast(id ? "分类已更新" : "分类已创建", "success");
      await loadAll();
    } catch (err) {
      formError("categoryError", err.message);
    } finally {
      button.disabled = false;
    }
  }

  // -------------------------------------------------------------------------
  // Notices

  function renderNotices() {
    $("#noticeTable").innerHTML = state.notices.length ? state.notices.map(function (notice) {
      return "<tr><td><strong>" + esc(notice.title) + "</strong><div style=\"font-size:10.5px;color:var(--faint);max-width:420px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap\">" +
        esc(notice.content || "—") + "</div></td><td><span class=\"badge-status " +
        (notice.level === "danger" ? "pending" : notice.level === "success" ? "ok" : "admin") + "\">" +
        esc(levelLabel(notice.level)) + "</span></td><td>" + (notice.expiresAt ? formatDateTime(notice.expiresAt) : "长期") +
        '</td><td><span class="badge-status ' + (notice.active ? "ok" : "hidden") + '">' +
        (notice.active ? "启用" : "停用") + '</span></td><td><div class="row-actions">' +
        '<button class="mini-btn" type="button" data-edit-notice="' + notice.id + '">' + icon("edit", 15) + "</button>" +
        '<button class="mini-btn danger" type="button" data-delete-notice="' + notice.id + '">' + icon("trash", 15) + "</button>" +
        "</div></td></tr>";
    }).join("") : '<tr><td colspan="5"><div class="image-empty">还没有公告</div></td></tr>';

    $$("[data-edit-notice]").forEach(function (button) {
      button.addEventListener("click", function () { openNoticeDialog(findNotice(Number(button.dataset.editNotice))); });
    });
    $$("[data-delete-notice]").forEach(function (button) {
      button.addEventListener("click", function () {
        var notice = findNotice(Number(button.dataset.deleteNotice));
        if (!notice) return;
        confirmAction("删除公告？", "「" + notice.title + "」将不再显示。", async function () {
          await api("/api/admin/notices/" + notice.id, { method: "DELETE" });
          state.notices = state.notices.filter(function (n) { return n.id !== notice.id; });
          renderNotices();
          toast("公告已删除", "success");
        });
      });
    });
  }

  function levelLabel(level) {
    return { info: "信息", success: "成功", warning: "提醒", danger: "重要" }[level] || level;
  }

  function findNotice(id) {
    return state.notices.find(function (notice) { return notice.id === id; });
  }

  function openNoticeDialog(notice) {
    formError("noticeError", "");
    $("#noticeId").value = notice ? notice.id : "";
    $("#noticeDialogTitle").textContent = notice ? "编辑公告" : "新建公告";
    $("#noticeTitle").value = notice ? notice.title : "";
    $("#noticeContent").value = notice ? notice.content : "";
    $("#noticeLevel").value = notice ? notice.level : "info";
    $("#noticeLink").value = notice ? notice.link : "";
    $("#noticeSort").value = notice ? notice.sort : 0;
    $("#noticeActive").checked = notice ? notice.active : true;
    var expires = $("#noticeExpires");
    if (notice && notice.expiresAt) {
      var d = new Date(notice.expiresAt * 1000);
      expires.value = d.getFullYear() + "-" + pad(d.getMonth() + 1) + "-" + pad(d.getDate()) + "T" +
        pad(d.getHours()) + ":" + pad(d.getMinutes());
    } else {
      expires.value = "";
    }
    openDialog("noticeDialog");
  }

  async function submitNotice(event) {
    event.preventDefault();
    var id = $("#noticeId").value;
    var expires = $("#noticeExpires").value;
    var payload = {
      title: $("#noticeTitle").value.trim(),
      content: $("#noticeContent").value.trim(),
      level: $("#noticeLevel").value,
      link: $("#noticeLink").value.trim(),
      sort: Number($("#noticeSort").value) || 0,
      active: $("#noticeActive").checked,
      expiresAt: expires ? Math.floor(new Date(expires).getTime() / 1000) : 0
    };
    if (!payload.title) {
      formError("noticeError", "请填写公告标题");
      return;
    }
    try {
      var data = await api("/api/admin/notices" + (id ? "/" + id : ""), {
        method: id ? "PUT" : "POST",
        body: JSON.stringify(payload)
      });
      if (id) {
        var index = state.notices.findIndex(function (n) { return n.id === Number(id); });
        if (index >= 0) state.notices[index] = Object.assign(state.notices[index], data.notice);
      } else {
        state.notices.unshift(data.notice);
      }
      closeDialog("noticeDialog");
      renderNotices();
      toast(id ? "公告已更新" : "公告已创建", "success");
    } catch (err) {
      formError("noticeError", err.message);
    }
  }

  // -------------------------------------------------------------------------
  // Users

  function renderUsers() {
    $("#userTable").innerHTML = state.users.length ? state.users.map(function (user) {
      return '<tr><td><div class="cell-main"><span class="avatar" style="width:32px;height:32px;border-radius:9px">' +
        esc((user.displayName || user.username).slice(0, 1).toUpperCase()) + "</span><div><strong>" +
        esc(user.displayName || user.username) + "</strong><span>@" + esc(user.username) + "</span></div></div></td><td>" +
        '<span class="badge-status ' + (user.role === "admin" ? "admin" : "hidden") + '">' +
        (user.role === "admin" ? "管理员" : "普通成员") + "</span></td><td>" + formatDate(user.createdAt) +
        '</td><td><div class="row-actions">' +
        '<button class="mini-btn" type="button" data-edit-user="' + user.id + '" title="编辑">' + icon("edit", 15) + "</button>" +
        '<button class="mini-btn" type="button" data-reset-user="' + user.id + '" title="重置密码">' + icon("shield", 15) + "</button>" +
        (state.user && state.user.id === user.id ? "" :
          '<button class="mini-btn danger" type="button" data-delete-user="' + user.id + '" title="删除">' + icon("trash", 15) + "</button>") +
        "</div></td></tr>";
    }).join("") : '<tr><td colspan="4"><div class="image-empty">还没有用户</div></td></tr>';

    $$("[data-edit-user]").forEach(function (button) {
      button.addEventListener("click", function () { openUserDialog(findUser(Number(button.dataset.editUser))); });
    });
    $$("[data-reset-user]").forEach(function (button) {
      button.addEventListener("click", function () {
        var user = findUser(Number(button.dataset.resetUser));
        if (!user) return;
        $("#resetError").classList.remove("show");
        $("#resetPassword").value = "";
        $("#resetUserId").value = user.id;
        $("#resetTarget").textContent = "为 " + (user.displayName || user.username) + " 设置新密码";
        openDialog("passwordDialog");
      });
    });
    $$("[data-delete-user]").forEach(function (button) {
      button.addEventListener("click", function () {
        var user = findUser(Number(button.dataset.deleteUser));
        if (!user) return;
        confirmAction("删除用户？", "「" + (user.displayName || user.username) + "」将无法再登录。", async function () {
          await api("/api/admin/users/" + user.id, { method: "DELETE" });
          state.users = state.users.filter(function (u) { return u.id !== user.id; });
          renderUsers();
          toast("用户已删除", "success");
          await refreshStats();
        });
      });
    });
  }

  function findUser(id) {
    return state.users.find(function (user) { return user.id === id; });
  }

  function openUserDialog(user) {
    formError("userError", "");
    $("#userId").value = user ? user.id : "";
    $("#userDialogTitle").textContent = user ? "编辑用户" : "新建用户";
    $("#userUsername").value = user ? user.username : "";
    $("#userUsername").disabled = Boolean(user);
    $("#userDisplay").value = user ? user.displayName : "";
    $("#userRole").value = user ? user.role : "member";
    $("#userPassword").value = "";
    $("#userPasswordField").hidden = Boolean(user);
    openDialog("userDialog");
  }

  async function submitUser(event) {
    event.preventDefault();
    var id = $("#userId").value;
    var payload = {
      username: $("#userUsername").value.trim(),
      displayName: $("#userDisplay").value.trim(),
      role: $("#userRole").value
    };
    if (!id) {
      payload.password = $("#userPassword").value;
      if (!payload.username || !payload.password) {
        formError("userError", "请填写用户名和初始密码");
        return;
      }
    }
    try {
      var data = await api("/api/admin/users" + (id ? "/" + id : ""), {
        method: id ? "PUT" : "POST",
        body: JSON.stringify(payload)
      });
      if (id) {
        var index = state.users.findIndex(function (u) { return u.id === Number(id); });
        if (index >= 0) state.users[index] = Object.assign(state.users[index], { displayName: payload.displayName, role: payload.role });
      } else {
        state.users.push(data.user);
      }
      closeDialog("userDialog");
      renderUsers();
      toast(id ? "用户已更新" : "用户已创建", "success");
    } catch (err) {
      formError("userError", err.message);
    }
  }

  async function submitReset(event) {
    event.preventDefault();
    var id = $("#resetUserId").value;
    var password = $("#resetPassword").value;
    if (password.length < 8) {
      formError("resetError", "密码至少 8 位");
      return;
    }
    try {
      await api("/api/admin/users/" + id + "/password", {
        method: "POST",
        body: JSON.stringify({ password: password })
      });
      closeDialog("passwordDialog");
      toast("密码已重置", "success");
    } catch (err) {
      formError("resetError", err.message);
    }
  }

  // -------------------------------------------------------------------------
  // Images

  function renderImages() {
    $("#imageCount").textContent = state.images.length + " 张";
    $("#imageGrid").innerHTML = state.images.length ? state.images.map(function (image) {
      return '<div class="image-tile"><div class="thumb"><img src="/api/images/' + esc(image.id) +
        '" alt="' + esc(image.name) + '" loading="lazy"></div><div class="tile-meta"><span>' +
        esc(image.name || image.id) + "</span><span>" + formatBytes(image.size) + "</span></div>" +
        '<div class="tile-actions">' +
        '<button type="button" data-copy-image="' + esc(image.id) + '" title="复制地址">' + icon("link", 13) + "</button>" +
        '<button type="button" data-delete-image="' + esc(image.id) + '" title="删除">' + icon("trash", 13) + "</button>" +
        "</div></div>";
    }).join("") : '<div class="image-empty">还没有上传图片</div>';

    $$("[data-copy-image]").forEach(function (button) {
      button.addEventListener("click", async function () {
        var url = window.location.origin + "/api/images/" + button.dataset.copyImage;
        try {
          await navigator.clipboard.writeText(url);
          toast("图片地址已复制", "success");
        } catch (e) {
          toast(url);
        }
      });
    });
    $$("[data-delete-image]").forEach(function (button) {
      button.addEventListener("click", function () {
        var id = button.dataset.deleteImage;
        confirmAction("删除图片？", "引用该图片的卡片图标会失效。", async function () {
          await api("/api/admin/images/" + id, { method: "DELETE" });
          state.images = state.images.filter(function (img) { return img.id !== id; });
          renderImages();
          toast("图片已删除", "success");
        });
      });
    });
  }

  async function uploadImage(file, onDone) {
    if (!file) return;
    if (file.size > 2 * 1024 * 1024) {
      toast("图片不能超过 2MB", "error");
      return;
    }
    var form = new FormData();
    form.append("file", file);
    try {
      var data = await api("/api/admin/images", { method: "POST", body: form });
      state.images.unshift(data.image);
      renderImages();
      toast(data.deduped ? "图片已存在，直接复用" : "图片上传成功", "success");
      if (onDone) onDone(data.url);
    } catch (err) {
      toast(err.message, "error");
    }
  }

  // -------------------------------------------------------------------------
  // Settings

  function fillSettingsForm() {
    var s = state.settings;
    $("#setSiteName").value = s.siteName || "";
    $("#setSiteSubtitle").value = s.siteSubtitle || "";
    $("#setWelcome").value = s.welcomeText || "";
    $("#setAccent").value = s.accent || "indigo";
    $("#setInterval").value = s.noticeInterval || 8;
    $("#setSessionTTL").value = s.sessionTTL || 66;
    $("#setLogo").value = s.siteLogo || "";
    $("#setFooter").value = s.footerText || "";
    $("#setAllowApply").checked = s.allowApply !== "0";
    $("#setShowClicks").checked = s.showClicks !== "0";
    updateLogoPreview(s.siteLogo);
  }

  function updateLogoPreview(url) {
    var node = $("#logoPreview");
    if (url) {
      node.innerHTML = '<img src="' + esc(url) + '" alt="">';
    } else {
      node.textContent = "✦";
      node.innerHTML = "✦";
      node.style.background = "";
    }
  }

  async function submitSettings(event) {
    event.preventDefault();
    var payload = {
      siteName: $("#setSiteName").value.trim(),
      siteSubtitle: $("#setSiteSubtitle").value.trim(),
      welcomeText: $("#setWelcome").value.trim(),
      accent: $("#setAccent").value,
      noticeInterval: String($("#setInterval").value || 8),
      sessionTTL: String($("#setSessionTTL").value || 66),
      siteLogo: $("#setLogo").value.trim(),
      footerText: $("#setFooter").value.trim(),
      allowApply: $("#setAllowApply").checked ? "1" : "0",
      showClicks: $("#setShowClicks").checked ? "1" : "0"
    };
    try {
      var data = await api("/api/admin/settings", { method: "PUT", body: JSON.stringify(payload) });
      state.settings = data.settings;
      applySiteChrome();
      toast("设置已保存", "success");
    } catch (err) {
      toast(err.message, "error");
    }
  }

  async function submitPassword(event) {
    event.preventDefault();
    var current = $("#pwdCurrent").value;
    var next = $("#pwdNew").value;
    if (next.length < 8) {
      toast("新密码至少 8 位", "error");
      return;
    }
    try {
      await api("/api/admin/password", {
        method: "POST",
        body: JSON.stringify({ currentPassword: current, newPassword: next })
      });
      $("#passwordForm").reset();
      toast("密码已更新", "success");
    } catch (err) {
      toast(err.message, "error");
    }
  }

  // -------------------------------------------------------------------------
  // Navigation and events

  function switchView(view) {
    state.view = view;
    $$(".admin-nav-item").forEach(function (button) {
      button.classList.toggle("active", button.dataset.view === view);
    });
    $$(".view").forEach(function (panel) {
      panel.classList.toggle("active", panel.dataset.panel === view);
    });
    var titles = state.viewTitles[view] || [view, view];
    $("#viewTitle").textContent = titles[0];
    $("#crumb").textContent = titles[1];
    document.body.classList.remove("admin-nav-open");
    window.scrollTo({ top: 0, behavior: "smooth" });
    if (view === "overview") refreshStats().catch(function () {});
  }

  function bindEvents() {
    $("#adminNav").addEventListener("click", function (event) {
      var button = event.target.closest("[data-view]");
      if (button) switchView(button.dataset.view);
    });
    $$("[data-goto]").forEach(function (button) {
      button.addEventListener("click", function () { switchView(button.dataset.goto); });
    });
    $("#adminMenu").addEventListener("click", function () {
      document.body.classList.toggle("admin-nav-open");
    });
    document.addEventListener("click", function (event) {
      if (document.body.classList.contains("admin-nav-open") &&
        !event.target.closest(".admin-side") && !event.target.closest("#adminMenu")) {
        document.body.classList.remove("admin-nav-open");
      }
    });

    $("#themeToggle").addEventListener("click", function () {
      var next = document.documentElement.getAttribute("data-theme") === "dark" ? "light" : "dark";
      document.documentElement.setAttribute("data-theme", next);
      try { localStorage.setItem("tn-theme", next); } catch (e) { /* ignore */ }
    });

    $("#logoutButton").addEventListener("click", async function () {
      try { await api("/api/auth/logout", { method: "POST", body: "{}" }); } catch (e) { /* ignore */ }
      window.location.href = "/";
    });

    $$("[data-close-dialog]").forEach(function (button) {
      button.addEventListener("click", function () {
        var dialog = button.closest("dialog");
        if (dialog) dialog.close();
      });
    });

    $("#confirmButton").addEventListener("click", async function () {
      var action = state.confirmAction;
      state.confirmAction = null;
      closeDialog("confirmDialog");
      if (action) {
        try { await action(); } catch (err) { toast(err.message, "error"); }
      }
    });

    $("#cardForm").addEventListener("submit", submitCard);
    $("#categoryForm").addEventListener("submit", submitCategory);
    $("#noticeForm").addEventListener("submit", submitNotice);
    $("#userForm").addEventListener("submit", submitUser);
    $("#resetForm").addEventListener("submit", submitReset);
    $("#settingsForm").addEventListener("submit", submitSettings);
    $("#passwordForm").addEventListener("submit", submitPassword);

    $("#addCardButton").addEventListener("click", function () { openCardDialog(null); });
    $("#addCategoryButton").addEventListener("click", function () { openCategoryDialog(null); });
    $("#addNoticeButton").addEventListener("click", function () { openNoticeDialog(null); });
    $("#addUserButton").addEventListener("click", function () { openUserDialog(null); });

    $("#cardSearch").addEventListener("input", function () {
      state.cardFilters.q = $("#cardSearch").value;
      renderCards();
    });
    $("#cardStatusFilter").addEventListener("change", function () {
      state.cardFilters.status = $("#cardStatusFilter").value;
      renderCards();
    });
    $("#cardCategoryFilter").addEventListener("change", function () {
      state.cardFilters.categoryId = $("#cardCategoryFilter").value;
      renderCards();
    });

    $("#cardName").addEventListener("input", updateCardIconPreview);
    $("#cardIconType").addEventListener("change", function () {
      var type = $("#cardIconType").value;
      var value = $("#cardIconValue");
      if (type === "emoji" && !value.value) value.value = "🚀";
      if (type === "text" && !value.value) value.value = (($("#cardName").value || "A").slice(0, 1).toUpperCase());
      updateCardIconPreview();
    });
    $("#cardIconValue").addEventListener("input", updateCardIconPreview);

    $("#cardIconUpload").addEventListener("click", function () { $("#cardIconFile").click(); });
    $("#cardIconFile").addEventListener("change", function (event) {
      uploadImage(event.target.files[0], function (url) {
        $("#cardIconType").value = "image";
        $("#cardIconValue").value = url;
        updateCardIconPreview();
      });
      event.target.value = "";
    });

    $("#logoUploadButton").addEventListener("click", function () { $("#logoInput").click(); });
    $("#logoInput").addEventListener("change", function (event) {
      uploadImage(event.target.files[0], function (url) {
        $("#setLogo").value = url;
        updateLogoPreview(url);
      });
      event.target.value = "";
    });
    $("#setLogo").addEventListener("input", function () {
      updateLogoPreview($("#setLogo").value.trim());
    });

    var zone = $("#uploadZone");
    zone.addEventListener("click", function () { $("#imageInput").click(); });
    $("#imageInput").addEventListener("change", function (event) {
      Array.prototype.forEach.call(event.target.files, function (file) { uploadImage(file); });
      event.target.value = "";
    });
    ["dragenter", "dragover"].forEach(function (name) {
      zone.addEventListener(name, function (event) {
        event.preventDefault();
        zone.classList.add("dragging");
      });
    });
    ["dragleave", "drop"].forEach(function (name) {
      zone.addEventListener(name, function (event) {
        event.preventDefault();
        zone.classList.remove("dragging");
      });
    });
    zone.addEventListener("drop", function (event) {
      Array.prototype.forEach.call(event.dataTransfer.files, function (file) { uploadImage(file); });
    });
  }

  init();
})();