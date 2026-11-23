(function () {
  try {
    var saved = localStorage.getItem("tn-theme");
    var theme = saved || (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
    document.documentElement.setAttribute("data-theme", theme);
  } catch (e) {
    /* ignore */
  }
})();