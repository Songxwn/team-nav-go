/* global window, document, localStorage, navigator */
/*
 * Tiny dependency-free i18n layer shared by the public site and the admin
 * console.
 *
 * Source strings are written in Chinese. English lives in a phrase catalog so
 * the same catalog can translate static markup, dynamically rendered nodes and
 * toast/dialog messages. `t()` is used for strings with interpolation; the DOM
 * walker handles everything else and remembers the original text so switching
 * back to Chinese is lossless.
 */
(function () {
  "use strict";

  var STORAGE_KEY = "tn-lang";
  var COOKIE_KEY = "tn_lang";

  var english = {
    // ---------------- Public shell ----------------
    "星云导航": "Nebula Nav",
    "星云导航 · 团队服务": "Nebula Nav · Team Services",
    "团队服务": "Team Services",
    "团队服务导航 · 一站式访问团队的全部工具与平台": "Team service directory — every tool and platform in one place",
    "团队服务，一站式入口": "All team services in one place",
    "今天也要高效出发": "Let's make today count",
    "Built with Go · SQLite": "Built with Go · SQLite",
    "导航": "Navigation",
    "分类导航": "Category navigation",
    "全局搜索": "Global search",
    "搜索服务或分类": "Search services or categories",
    "搜索服务、分类或关键词…": "Search services, categories, or keywords…",
    "全部服务": "All services",
    "我的收藏": "Favorites",
    "进入后台": "Admin console",
    "登录": "Sign in",
    "退出登录": "Sign out",
    "切换主题": "Toggle theme",
    "切换深浅色主题": "Toggle light or dark theme",
    "切换语言": "Switch language",
    "打开导航": "Open navigation",
    "关闭": "Close",
    "取消": "Cancel",
    "确认": "Confirm",
    "打开": "open",
    "选择": "select",
    "打开后台": "Open console",
    "管理后台": "Admin console",
    "管理后台 · 星云导航": "Admin console · Nebula Nav",
    "管理后台 · {name}": "Admin console · {name}",
    "管理控制台": "Admin console",
    "控制台 /": "Console / ",
    "菜单": "Menu",
    "查看站点": "View site",
    "返回导航首页": "Back to homepage",
    "管理": "Manage",
    "概览": "Overview",
    "卡片管理": "Cards",
    "分类管理": "Categories",
    "通知公告": "Notices",
    "用户与权限": "Users & roles",
    "图片库": "Images",
    "系统设置": "Settings",

    // ---------------- Public hero / stats ----------------
    "WELCOME · 星云导航": "WELCOME · Nebula Nav",
    "把团队的每一个服务": "Every service your team uses",
    "放在顺手的位置": "right at your fingertips",
    "个服务": "services",
    "个分类": "categories",
    "次访问": "visits",
    "快速查找，试试「监控」": "Quick find — try “monitoring”",
    "提交卡片": "Submit a card",
    "收藏保存在浏览器本地；按下": "Favorites are stored in this browser. Press",
    "随时唤起全局搜索。": "to open global search anytime.",
    "正在查看我的收藏": "Viewing your favorites",
    "显示全部": "Show all",
    "热门服务": "Popular services",
    "分类": "Categories",
    "服务": "Services",
    "未分类": "Uncategorized",
    "查看详情": "View details",
    "公告": "Notice",
    "收藏": "Favorite",
    "取消收藏": "Unfavorite",
    "这里还没有卡片": "No cards here yet",
    "只保存在当前浏览器": "Stored in this browser only",
    "还没有收藏，点击卡片右上角的星标即可收藏": "No favorites yet. Tap the star on a card to add one.",
    "还没有任何服务，登录后台创建第一张卡片吧": "No services yet. Sign in to the console and create your first card.",
    "加载失败，请刷新页面重试": "Failed to load. Please refresh the page.",
    "还没有访问数据": "No visit data yet",
    "暂无点击数据": "No click data yet",
    "全部状态": "All statuses",
    "全部分类": "All categories",
    "顶级分类": "Top level",

    // ---------------- Auth / apply ----------------
    "登录工作台": "Sign in to the console",
    "管理员可以管理卡片、分类与站点设置": "Admins manage cards, categories, and site settings",
    "用户名": "Username",
    "密码": "Password",
    "请输入用户名": "Enter your username",
    "请输入密码": "Enter your password",
    "还没有账号？请联系管理员分配权限": "No account yet? Ask an administrator for access.",
    "提交新卡片": "Submit a new card",
    "审核通过后会出现在导航首页": "It appears on the homepage after approval",
    "服务名称": "Service name",
    "例如：Grafana": "e.g. Grafana",
    "访问地址": "URL",
    "所属分类": "Category",
    "标签": "Tags",
    "用逗号分隔，如：监控, 运维": "Comma separated, e.g. monitoring, ops",
    "最多 6 个标签": "Up to 6 tags",
    "一句话描述": "Short description",
    "这个服务是做什么的？": "What is this service for?",
    "提交审核": "Submit for review",
    "提交成功，等待管理员审核": "Submitted. An administrator will review it.",
    "管理员已关闭卡片申请": "Card submissions are disabled.",
    "管理员还没有创建分类，暂时无法提交": "No categories yet, so submissions are unavailable.",
    "请填写名称、地址并选择分类": "Fill in the name and URL, and pick a category.",
    "请输入用户名和密码": "Enter your username and password.",
    "登录失败": "Sign in failed",
    "登录中…": "Signing in…",
    "提交失败": "Submission failed",
    "提交中…": "Submitting…",
    "欢迎回来，{name}": "Welcome back, {name}",
    "访客模式": "Guest mode",
    "登录后可提交卡片": "Sign in to submit cards",
    "管理员 · 已登录": "Admin · signed in",
    "成员 · 已登录": "Member · signed in",
    "公告 {n}": "Notice {n}",
    "服务 · {n}": "Services · {n}",
    "{days} 天 {hours} 小时后截止": "Ends in {days}d {hours}h",
    "{hours} 小时后截止": "Ends in {hours}h",
    "没有找到与「{q}」相关的服务": "No services match “{q}”",
    "加载数据失败：{msg}": "Failed to load data: {msg}",
    "已登录": "Signed in",

    // ---------------- Admin overview ----------------
    "服务卡片": "Service cards",
    "{n} 个待审核": "{n} pending",
    "最多三级结构": "Up to three levels",
    "累计点击": "Total clicks",
    "来自全部卡片": "Across all cards",
    "今日访问": "Visits today",
    "首页打开次数": "Homepage opens",
    "用户": "Users",
    "含管理员账号": "Including admins",
    "数据库": "Database",
    "{n} 张图片": "{n} images",
    "最近 7 天访问": "Visits in the last 7 days",
    "每天打开导航首页的次数": "Homepage opens per day",
    "实时统计": "Live",
    "按累计点击排序": "Sorted by total clicks",
    "最近添加": "Recently added",
    "最新创建的卡片": "Newest cards",
    "管理全部": "Manage all",
    "状态": "Status",
    "创建时间": "Created",
    "还没有卡片": "No cards yet",

    // ---------------- Admin cards ----------------
    "搜索名称、描述或标签": "Search name, description, or tags",
    "已上线": "Live",
    "待审核": "Pending",
    "新建卡片": "New card",
    "排序": "Order",
    "点击": "Clicks",
    "操作": "Actions",
    "上移": "Move up",
    "下移": "Move down",
    "未设置链接": "No link set",
    "审核通过": "Approve",
    "编辑": "Edit",
    "删除": "Delete",
    "没有匹配的卡片": "No matching cards",
    "卡片已删除": "Card deleted",
    "卡片已更新": "Card updated",
    "卡片已创建": "Card created",
    "已通过审核": "Approved",
    "删除卡片？": "Delete card?",
    "「{name}」将被永久删除。": "“{name}” will be permanently deleted.",
    "编辑卡片": "Edit card",
    "名称": "Name",
    "图标类型": "Icon type",
    "文字": "Text",
    "上传图片": "Uploaded image",
    "图片链接": "Image URL",
    "图标内容": "Icon content",
    "🚀 或 /api/images/xxx": "🚀 or /api/images/xxx",
    "描述": "Description",
    "保存卡片": "Save card",
    "图标支持 Emoji、文字、图片链接或上传": "Icons can be Emoji, text, image URLs, or uploads",
    "数字越小越靠前，也可以保存后在列表中用箭头调整": "Smaller numbers sort first. You can also reorder with the arrows after saving.",
    "请填写名称并选择分类": "Enter a name and choose a category.",

    // ---------------- Admin categories ----------------
    "最多支持三级分类，删除分类会同时移除其下的卡片。": "Up to three levels. Deleting a category also removes its cards.",
    "新建分类": "New category",
    "分类名称": "Category name",
    "层级": "Level",
    "卡片": "Cards",
    "可见": "Visible",
    "隐藏": "Hidden",
    "还没有分类": "No categories yet",
    "第 {n} 级": "Level {n}",
    "分类已创建": "Category created",
    "分类已更新": "Category updated",
    "分类已删除": "Category deleted",
    "删除分类？": "Delete category?",
    "「{name}」及其子分类、卡片都会被删除。": "“{name}” and its subcategories and cards will be deleted.",
    "编辑分类": "Edit category",
    "用于组织首页卡片，最多三级": "Organize homepage cards in up to three levels",
    "图标": "Icon",
    "监控, 运维": "monitoring, ops",
    "上级分类": "Parent category",
    "排序值": "Sort order",
    "首页可见": "Visible on homepage",
    "关闭后仅后台可见": "When off, only admins can see it",
    "保存分类": "Save category",
    "请填写分类名称": "Enter a category name.",

    // ---------------- Admin notices ----------------
    "公告会显示在首页顶部，支持定时切换与倒计时。": "Notices appear at the top of the homepage with rotation and countdown.",
    "新建公告": "New notice",
    "级别": "Level",
    "截止时间": "Expires",
    "还没有公告": "No notices yet",
    "信息": "Info",
    "成功": "Success",
    "提醒": "Warning",
    "重要": "Important",
    "长期": "No expiry",
    "启用": "Active",
    "停用": "Disabled",
    "编辑公告": "Edit notice",
    "显示在首页顶部，支持倒计时": "Shown at the top of the homepage with a countdown",
    "标题": "Title",
    "内容": "Content",
    "跳转链接": "Link",
    "https://…（可选）": "https://… (optional)",
    "留空表示长期有效": "Leave empty for no expiry",
    "关闭后不显示": "When off, it stays hidden",
    "保存公告": "Save notice",
    "公告已创建": "Notice created",
    "公告已更新": "Notice updated",
    "公告已删除": "Notice deleted",
    "删除公告？": "Delete notice?",
    "「{title}」将不再显示。": "“{title}” will no longer be shown.",
    "请填写公告标题": "Enter a notice title.",

    // ---------------- Admin users ----------------
    "管理员可进入后台；普通成员只能浏览和提交卡片。": "Admins can access the console; members can browse and submit cards.",
    "新建用户": "New user",
    "角色": "Role",
    "还没有用户": "No users yet",
    "管理员": "Admin",
    "普通成员": "Member",
    "重置密码": "Reset password",
    "删除用户？": "Delete user?",
    "「{name}」将无法再登录。": "“{name}” will no longer be able to sign in.",
    "为 {name} 设置新密码": "Set a new password for {name}",
    "用户已删除": "User deleted",
    "用户已更新": "User updated",
    "用户已创建": "User created",
    "编辑用户": "Edit user",
    "分配后台管理或浏览权限": "Grant console access or browsing only",
    "3-32 位字母、数字或 _ . -": "3-32 letters, digits, or _ . -",
    "初始密码": "Initial password",
    "至少 8 位": "At least 8 characters",
    "显示名称": "Display name",
    "保存用户": "Save user",
    "请填写用户名和初始密码": "Enter a username and initial password.",
    "密码已重置": "Password reset",
    "密码至少 8 位": "Password must be at least 8 characters.",
    "新密码": "New password",
    "确认重置": "Reset password",
    "为用户设置新密码": "Set a new password for this user",

    // ---------------- Admin images ----------------
    "拖拽图片到这里，或点击上传": "Drop images here, or click to upload",
    "支持 PNG / JPEG / GIF / WebP / AVIF / ICO，单张不超过 2MB，自动按内容去重": "PNG / JPEG / GIF / WebP / AVIF / ICO, up to 2 MB each, deduplicated by content",
    "已上传图片": "Uploaded images",
    "图片保存在 SQLite，并由内存 LRU 缓存加速": "Images live in SQLite and are accelerated by an in-memory LRU cache",
    "还没有上传图片": "No images uploaded yet",
    "复制地址": "Copy URL",
    "删除图片？": "Delete image?",
    "引用该图片的卡片图标会失效。": "Cards using this image will lose their icon.",
    "图片已删除": "Image deleted",
    "图片地址已复制": "Image URL copied",
    "图片上传成功": "Image uploaded",
    "图片已存在，直接复用": "Image already exists and was reused",
    "图片不能超过 2MB": "Images must be 2 MB or smaller.",
    "上传": "Upload",
    "0 张": "0 images",

    // ---------------- Admin settings ----------------
    "站点信息": "Site settings",
    "这些内容会展示在导航首页": "These appear on the public homepage",
    "站点名称": "Site name",
    "副标题": "Subtitle",
    "欢迎语": "Welcome message",
    "界面语言": "Interface language",
    "访客首次访问时使用该语言，可在页面右上角切换": "Used for first-time visitors. Anyone can switch it from the header.",
    "简体中文": "简体中文",
    "主题色": "Accent color",
    "星云紫": "Nebula Indigo",
    "翡翠绿": "Emerald",
    "玫瑰红": "Rose",
    "琥珀金": "Amber",
    "深海蓝": "Ocean",
    "公告切换间隔（秒）": "Notice rotation (seconds)",
    "登录会话有效期（天）": "Session lifetime (days)",
    "默认 66 天，可设置 1-365 天；修改仅对之后的新登录生效，已登录会话保持原有效期": "Default 66 days, range 1-365. Changes apply to new sign-ins only; existing sessions keep their expiry.",
    "站点 LOGO": "Site logo",
    "图片地址，或点击右侧上传": "Image URL, or upload on the right",
    "页脚文字": "Footer text",
    "允许访客提交卡片": "Allow visitors to submit cards",
    "提交后进入待审核列表": "Submissions go to the pending queue",
    "在卡片上展示点击次数": "Show click counts on cards",
    "关闭后前台隐藏统计数字": "Hide click counts on the public site",
    "保存设置": "Save settings",
    "设置已保存": "Settings saved",
    "修改我的密码": "Change my password",
    "定期更换密码更安全": "Rotate your password regularly",
    "当前密码": "Current password",
    "更新密码": "Update password",
    "密码已更新": "Password updated",
    "新密码至少 8 位": "New password must be at least 8 characters.",
    "数据备份": "Data backup",
    "SQLite 数据库包含卡片、设置与图片": "The SQLite database contains cards, settings, and images",
    "下载快照后请妥善保存。恢复时停止服务，用备份文件替换 data/team-nav.db 即可。": "Keep the snapshot safe. To restore, stop the service and replace data/team-nav.db with it.",
    "下载数据库快照": "Download database snapshot",
    "单二进制部署，内嵌前端与图片缓存。版本": "Single binary with embedded frontend and image cache. Version",
    "关于": "About",
    "GitHub 仓库": "GitHub repository",
    "GitHub 仓库 · Songxwn/team-nav-go": "GitHub repo · Songxwn/team-nav-go",
    "打开 GitHub 仓库": "Open the GitHub repository",

    // ---------------- Shared states / server fallbacks ----------------
    "确认删除？": "Confirm deletion?",
    "此操作无法撤销": "This action cannot be undone",
    "确认删除": "Delete",
    "删除图片": "Delete image",
    "删除公告": "Delete notice",
    "删除分类": "Delete category",
    "删除卡片": "Delete card",
    "删除用户": "Delete user",
    "请求失败（{status}）": "Request failed ({status})",
    "需要登录": "Sign-in required",
    "未知错误": "Unknown error",
    "已复制": "Copied",
    "暂无数据": "No data yet",
    "返回": "Back"
  };

  var lang = detect();
  var textOriginals = new WeakMap();
  var attrOriginals = new WeakMap();
  var changeHandler = null;

  function normalize(value) {
    return value === "en" ? "en" : "zh";
  }

  function stored() {
    try {
      var value = localStorage.getItem(STORAGE_KEY);
      return value === "en" || value === "zh" ? value : null;
    } catch (e) {
      return null;
    }
  }

  function queryLang() {
    try {
      var query = new URLSearchParams(location.search).get("lang");
      if (query === "en" || query === "zh") return query;
    } catch (e) {
      /* no URLSearchParams */
    }
    return null;
  }

  function detect() {
    var query = queryLang();
    if (query) return query;
    var saved = stored();
    if (saved) return saved;
    var nav = String(navigator.language || navigator.userLanguage || "").toLowerCase();
    return nav.indexOf("zh") === 0 ? "zh" : "en";
  }

  function cookie(value) {
    try {
      document.cookie = COOKIE_KEY + "=" + value + "; path=/; max-age=31536000; SameSite=Lax";
    } catch (e) {
      /* cookies disabled */
    }
  }

  function hasStoredChoice() {
    return stored() !== null;
  }

  // hasExplicitChoice is true when the visitor picked a language or followed a
  // ?lang= link, in which case the site default should not override it.
  function hasExplicitChoice() {
    return queryLang() !== null || stored() !== null;
  }

  // t translates a source phrase. Placeholders use {name} syntax.
  function t(phrase, vars) {
    var value = lang === "en" ? (english[phrase] || phrase) : phrase;
    if (vars) {
      value = value.replace(/\{(\w+)\}/g, function (match, key) {
        return vars[key] == null ? "" : String(vars[key]);
      });
    }
    return value;
  }

  // plural picks the English singular/plural form while keeping Chinese simple.
  function plural(n, zhPhrase, enOne, enMany) {
    var template = lang === "en" ? (n === 1 ? enOne : enMany) : zhPhrase;
    return template.replace(/\{n\}/g, String(n));
  }

  function locale() {
    return lang === "en" ? "en-US" : "zh-CN";
  }

  function translateTextNode(node) {
    var raw = node.nodeValue == null ? "" : node.nodeValue;
    var original = textOriginals.get(node);

    // A node can be reused with new text (titles, counters, etc.). Drop the
    // remembered original when the current text is neither the original nor
    // the translation we produced, so stale copies never overwrite fresh text.
    if (original != null) {
      var prevKey = original.trim();
      var prevLead = (original.match(/^\s*/) || [""])[0];
      var prevTrail = (original.match(/\s*$/) || [""])[0];
      var expectedEn = prevLead + english[prevKey] + prevTrail;
      var valid = (prevKey in english) &&
        (raw === original || raw === expectedEn || raw === english[prevKey]);
      if (!valid) {
        textOriginals.delete(node);
        original = null;
      }
    }

    if (original == null) {
      if (!/[\u4e00-\u9fff]/.test(raw)) return;
      var key = raw.trim();
      if (!key || !(key in english)) return;
      original = raw;
      textOriginals.set(node, original);
    }
    var baseKey = original.trim();
    if (!(baseKey in english)) return;
    if (lang === "en") {
      var lead = (original.match(/^\s*/) || [""])[0];
      var trail = (original.match(/\s*$/) || [""])[0];
      node.nodeValue = lead + english[baseKey] + trail;
    } else {
      node.nodeValue = original;
    }
  }

  function translateAttribute(el, attr) {
    var current = el.getAttribute(attr);
    var map = attrOriginals.get(el);
    if (!map) {
      map = {};
      attrOriginals.set(el, map);
    }
    if (attr in map) {
      var prev = map[attr];
      var prevKey = prev.trim();
      if (!(prevKey in english) || (current !== prev && current !== english[prevKey])) {
        delete map[attr];
      }
    }
    if (!(attr in map)) {
      if (!current || !/[\u4e00-\u9fff]/.test(current)) return;
      map[attr] = current;
    }
    var original = map[attr];
    var key = original.trim();
    if (!key || !(key in english)) return;
    el.setAttribute(attr, lang === "en" ? t(key) : original);
  }

  function localize(root) {
    if (!root) return;
    if (root.nodeType === 3) {
      translateTextNode(root);
      return;
    }
    var doc = root.ownerDocument || document;
    var walker = doc.createTreeWalker(root, 4 /* NodeFilter.SHOW_TEXT */, null);
    var node;
    while ((node = walker.nextNode())) translateTextNode(node);
    if (root.querySelectorAll) {
      var nodes = root.querySelectorAll("[placeholder], [title], [aria-label]");
      Array.prototype.forEach.call(nodes, function (el) {
        if (el.hasAttribute("placeholder")) translateAttribute(el, "placeholder");
        if (el.hasAttribute("title")) translateAttribute(el, "title");
        if (el.hasAttribute("aria-label")) translateAttribute(el, "aria-label");
      });
    }
  }

  function applyDocument() {
    document.documentElement.lang = lang === "en" ? "en" : "zh-CN";
    cookie(lang);
    localize(document.body);

    // <title> and the meta description live in <head>, outside the body
    // localizer, so they are handled explicitly. The original text is kept so
    // switching back works; both apps may overwrite the title afterwards.
    var titleEl = document.querySelector("title");
    if (titleEl) {
      var titleOriginal = titleEl.getAttribute("data-i18n-original");
      if (titleOriginal == null) {
        titleOriginal = titleEl.textContent || "";
        titleEl.setAttribute("data-i18n-original", titleOriginal);
      }
      titleEl.textContent = lang === "en" ? t(titleOriginal) : titleOriginal;
    }

    var desc = document.querySelector('meta[name="description"]');
    if (desc) {
      var original = desc.getAttribute("data-i18n-original");
      if (original == null) {
        original = desc.getAttribute("content") || "";
        desc.setAttribute("data-i18n-original", original);
      }
      if (original) {
        desc.setAttribute("content", lang === "en" ? t(original) : original);
      }
    }
  }

  function setLang(next, options) {
    var opts = options || {};
    lang = normalize(next);
    if (opts.persist !== false) {
      try {
        localStorage.setItem(STORAGE_KEY, lang);
      } catch (e) {
        /* private mode */
      }
    }
    if (opts.silent) {
      cookie(lang);
      document.documentElement.lang = lang === "en" ? "en" : "zh-CN";
    } else {
      applyDocument();
      if (changeHandler) changeHandler(lang);
    }
  }

  window.I18N = {
    t: t,
    plural: plural,
    locale: locale,
    lang: function () { return lang; },
    setLang: setLang,
    localize: localize,
    hasStoredChoice: hasStoredChoice,
    hasExplicitChoice: hasExplicitChoice,
    onChange: function (handler) { changeHandler = handler; }
  };

  applyDocument();
})();