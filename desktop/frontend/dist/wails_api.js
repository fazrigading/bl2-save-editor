// Wails transport shim: replaces the fetch-based api() from app.js.
// Loaded after app.js so this override wins; every backend call then goes
// through the Go bridge instead of HTTP.
(function () {
    function backend() {
        return window.go.bridge.Bridge;
    }

    async function api(url, opts = {}) {
        const method = (opts.method || "GET").toUpperCase();
        let body = null;
        if (opts.body != null) {
            body = typeof opts.body === "string" ? opts.body : JSON.stringify(opts.body);
        }
        let raw;
        try {
            raw = await backend().Invoke(method, url, body);
        } catch (err) {
            const msg = (err && err.message) ? err.message : String(err);
            toast(msg, "error");
            throw new Error(msg);
        }
        let json;
        try {
            json = JSON.parse(raw);
        } catch (e) {
            const msg = "Backend returned non-JSON response";
            toast(msg, "error");
            throw new Error(msg);
        }
        if (json && json.error) {
            const msg = json.error;
            toast(msg, "error");
            throw new Error(msg);
        }
        return json;
    }

    window.api = api;
})();
