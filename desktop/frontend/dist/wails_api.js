// Wails first-run check: if no valid config exists yet, switch to the setup
// screen. (The api() transport lives in app.js itself and talks to the Go
// bridge via window.go.bridge.Bridge.)
(function () {
    document.addEventListener("DOMContentLoaded", async function () {
        if (window.location.pathname.indexOf("setup") !== -1) return;
        if (!window.go || !window.go.bridge || !window.go.bridge.Bridge) return;
        try {
            var raw = await window.go.bridge.Bridge.Invoke("GET", "/api/configured", null);
            var res = JSON.parse(raw);
            if (!res.configured) window.location.href = "/setup.html";
        } catch (e) {}
    });
})();
