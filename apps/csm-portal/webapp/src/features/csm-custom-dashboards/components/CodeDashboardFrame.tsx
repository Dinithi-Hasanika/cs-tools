// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { Alert, Box } from "@wso2/oxygen-ui";
import { type JSX, useEffect, useRef, useState } from "react";
import { useCodeDashboardPortal } from "@features/csm-custom-dashboards/api/useCodeDashboardPortal";

/**
 * The document loaded into the iframe: an empty `#root`, a `portal` whose every
 * call is a postMessage to this page, and a runner that turns the received
 * source into its `render` function and calls it.
 *
 * EXPERIMENTAL (POC) EXCEPTION to the webapp CLAUDE.md's "no eval, new
 * Function, or dynamic <script>" rule, agreed for the proof of concept only:
 * the runner is an inline script and uses `new Function`. It runs only inside
 * the sandboxed iframe below (opaque origin: no access to this page, its
 * cookies, storage or tokens). No CSP is applied yet. Must be revisited (a
 * separate sandbox origin, a CSP) before this goes beyond the POC.
 */
const RUNNER = `<!doctype html><html><head><meta charset="utf-8">
<style>body{font:14px/1.45 system-ui,sans-serif;margin:12px;color:#1d2330;background:#fff}</style></head>
<body><div id="root"></div><script>
let next = 0; const pending = new Map();
const call = (fn, args) => new Promise((resolve, reject) => {
  const id = ++next; pending.set(id, { resolve, reject });
  parent.postMessage({ type: "call", id, fn, args: JSON.parse(JSON.stringify(args ?? null)) }, "*");
});
const portal = new Proxy({}, { get: (_, fn) => (args) => call(String(fn), args) });
addEventListener("message", async (e) => {
  if (e.source !== parent) return;
  const m = e.data || {};
  if (m.type === "result") {
    const p = pending.get(m.id); if (!p) return; pending.delete(m.id);
    m.error ? p.reject(new Error(m.error)) : p.resolve(m.value);
  } else if (m.type === "run") {
    try {
      const render = new Function(m.source + "\\nreturn typeof render === 'function' ? render : undefined;")();
      if (!render) throw new Error("the code does not define a render(portal, root) function");
      await render(portal, document.getElementById("root"));
    } catch (err) {
      parent.postMessage({ type: "error", message: String((err && err.message) || err) }, "*");
    }
  }
});
parent.postMessage({ type: "ready" }, "*");
</script></body></html>`;

interface FrameMessage {
  type?: string;
  id?: number;
  fn?: string;
  args?: unknown;
  message?: string;
}

/**
 * Experimental (POC): runs a code dashboard's `render(portal, root)` inside a
 * sandboxed iframe (`sandbox="allow-scripts"` only) and answers its `portal.*`
 * calls from this page. Only the names `useCodeDashboardPortal` returns are
 * answered (all with real data); any other call is refused. An error thrown
 * by the code is shown above the frame.
 *
 * Mount it with `key={source}` so a new source gets a fresh iframe and state.
 */
export default function CodeDashboardFrame({ source }: { source: string }): JSX.Element {
  const frameRef = useRef<HTMLIFrameElement>(null);
  const [error, setError] = useState<string | null>(null);
  const portal = useCodeDashboardPortal();

  useEffect(() => {
    const onMessage = (event: MessageEvent<FrameMessage>): void => {
      const frame = frameRef.current?.contentWindow;
      if (!frame || event.source !== frame) return; // only our own iframe
      const msg = event.data ?? {};
      if (msg.type === "ready") {
        frame.postMessage({ type: "run", source }, "*");
      } else if (msg.type === "error") {
        setError(msg.message ?? "The dashboard code failed.");
      } else if (msg.type === "call" && typeof msg.fn === "string") {
        const handler = Object.hasOwn(portal, msg.fn) ? portal[msg.fn] : undefined;
        if (!handler) {
          frame.postMessage({ type: "result", id: msg.id, error: `portal.${msg.fn} is not available` }, "*");
          return;
        }
        Promise.resolve()
          .then(() => handler(msg.args))
          .then(
            (value) => frame.postMessage({ type: "result", id: msg.id, value }, "*"),
            (err: unknown) => frame.postMessage({ type: "result", id: msg.id, error: String(err) }, "*"),
          );
      }
    };
    window.addEventListener("message", onMessage);
    return () => window.removeEventListener("message", onMessage);
  }, [source, portal]);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1, flex: 1, minHeight: 0 }}>
      {error && <Alert severity="error">The dashboard code failed: {error}</Alert>}
      <Box
        component="iframe"
        ref={frameRef}
        title="Generated dashboard"
        sandbox="allow-scripts"
        srcDoc={RUNNER}
        sx={{ flex: 1, minHeight: 0, width: "100%", border: 0, borderRadius: 1, bgcolor: "#fff" }}
      />
    </Box>
  );
}
