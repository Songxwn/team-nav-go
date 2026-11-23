/* global localStorage, fetch, IntersectionObserver */
(function () {
  "use strict";

  var $ = function (sel, root) { return (root || document).querySelector(sel); };
  var $$ = function (sel, root) { return Array.prototype.slice.call((root || document).querySelectorAll(sel)); };

  var state = {
    site: {},
    categories: [],
    cards: [],
    notices: [],
    user: null,
    stats: {},
    categoryMap: new Map(),
    favorites: loadFavorites(),
    favOnly: false,
    noticeIndex: 0,
    noticeTimer: null
  };

  var el = {
    nav: $("#categoryNav"),
    sections: $("#sections"),
    noticeBar: $("#noticeBar"),
    userChip: $("#userChip"),
    adminEntry: $("#adminEntry"),
    loginButton: $("#loginButton"),
    applyButton: $("#applyButton"),
    favButton: $("#favFilterButton"),
    favBanner: $("#favBanner"),
    paletteOverlay: $("#searchOverlay"),
    paletteInput: $("#paletteInput"),
    paletteBody: $("#paletteBody"),
    loginDialog: $("#loginDialog"),
    loginForm: $("#loginForm"),
    applyDialog: $("#applyDialog"),
    applyForm: $("#applyForm"),
    toasts: $("#toasts")
  };

  var paletteRows = [];
  var paletteIndex = 0;

  // -------------------------------------------------------------------------
  // Utilities

  function esc(value) {
    return String(value == null ? "" : value)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#39;");
  }

  function loadFavorites() {
    try {
      var raw = JSON.parse(localStorage.getItem("tn-favorites") || "[]");
      return new Set(raw.filter(function (n) { return typeof n === "number"; }));
    } catch (e) {
      return new Set();
    }
  }

  function saveFavorites() {
    try {
      localStorage.setItem("tn-favorites", JSON.stringify(Array.from(state.favorites)));
    } catch (e) {
      /* private mode */
    }
  }

  function toast(message, kind) {
    var node = document.createElement("div");
    node.className = "toast " + (kind || "");
    node.innerHTML = '<span class="toast-dot"></span><span></span>';
    node.lastElementChild.textContent = message;
    el.toasts.appendChild(node);
    window.setTimeout(function () {
      node.classList.add("out");
      window.setTimeout(function () { node.remove(); }, 260);
    }, 3200);
  }

  function icon(name, size) {
    return '<svg class="icon"' + (size ? ' style="width:' + size + "px;height:" + size + 'px"' : "") +
      '><use href="#i-' + name + '"/></svg>';
  }

  function toneFor(text) {
    var palette = ["#6366f1", "#8b5cf6", "#0ea5e9", "#14b8a6", "#f59e0b", "#ef4444", "#ec4899", "#10b981", "#0284c7", "#7c3aed"];
    var hash = 0;
    var str = String(text || "");
    for (var i = 0; i < str.length; i++) hash = (hash * 31 + str.charCodeAt(i)) >>> 0;
    return palette[hash % palette.length];
  }

  function hostOf(url) {
    try {
      return new URL(url).host.replace(/^www\./, "");
    } catch (e) {
      return url || "";
    }
  }

  function isImageIcon(card) {
    return (card.iconType === "image" || card.iconType === "url") && card.iconValue;
  }

  function iconMarkup(card, cls) {
    var extra = cls || "";
    if (isImageIcon(card)) {
      return '<span class="' + extra + '" style="--tone:' + toneFor(card.name) + '">' +
        '<img src="' + esc(card.iconValue) + '" alt="" loading="lazy" data-letter="' +
        esc(firstGlyph(card)) + '" data-tone="' + toneFor(card.name) + '"></span>';
    }
    var content;
    if (card.iconType === "emoji" && card.iconValue) {
      content = esc(card.iconValue.slice(0, 4));
    } else if (card.iconType === "text" && card.iconValue) {
      content = esc(card.iconValue.slice(0, 2));
    } else {
      content = esc(firstGlyph(card));
    }
    return '<span class="' + extra + '" style="--tone:' + toneFor(card.name) + '">' + content + "</span>";
  }

  function firstGlyph(card) {
    var name = String(card.name || "?").trim();
    return name ? Array.from(name)[0].toUpperCase() : "?";
  }

  function bindIconFallbacks(root) {
    $$("img[data-letter]", root).forEach(function (img) {
      img.addEventListener("error", function () {
        var holder = img.parentElement;
        if (!holder) return;
        holder.textContent = img.dataset.letter || "?";
        holder.style.background = "linear-gradient(140deg," + (img.dataset.tone || "#6366f1") + ",#0f172a)";
      }, { once: true });
    });
  }

  function cardURL(card) {
    return card.url || "";
  }

  // -------------------------------------------------------------------------
  // Bootstrap

  async function init() {
    applyThemeLabel();
    $("#todayLabel").textContent = formatToday();
    $("#searchShortcut").textContent = shortcutLabel();
    $("#searchShortcut2").textContent = shortcutLabel();
    bindGlobalEvents();

    try {
      var res = await fetch("/api/bootstrap", { headers: { Accept: "application/json" } });
      if (!res.ok) throw new Error("HTTP " + res.status);
      var data = await res.json();
      state.site = data.site || {};
      state.categories = data.categories || [];
      state.cards = data.cards || [];
      state.notices = data.notices || [];
      state.user = data.user || null;
      state.stats = data.stats || {};
      indexCategories(state.categories, 0);
      applySite();
      renderAll();
      if (new URLSearchParams(location.search).get("login") === "1" && !state.user) {
        openLogin();
      }
    } catch (err) {
      el.sections.innerHTML = '<div class="empty">' + icon("info") +
        "<span>加载失败，请刷新页面重试</span></div>";
      toast("加载数据失败：" + err.message, "error");
    }
  }

  function indexCategories(nodes, depth) {
    nodes.forEach(function (node) {
      node.depth = depth;
      state.categoryMap.set(node.id, node);
      if (node.children && node.children.length) indexCategories(node.children, depth + 1);
    });
  }

  function applySite() {
    var site = state.site;
    var name = site.siteName || "星云导航";
    document.title = name + " · 团队服务";
    $("#brandName").textContent = name;
    $("#brandSubtitle").textContent = site.siteSubtitle || "";
    $("#heroEyebrow").textContent = "WELCOME · " + name;
    $("#heroSubtitle").textContent = site.siteSubtitle || "团队服务，一站式入口";
    $("#footerText").textContent = site.footerText || "Built with Go · SQLite";
    document.documentElement.setAttribute("data-accent", site.accent || "indigo");
    if (site.welcomeText) $("#todayLabel").textContent = site.welcomeText + " · " + formatToday();

    var mark = $("#brandMark");
    if (site.siteLogo) {
      mark.innerHTML = '<img src="' + esc(site.siteLogo) + '" alt="">';
    }

    updateStats();
  }

  function updateStats() {
    $("#statServices").textContent = state.cards.length;
    $("#statCategories").textContent = state.categories.length;
    $("#statClicks").textContent = formatNumber(state.stats.clicks || 0);
  }

  function formatNumber(n) {
    if (n >= 10000) return (n / 10000).toFixed(1).replace(/\.0$/, "") + "w";
    if (n >= 1000) return (n / 1000).toFixed(1).replace(/\.0$/, "") + "k";
    return String(n);
  }

  function formatToday() {
    try {
      return new Intl.DateTimeFormat("zh-CN", { month: "long", day: "numeric", weekday: "short" }).format(new Date());
    } catch (e) {
      return new Date().toLocaleDateString();
    }
  }

  function shortcutLabel() {
    return /Mac/i.test(navigator.platform || "") ? "⌘ K" : "Ctrl K";
  }

  // -------------------------------------------------------------------------
  // Render

  function renderAll() {
    renderUser();
    renderNav();
    renderNotice();
    renderSections();
  }

  function cardsOf(categoryID) {
    return state.cards.filter(function (card) { return card.categoryId === categoryID; });
  }

  function countSubtree(node) {
    var total = cardsOf(node.id).length;
    (node.children || []).forEach(function (child) { total += countSubtree(child); });
    return total;
  }

  function renderUser() {
    var user = state.user;
    if (user) {
      el.userChip.innerHTML =
        '<div class="avatar">' + esc((user.displayName || user.username || "?").slice(0, 1).toUpperCase()) + "</div>" +
        '<div class="meta"><strong>' + esc(user.displayName || user.username) + "</strong><span>" +
        (user.role === "admin" ? "管理员 · 已登录" : "成员 · 已登录") + "</span></div>" +
        '<button class="icon-button" type="button" id="logoutButton" title="退出登录" style="width:30px;height:30px">' +
        icon("logout", 15) + "</button>";
      $("#logoutButton").addEventListener("click", logout);
      el.adminEntry.hidden = user.role !== "admin";
      el.loginButton.hidden = true;
    } else {
      el.userChip.innerHTML =
        '<div class="avatar" style="background:linear-gradient(140deg,#94a3b8,#475569)">?</div>' +
        '<div class="meta"><strong>访客模式</strong><span>登录后可提交卡片</span></div>' +
        '<button class="btn btn-soft btn-sm" type="button" id="chipLogin" style="height:30px;padding:0 10px">登录</button>';
      $("#chipLogin").addEventListener("click", openLogin);
      el.adminEntry.hidden = true;
      el.loginButton.hidden = false;
    }
  }

  function renderNav() {
    var favCount = state.favorites.size;
    var html = '<button class="nav-item' + (state.favOnly ? "" : " active") + '" type="button" data-target="top">' +
      icon("grid", 16) + '<span>全部服务</span><span class="nav-count">' + state.cards.length + "</span></button>";

    html += '<button class="nav-item' + (state.favOnly ? " active" : "") + '" type="button" data-fav-nav>' +
      '<span class="nav-emoji">★</span><span>我的收藏</span><span class="nav-count">' + favCount + "</span></button>";

    state.categories.forEach(function (root) {
      var count = countSubtree(root);
      if (!count) return;
      html += '<div class="nav-group">' +
        '<a class="nav-item" href="#cat-' + root.id + '" data-target="cat-' + root.id + '">' +
        '<span class="nav-emoji">' + esc(root.icon || "📁") + "</span><span>" + esc(root.name) + "</span>" +
        '<span class="nav-count">' + count + "</span></a>";
      var children = (root.children || []).filter(function (child) { return countSubtree(child) > 0; });
      if (children.length) {
        html += '<div class="nav-children">';
        children.forEach(function (child) {
          html += '<a class="nav-child" href="#sub-' + child.id + '"><span class="dot"></span>' +
            esc(child.name) + "</a>";
        });
        html += "</div>";
      }
      html += "</div>";
    });

    el.nav.innerHTML = html;
    $$("[data-target]", el.nav).forEach(function (link) {
      link.addEventListener("click", function () {
        closeNav();
        state.favOnly = false;
        el.favBanner.classList.remove("show");
        if (link.dataset.target === "top") {
          window.scrollTo({ top: 0, behavior: "smooth" });
          $$(".nav-item", el.nav).forEach(function (item) { item.classList.remove("active"); });
          link.classList.add("active");
        }
      });
    });
    $("[data-fav-nav]", el.nav).addEventListener("click", function () {
      state.favOnly = true;
      closeNav();
      renderNav();
      renderSections();
    });
  }

  function renderNotice() {
    var notices = state.notices || [];
    if (!notices.length) {
      el.noticeBar.hidden = true;
      if (state.noticeTimer) window.clearInterval(state.noticeTimer);
      return;
    }
    el.noticeBar.hidden = false;
    function paint() {
      var notice = notices[state.noticeIndex % notices.length];
      var countdown = "";
      if (notice.expiresAt) {
        var left = notice.expiresAt * 1000 - Date.now();
        if (left > 0) {
          var days = Math.floor(left / 86400000);
          var hours = Math.floor((left % 86400000) / 3600000);
          countdown = days > 0 ? days + " 天 " + hours + " 小时后截止" : hours + " 小时后截止";
        }
      }
      el.noticeBar.dataset.level = notice.level || "info";
      el.noticeBar.innerHTML =
        '<span class="notice-icon">' + icon("megaphone") + "</span>" +
        '<div class="notice-content"><strong>' + esc(notice.title) + "</strong>" +
        (notice.content ? '<span class="body">' + esc(notice.content) + "</span>" : "") +
        (countdown ? '<span class="notice-countdown">' + esc(countdown) + "</span>" : "") +
        (notice.link ? '<a class="btn btn-soft btn-sm" style="margin-left:auto" href="' + esc(notice.link) +
          '" target="_blank" rel="noopener">查看详情</a>' : "") +
        "</div>" +
        '<div class="notice-dots">' + notices.map(function (_, i) {
          return '<button type="button" aria-label="公告 ' + (i + 1) + '" class="' + (i === state.noticeIndex % notices.length ? "active" : "") + '" data-notice="' + i + '"></button>';
        }).join("") + "</div>";
      $$("[data-notice]", el.noticeBar).forEach(function (dot) {
        dot.addEventListener("click", function () {
          state.noticeIndex = Number(dot.dataset.notice);
          paint();
          restartNoticeTimer();
        });
      });
    }
    paint();
    restartNoticeTimer();
  }

  function restartNoticeTimer() {
    if (state.noticeTimer) window.clearInterval(state.noticeTimer);
    if (state.notices.length < 2) return;
    var seconds = Number(state.site.noticeInterval || 8);
    if (!seconds || seconds < 3) seconds = 8;
    state.noticeTimer = window.setInterval(function () {
      state.noticeIndex = (state.noticeIndex + 1) % state.notices.length;
      renderNotice();
    }, seconds * 1000);
  }

  function cardMarkup(card) {
    var fav = state.favorites.has(card.id);
    var tags = (card.tags || []).slice(0, 2).map(function (tag) {
      return '<span class="tag">' + esc(tag) + "</span>";
    }).join("");
    var clicks = state.site.showClicks === "0" ? "" :
      '<span class="card-clicks">' + icon("click", 12) + formatNumber(card.clicks || 0) + "</span>";
    var href = card.url || "";
    return '<article class="card' + (fav ? " is-fav" : "") + '" data-card-id="' + card.id + '">' +
      '<a class="card-open" href="' + esc(href || "#") + '"' +
      (href ? ' target="_blank" rel="noopener" data-card-url="' + esc(href) + '"' : ' aria-disabled="true"') + ">" +
      '<span class="card-top">' + iconMarkup(card, "card-icon") +
      '<span class="card-heading"><span class="card-name">' + esc(card.name) + icon("arrow", 13) + "</span>" +
      '<span class="card-host">' + esc(hostOf(href)) + "</span></span></span>" +
      (card.description ? '<span class="card-desc">' + esc(card.description) + "</span>" : '<span class="card-desc"></span>') +
      '<span class="card-foot">' + tags + clicks + "</span>" +
      "</a>" +
      '<button class="fav' + (fav ? " on" : "") + '" type="button" data-fav="' + card.id +
      '" aria-label="收藏" title="' + (fav ? "取消收藏" : "收藏") + '">' + icon("star", 15) + "</button>" +
      "</article>";
  }

  function gridMarkup(cards) {
    if (!cards.length) {
      return '<div class="empty">' + icon("sparkle") + "<span>这里还没有卡片</span></div>";
    }
    return '<div class="grid">' + cards.map(cardMarkup).join("") + "</div>";
  }

  function childHasCards(node) {
    if (cardsOf(node.id).length) return true;
    return (node.children || []).some(childHasCards);
  }

  function renderSections() {
    if (state.favOnly) {
      var favorites = state.cards.filter(function (card) { return state.favorites.has(card.id); });
      el.favBanner.classList.add("show");
      el.sections.innerHTML = '<section class="cat-section"><div class="section-head">' +
        '<div class="section-title"><span class="section-icon">★</span><div><h2>我的收藏</h2>' +
        "<p>只保存在当前浏览器</p></div></div>" +
        '<div class="section-meta"><span class="count-badge">' + favorites.length + " 个服务</span></div></div>" +
        (favorites.length ? gridMarkup(favorites) :
          '<div class="empty">' + icon("star") + "<span>还没有收藏，点击卡片右上角的星标即可收藏</span></div>") +
        "</section>";
      bindCardEvents(el.sections);
      return;
    }

    el.favBanner.classList.remove("show");
    var html = "";
    state.categories.forEach(function (root) {
      var total = countSubtree(root);
      if (!total) return;
      var inner = "";
      var direct = cardsOf(root.id);
      if (direct.length) inner += gridMarkup(direct);
      (root.children || []).forEach(function (child) {
        if (!childHasCards(child)) return;
        var childCards = cardsOf(child.id);
        inner += '<div class="subsection" id="sub-' + child.id + '">' +
          '<div class="subhead"><span class="nav-emoji">' + esc(child.icon || "•") + "</span><h3>" +
          esc(child.name) + '</h3><span class="line"></span><span class="count-badge">' +
          countSubtree(child) + "</span></div>";
        if (childCards.length) inner += gridMarkup(childCards);
        (child.children || []).forEach(function (grand) {
          var grandCards = cardsOf(grand.id);
          if (!grandCards.length) return;
          inner += '<div class="subsection"><div class="subhead"><h4>' + esc(grand.name) +
            '</h4><span class="line"></span><span class="count-badge">' + grandCards.length + "</span></div>" +
            gridMarkup(grandCards) + "</div>";
        });
        inner += "</div>";
      });

      html += '<section class="cat-section" id="cat-' + root.id + '">' +
        '<div class="section-head"><div class="section-title"><span class="section-icon">' +
        esc(root.icon || "📁") + "</span><div><h2>" + esc(root.name) + "</h2>" +
        (root.description ? "<p>" + esc(root.description) + "</p>" : "") + "</div></div>" +
        '<div class="section-meta"><span class="count-badge">' + total + " 个服务</span></div></div>" +
        inner + "</section>";
    });

    el.sections.innerHTML = html || '<div class="empty">' + icon("sparkle") +
      "<span>还没有任何服务，登录后台创建第一张卡片吧</span></div>";
    bindCardEvents(el.sections);
    observeSections();
  }

  function bindCardEvents(root) {
    bindIconFallbacks(root);
    $$("[data-fav]", root).forEach(function (button) {
      button.addEventListener("click", function (event) {
        event.preventDefault();
        event.stopPropagation();
        var id = Number(button.dataset.fav);
        if (state.favorites.has(id)) state.favorites.delete(id);
        else state.favorites.add(id);
        saveFavorites();
        var card = button.closest(".card");
        button.classList.toggle("on", state.favorites.has(id));
        button.title = state.favorites.has(id) ? "取消收藏" : "收藏";
        if (card) card.classList.toggle("is-fav", state.favorites.has(id));
        renderNav();
        if (state.favOnly) renderSections();
      });
    });
    $$("[data-card-id]", root).forEach(function (cardNode) {
      var link = $("[data-card-url]", cardNode);
      if (!link) return;
      link.addEventListener("click", function () {
        var id = cardNode.dataset.cardId;
        try {
          fetch("/api/cards/" + id + "/click", { method: "POST", keepalive: true }).catch(function () {});
        } catch (e) { /* ignore */ }
      });
    });
    $$(".card-open[aria-disabled='true']", root).forEach(function (link) {
      link.addEventListener("click", function (event) { event.preventDefault(); });
    });
  }

  function observeSections() {
    if (!("IntersectionObserver" in window)) return;
    var links = $$(".nav-item[data-target^='cat-']", el.nav);
    var map = new Map();
    links.forEach(function (link) { map.set(link.dataset.target, link); });
    var observer = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (!entry.isIntersecting) return;
        links.forEach(function (link) { link.classList.remove("active"); });
        var active = map.get(entry.target.id);
        if (active) active.classList.add("active");
        var all = $(".nav-item[data-target='top']", el.nav);
        if (all) all.classList.remove("active");
      });
    }, { rootMargin: "-96px 0px -65% 0px", threshold: 0 });
    $$(".cat-section[id]", el.sections).forEach(function (section) { observer.observe(section); });
  }

  // -------------------------------------------------------------------------
  // Search palette

  function openSearch() {
    renderPalette("");
    el.paletteOverlay.classList.add("open");
    document.body.style.overflow = "hidden";
    window.setTimeout(function () { el.paletteInput.focus(); }, 30);
  }

  function closeSearch() {
    el.paletteOverlay.classList.remove("open");
    document.body.style.overflow = "";
    el.paletteInput.value = "";
  }

  function renderPalette(term) {
    var query = term.trim().toLowerCase();
    paletteRows = [];
    var html = "";

    var cards = state.cards;
    if (!query) {
      cards = state.cards.slice().sort(function (a, b) { return (b.clicks || 0) - (a.clicks || 0); }).slice(0, 6);
      html += '<div class="palette-group">热门服务</div>';
    } else {
      cards = cards.filter(function (card) {
        var haystack = [card.name, card.description, card.url, (card.tags || []).join(" ")].join(" ").toLowerCase();
        return haystack.indexOf(query) >= 0;
      }).slice(0, 9);
      html += '<div class="palette-group">服务 · ' + cards.length + "</div>";
    }

    if (!cards.length && query) {
      html += '<div class="palette-empty">没有找到与「' + esc(term) + '」相关的服务</div>';
    }

    cards.forEach(function (card) {
      var index = paletteRows.length;
      paletteRows.push({ type: "card", card: card });
      html += '<button type="button" class="palette-row" data-index="' + index + '">' +
        iconMarkup(card, "mini-icon") +
        '<span class="palette-text"><strong>' + esc(card.name) + "</strong><span>" +
        esc(categoryPath(card.categoryId)) + " · " + esc(hostOf(card.url)) + "</span></span>" +
        '<span class="enter">↵</span></button>';
    });

    var categories = state.categories.filter(function (cat) {
      return !query || cat.name.toLowerCase().indexOf(query) >= 0;
    }).slice(0, 5);
    if (categories.length) {
      html += '<div class="palette-group">分类</div>';
      categories.forEach(function (cat) {
        var index = paletteRows.length;
        paletteRows.push({ type: "category", category: cat });
        html += '<button type="button" class="palette-row" data-index="' + index + '">' +
          '<span class="mini-icon" style="background:linear-gradient(140deg,' + toneFor(cat.name) + ",#0f172a)\">" +
          esc(cat.icon || "📁") + "</span>" +
          '<span class="palette-text"><strong>' + esc(cat.name) + "</strong><span>" +
          countSubtree(cat) + " 个服务</span></span>" +
          '<span class="enter">↵</span></button>';
      });
    }

    el.paletteBody.innerHTML = html;
    paletteIndex = 0;
    highlightPalette();
    $$(".palette-row", el.paletteBody).forEach(function (row) {
      row.addEventListener("click", function () { activatePalette(Number(row.dataset.index)); });
      row.addEventListener("mousemove", function () {
        paletteIndex = Number(row.dataset.index);
        highlightPalette();
      });
    });
  }

  function categoryPath(id) {
    var node = state.categoryMap.get(id);
    if (!node) return "未分类";
    var parts = [node.name];
    var parent = state.categoryMap.get(node.parentId);
    while (parent) {
      parts.unshift(parent.name);
      parent = state.categoryMap.get(parent.parentId);
    }
    return parts.join(" / ");
  }

  function highlightPalette() {
    $$(".palette-row", el.paletteBody).forEach(function (row) {
      row.classList.toggle("active", Number(row.dataset.index) === paletteIndex);
    });
  }

  function activatePalette(index) {
    var row = paletteRows[index];
    if (!row) return;
    if (row.type === "card") {
      var url = cardURL(row.card);
      if (url) {
        try { fetch("/api/cards/" + row.card.id + "/click", { method: "POST", keepalive: true }).catch(function () {}); } catch (e) { /* ignore */ }
        window.open(url, "_blank", "noopener");
      }
      closeSearch();
      return;
    }
    closeSearch();
    state.favOnly = false;
    renderNav();
    renderSections();
    var target = document.getElementById("cat-" + row.category.id);
    if (target) target.scrollIntoView({ behavior: "smooth", block: "start" });
    closeNav();
  }

  // -------------------------------------------------------------------------
  // Auth

  function openLogin() {
    $("#loginError").classList.remove("show");
    el.loginForm.reset();
    el.loginDialog.showModal();
    window.setTimeout(function () { $("#loginUsername").focus(); }, 60);
  }

  async function logout() {
    try {
      await fetch("/api/auth/logout", { method: "POST", headers: { "Content-Type": "application/json" } });
    } catch (e) { /* ignore */ }
    state.user = null;
    location.reload();
  }

  async function submitLogin(event) {
    event.preventDefault();
    var button = $("#loginSubmit");
    var error = $("#loginError");
    var payload = {
      username: $("#loginUsername").value.trim(),
      password: $("#loginPassword").value
    };
    if (!payload.username || !payload.password) {
      error.textContent = "请输入用户名和密码";
      error.classList.add("show");
      return;
    }
    button.disabled = true;
    button.textContent = "登录中…";
    try {
      var res = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });
      var data = await res.json().catch(function () { return {}; });
      if (!res.ok) throw new Error(data.error || "登录失败");
      state.user = data.user;
      el.loginDialog.close();
      toast("欢迎回来，" + (state.user.displayName || state.user.username), "success");
      var boot = await fetch("/api/bootstrap").then(function (r) { return r.json(); });
      state.user = boot.user;
      state.site = boot.site || state.site;
      renderUser();
      renderNav();
      applySite();
      renderSections();
    } catch (err) {
      error.textContent = err.message;
      error.classList.add("show");
    } finally {
      button.disabled = false;
      button.textContent = "登录";
    }
  }

  // -------------------------------------------------------------------------
  // Apply

  function openApply() {
    if (state.site.allowApply === "0") {
      toast("管理员已关闭卡片申请", "error");
      return;
    }
    $("#applyError").classList.remove("show");
    el.applyForm.reset();
    var select = $("#applyCategory");
    var options = [];
    state.categories.forEach(function (root) {
      var children = (root.children || []).filter(function (child) { return childHasCards(child); });
      if (!children.length) {
        options.push({ id: root.id, label: root.name });
      } else {
        children.forEach(function (child) { options.push({ id: child.id, label: root.name + " / " + child.name }); });
      }
    });
    select.innerHTML = options.map(function (opt) {
      return '<option value="' + opt.id + '">' + esc(opt.label) + "</option>";
    }).join("");
    el.applyDialog.showModal();
    window.setTimeout(function () { $("#applyName").focus(); }, 60);
  }

  async function submitApply(event) {
    event.preventDefault();
    var button = $("#applySubmit");
    var error = $("#applyError");
    var payload = {
      name: $("#applyName").value.trim(),
      url: $("#applyURL").value.trim(),
      categoryId: Number($("#applyCategory").value),
      description: $("#applyDesc").value.trim(),
      tags: $("#applyTags").value.split(/[,，]/).map(function (tag) { return tag.trim(); }).filter(Boolean)
    };
    if (!payload.name || !payload.url || !payload.categoryId) {
      error.textContent = "请填写名称、地址并选择分类";
      error.classList.add("show");
      return;
    }
    button.disabled = true;
    button.textContent = "提交中…";
    try {
      var res = await fetch("/api/apply", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });
      var data = await res.json().catch(function () { return {}; });
      if (!res.ok) throw new Error(data.error || "提交失败");
      el.applyDialog.close();
      toast(data.message || "提交成功，等待管理员审核", "success");
    } catch (err) {
      error.textContent = err.message;
      error.classList.add("show");
    } finally {
      button.disabled = false;
      button.textContent = "提交审核";
    }
  }

  // -------------------------------------------------------------------------
  // Theme, navigation and events

  function applyThemeLabel() {
    var dark = document.documentElement.getAttribute("data-theme") === "dark";
    var meta = document.querySelector('meta[name="theme-color"]');
    if (meta) meta.setAttribute("content", dark ? "#070b16" : "#f3f5fc");
  }

  function toggleTheme() {
    var next = document.documentElement.getAttribute("data-theme") === "dark" ? "light" : "dark";
    document.documentElement.setAttribute("data-theme", next);
    try { localStorage.setItem("tn-theme", next); } catch (e) { /* ignore */ }
    applyThemeLabel();
  }

  function openNav() {
    document.body.classList.add("nav-open");
    $("#menuToggle").setAttribute("aria-expanded", "true");
  }

  function closeNav() {
    document.body.classList.remove("nav-open");
    $("#menuToggle").setAttribute("aria-expanded", "false");
  }

  function bindGlobalEvents() {
    $("#themeToggle").addEventListener("click", toggleTheme);
    $("#menuToggle").addEventListener("click", function () {
      if (document.body.classList.contains("nav-open")) closeNav();
      else openNav();
    });
    el.loginButton.addEventListener("click", openLogin);
    el.applyButton.addEventListener("click", openApply);
    el.favButton.addEventListener("click", function () {
      state.favOnly = !state.favOnly;
      renderNav();
      renderSections();
      if (state.favOnly) {
        el.favBanner.classList.add("show");
        window.scrollTo({ top: 0, behavior: "smooth" });
      }
    });
    $("#clearFavFilter").addEventListener("click", function () {
      state.favOnly = false;
      renderNav();
      renderSections();
    });

    $$("[data-open-search]").forEach(function (button) {
      button.addEventListener("click", openSearch);
    });
    el.paletteInput.addEventListener("input", function () {
      renderPalette(el.paletteInput.value);
    });
    el.paletteOverlay.addEventListener("mousedown", function (event) {
      if (event.target === el.paletteOverlay) closeSearch();
    });

    el.loginForm.addEventListener("submit", submitLogin);
    el.applyForm.addEventListener("submit", submitApply);
    $$("[data-close-dialog]").forEach(function (button) {
      button.addEventListener("click", function () {
        var dialog = button.closest("dialog");
        if (dialog) dialog.close();
      });
    });

    document.addEventListener("keydown", function (event) {
      var meta = event.ctrlKey || event.metaKey;
      if (meta && event.key.toLowerCase() === "k") {
        event.preventDefault();
        if (el.paletteOverlay.classList.contains("open")) closeSearch();
        else openSearch();
        return;
      }
      if (!el.paletteOverlay.classList.contains("open")) {
        if (event.key === "Escape") closeNav();
        return;
      }
      if (event.key === "Escape") {
        event.preventDefault();
        closeSearch();
      } else if (event.key === "ArrowDown") {
        event.preventDefault();
        paletteIndex = Math.min(paletteIndex + 1, paletteRows.length - 1);
        highlightPalette();
        scrollPalette();
      } else if (event.key === "ArrowUp") {
        event.preventDefault();
        paletteIndex = Math.max(paletteIndex - 1, 0);
        highlightPalette();
        scrollPalette();
      } else if (event.key === "Enter") {
        event.preventDefault();
        activatePalette(paletteIndex);
      }
    });

    document.addEventListener("click", function (event) {
      if (document.body.classList.contains("nav-open") &&
        !event.target.closest(".sidebar") && !event.target.closest("#menuToggle")) {
        closeNav();
      }
    });
  }

  function scrollPalette() {
    var row = $('.palette-row[data-index="' + paletteIndex + '"]', el.paletteBody);
    if (row) row.scrollIntoView({ block: "nearest" });
  }

  init();
})();