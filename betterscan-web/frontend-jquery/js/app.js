/* BetterScan jQuery frontend - shared helpers.
 *
 * Deliberately dependency-light: plain jQuery + Bootstrap, no build step.
 * The backend API is the same one the Next.js frontend uses; we just store the
 * JWT in localStorage and attach it as a Bearer token on every request.
 */
(function (window) {
  "use strict";

  // API base URL. Override by setting window.BETTERSCAN_API before this script
  // loads (see config.js) or via a <meta name="betterscan-api"> tag.
  var API_BASE =
    window.BETTERSCAN_API ||
    (function () {
      var meta = document.querySelector('meta[name="betterscan-api"]');
      return (meta && meta.getAttribute("content")) || "http://localhost:8080";
    })();

  var TOKEN_KEY = "betterscan_token";
  var USER_KEY = "betterscan_user";

  function getToken() {
    return window.localStorage.getItem(TOKEN_KEY);
  }

  function getUser() {
    try {
      return JSON.parse(window.localStorage.getItem(USER_KEY) || "null");
    } catch (e) {
      return null;
    }
  }

  function setSession(token, user) {
    window.localStorage.setItem(TOKEN_KEY, token);
    window.localStorage.setItem(USER_KEY, JSON.stringify(user || null));
  }

  function clearSession() {
    window.localStorage.removeItem(TOKEN_KEY);
    window.localStorage.removeItem(USER_KEY);
  }

  // Redirect to the login page if no token is present. Returns true if authed.
  function requireAuth() {
    if (!getToken()) {
      window.location.href = "login.html";
      return false;
    }
    return true;
  }

  // For login/register pages: bounce to the dashboard if already signed in.
  function redirectIfAuthed() {
    if (getToken()) {
      window.location.href = "index.html";
    }
  }

  // jQuery AJAX wrapper that attaches the bearer token and handles 401 by
  // clearing the session and returning to login.
  function api(path, options) {
    options = options || {};
    var settings = {
      url: path.indexOf("http") === 0 ? path : API_BASE + path,
      method: options.method || "GET",
      dataType: "json",
      headers: {},
    };
    var token = getToken();
    if (token) {
      settings.headers.Authorization = "Bearer " + token;
    }
    if (options.body !== undefined) {
      settings.contentType = "application/json";
      settings.data = JSON.stringify(options.body);
    }

    return $.ajax(settings).fail(function (xhr) {
      if (xhr.status === 401 && !options.noRedirect) {
        clearSession();
        window.location.href = "login.html";
      }
    });
  }

  // Extract a human-readable error message from a failed jQuery xhr.
  function errorMessage(xhr, fallback) {
    if (xhr && xhr.responseJSON && xhr.responseJSON.error) {
      return xhr.responseJSON.error;
    }
    if (xhr && xhr.status === 0) {
      return "Could not reach the server.";
    }
    return fallback || "Something went wrong.";
  }

  // Minimal HTML escaping for any value rendered into the DOM.
  function esc(value) {
    return $("<div>").text(value == null ? "" : String(value)).html();
  }

  // Render the shared top navigation into #nav. activePage is one of
  // "dashboard" | "projects".
  function renderNav(activePage) {
    var user = getUser();
    var initial = user && user.name ? user.name.charAt(0).toUpperCase() : "U";
    var label = user ? esc(user.name || user.email) : "";

    var html =
      '<nav class="navbar navbar-expand-lg navbar-light bg-white border-bottom">' +
      '  <div class="container-fluid" style="max-width:80rem;">' +
      '    <a class="navbar-brand fw-bold" href="index.html">BetterScan</a>' +
      '    <button class="navbar-toggler" type="button" data-bs-toggle="collapse" data-bs-target="#navItems">' +
      '      <span class="navbar-toggler-icon"></span>' +
      "    </button>" +
      '    <div class="collapse navbar-collapse" id="navItems">' +
      '      <ul class="navbar-nav me-auto">' +
      navLink("index.html", "Dashboard", activePage === "dashboard") +
      navLink("projects.html", "Projects", activePage === "projects") +
      navLink("integrations.html", "Integrations", activePage === "integrations") +
      "      </ul>" +
      '      <div class="d-flex align-items-center gap-2">' +
      '        <span class="rounded-circle bg-primary text-white d-inline-flex align-items-center justify-content-center" style="width:32px;height:32px;font-size:0.85rem;">' +
      esc(initial) +
      "</span>" +
      '        <span class="text-muted small d-none d-sm-inline">' + label + "</span>" +
      '        <button id="logoutBtn" class="btn btn-outline-secondary btn-sm">Log out</button>' +
      "      </div>" +
      "    </div>" +
      "  </div>" +
      "</nav>";

    $("#nav").html(html);

    $("#logoutBtn").on("click", function () {
      api("/api/v1/auth/logout", { method: "POST", noRedirect: true }).always(
        function () {
          clearSession();
          window.location.href = "login.html";
        }
      );
    });
  }

  function navLink(href, label, active) {
    return (
      '<li class="nav-item">' +
      '<a class="nav-link' +
      (active ? " active fw-semibold" : "") +
      '" href="' +
      href +
      '">' +
      esc(label) +
      "</a></li>"
    );
  }

  window.BetterScan = {
    API_BASE: API_BASE,
    getToken: getToken,
    getUser: getUser,
    setSession: setSession,
    clearSession: clearSession,
    requireAuth: requireAuth,
    redirectIfAuthed: redirectIfAuthed,
    api: api,
    errorMessage: errorMessage,
    esc: esc,
    renderNav: renderNav,
  };
})(window);
