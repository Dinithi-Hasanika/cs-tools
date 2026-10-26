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

/**
 * Experimental (POC): the drawing kit a code dashboard runs with. The sandbox
 * runner (CodeDashboardFrame) loads this source into the iframe before the
 * generated code, so `render(portal, root)` can call these names without
 * writing them: one stylesheet and the same helpers for every dashboard, and
 * far less for the model to write (it used to copy all of this into each
 * reply).
 *
 * The code catalogue (backend `internal/dashboard/catalogue/code_catalogue.md`,
 * section 7) documents these names and signatures for the model; keep the two
 * in step (`DASHBOARD_KIT_NAMES` here, the list in the backend's
 * `catalogue_test.go`).
 *
 * Plain JavaScript in a raw string: it runs only inside the sandboxed iframe,
 * never in the portal page. It must contain no backtick, no `${` and no
 * closing script tag.
 */
export const DASHBOARD_KIT_SOURCE = String.raw`
const STYLE = [
  '.d{color:#1d2330;font:14px/1.45 system-ui,-apple-system,"Segoe UI",sans-serif}',
  '.d *{box-sizing:border-box}',
  '.d-head{display:flex;flex-wrap:wrap;align-items:baseline;justify-content:space-between;gap:4px 16px;margin:0 0 16px}',
  '.d-title{margin:0;font-size:20px;font-weight:600}',
  '.d-sub{margin:2px 0 0;color:#5b6474;font-size:13px}',
  '.d-time{color:#8a93a3;font-size:12px}',
  '.d-kpis{display:grid;grid-template-columns:repeat(auto-fit,minmax(170px,1fr));gap:12px;margin:0 0 16px}',
  '.d-kpi{background:#fff;border:1px solid #e3e6ea;border-left:4px solid var(--c,#1565c0);border-radius:10px;padding:12px 16px}',
  '.d-kpi-label{color:#5b6474;font-size:12px;font-weight:600;text-transform:uppercase;letter-spacing:.03em}',
  '.d-kpi-value{font-size:28px;font-weight:600;line-height:1.2;margin-top:4px;font-variant-numeric:tabular-nums}',
  '.d-kpi-note{color:#8a93a3;font-size:12px}',
  '.d-grid{display:grid;grid-template-columns:repeat(12,minmax(0,1fr));gap:16px}',
  '.d-card{grid-column:span var(--span,6);background:#fff;border:1px solid #e3e6ea;border-radius:10px;padding:16px;box-shadow:0 1px 2px rgba(16,24,40,.04);min-width:0}',
  '.d-card-title{margin:0;font-size:15px;font-weight:600}',
  '.d-card-sub{margin:2px 0 12px;color:#8a93a3;font-size:12px}',
  '.d-bar{display:grid;grid-template-columns:minmax(72px,130px) 1fr auto;align-items:center;gap:10px;margin:8px 0}',
  '.d-bar-label{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}',
  '.d-bar-track{height:10px;background:#f0f2f5;border-radius:5px;overflow:hidden}',
  '.d-bar-fill{height:100%;border-radius:5px}',
  '.d-bar-value{font-variant-numeric:tabular-nums;font-weight:600;min-width:28px;text-align:right}',
  '.d-scroll{overflow-x:auto}',
  '.d-table{width:100%;border-collapse:collapse}',
  '.d-table th{text-align:left;color:#5b6474;font-size:12px;font-weight:600;padding:6px 8px;border-bottom:1px solid #e3e6ea;white-space:nowrap}',
  '.d-table td{padding:8px;border-bottom:1px solid #f0f2f5;vertical-align:middle}',
  '.d-table tr:last-child td{border-bottom:0}',
  '.d-table tbody tr:hover{background:#f8f9fb}',
  '.d-nowrap{white-space:nowrap}',
  '.d-trunc{width:100%;max-width:0;min-width:120px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}',
  '.d-chip{display:inline-block;padding:2px 8px;border-radius:999px;font-size:12px;font-weight:500;white-space:nowrap;background:#eef1f5;color:#3b4453}',
  '.d-empty{color:#8a93a3;text-align:center;padding:24px 8px}',
  '@media (max-width:760px){.d-card{grid-column:span 12}}',
].join("\n");
const COLOURS = {
  catastrophic: ["#7f1d1d", "#fde8e8"], critical: ["#c62828", "#fdecea"], high: ["#e65100", "#fff1e6"],
  medium: ["#a16207", "#fef6dc"], low: ["#2e7d32", "#e9f5ea"],
  open: ["#1565c0", "#e8f1fc"], work_in_progress: ["#6a1b9a", "#f4ebf8"], waiting_on_wso2: ["#e65100", "#fff1e6"],
  awaiting_info: ["#00796b", "#e3f4f1"], solution_proposed: ["#00796b", "#e3f4f1"], reopened: ["#c62828", "#fdecea"],
  closed: ["#546e7a", "#eef1f5"],
  moderate: ["#a16207", "#fef6dc"], planning: ["#546e7a", "#eef1f5"],
  new: ["#1565c0", "#e8f1fc"], in_progress: ["#6a1b9a", "#f4ebf8"], on_hold: ["#e65100", "#fff1e6"],
  resolved: ["#2e7d32", "#e9f5ea"], cancelled: ["#546e7a", "#eef1f5"],
  assess: ["#1565c0", "#e8f1fc"], authorize: ["#6a1b9a", "#f4ebf8"], customer_approval: ["#e65100", "#fff1e6"],
  scheduled: ["#00796b", "#e3f4f1"], implement: ["#6a1b9a", "#f4ebf8"], review: ["#1565c0", "#e8f1fc"],
  customer_review: ["#e65100", "#fff1e6"], rollback: ["#c62828", "#fdecea"], canceled: ["#546e7a", "#eef1f5"],
};
const SERIES = ["#1565c0", "#00897b", "#6a1b9a", "#e65100", "#5d4037", "#546e7a"];

function el(tag, cls, text) {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  if (text !== undefined && text !== null) n.textContent = String(text);
  return n;
}
function human(v) {
  if (!v) return "-";
  let s = String(v);
  if (/^[A-Z0-9_]+$/.test(s) || s.includes("_")) s = s.toLowerCase(); // a code value, not a name
  s = s.replace(/_/g, " ").replace(/\bwso2\b/gi, "WSO2");
  return s.charAt(0).toUpperCase() + s.slice(1);
}
function ago(iso) {
  if (!iso) return "-";
  const days = Math.floor((Date.now() - new Date(iso).getTime()) / 86400000);
  return days <= 0 ? "Today" : days === 1 ? "Yesterday" : days < 30 ? days + " days ago" : new Date(iso).toLocaleDateString();
}
function chip(value) {
  const c = el("span", "d-chip", human(value));
  const pair = COLOURS[String(value).toLowerCase()];
  if (pair) { c.style.color = pair[0]; c.style.background = pair[1]; }
  return c;
}
function header(root, title, subtitle) {
  const style = document.createElement("style");
  style.textContent = STYLE;
  root.appendChild(style);
  const page = el("div", "d");
  const head = el("div", "d-head");
  const left = el("div");
  left.appendChild(el("h2", "d-title", title));
  if (subtitle) left.appendChild(el("p", "d-sub", subtitle));
  head.appendChild(left);
  head.appendChild(el("span", "d-time", "Updated " + new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })));
  page.appendChild(head);
  root.appendChild(page);
  return page;
}
function kpis(page, items) { // items: [{ label, value, note?, colour? }]
  const row = el("div", "d-kpis");
  for (const k of items) {
    const box = el("div", "d-kpi");
    if (k.colour) box.style.setProperty("--c", k.colour);
    box.appendChild(el("div", "d-kpi-label", k.label));
    box.appendChild(el("div", "d-kpi-value", k.value));
    if (k.note) box.appendChild(el("div", "d-kpi-note", k.note));
    row.appendChild(box);
  }
  page.appendChild(row);
}
function grid(page) { const g = el("div", "d-grid"); page.appendChild(g); return g; }
function card(g, title, subtitle, span) {
  const c = el("section", "d-card");
  c.style.setProperty("--span", span || 6);
  c.appendChild(el("h3", "d-card-title", title));
  c.appendChild(el("p", "d-card-sub", subtitle || ""));
  g.appendChild(c);
  return c;
}
function bars(c, rows, empty) { // rows: [{ label, count, colour? }]
  if (!rows.some((r) => r.count > 0)) { c.appendChild(el("div", "d-empty", empty || "Nothing to show.")); return; }
  const max = Math.max(...rows.map((r) => r.count));
  rows.forEach((r, i) => {
    const row = el("div", "d-bar");
    row.appendChild(el("span", "d-bar-label", r.label));
    const track = el("div", "d-bar-track");
    const fill = el("div", "d-bar-fill");
    fill.style.width = (r.count / max) * 100 + "%";
    fill.style.background = r.colour || SERIES[i % SERIES.length];
    track.appendChild(fill);
    row.appendChild(track);
    row.appendChild(el("span", "d-bar-value", r.count));
    c.appendChild(row);
  });
}
function table(c, columns, items, empty) { // columns: [{ title, value: (item) => string | Node, cls? }]
  if (items.length === 0) { c.appendChild(el("div", "d-empty", empty || "Nothing to show.")); return; }
  const t = el("table", "d-table");
  const head = el("tr");
  for (const col of columns) head.appendChild(el("th", null, col.title));
  t.appendChild(el("thead")).appendChild(head);
  const body = el("tbody");
  for (const item of items) {
    const tr = el("tr");
    for (const col of columns) {
      const td = el("td", col.cls);
      const v = col.value(item);
      if (v instanceof Node) td.appendChild(v); else td.textContent = v === null || v === undefined ? "-" : String(v);
      tr.appendChild(td);
    }
    body.appendChild(tr);
  }
  t.appendChild(body);
  c.appendChild(el("div", "d-scroll")).appendChild(t);
}
function trend(c, points, empty) { // points: caseVolumeTrend's points
  if (!points.some((p) => p.opened > 0 || p.closed > 0)) { c.appendChild(el("div", "d-empty", empty || "Nothing to show.")); return; }
  const W = Math.max(280, c.clientWidth - 32), H = 200, L = 32, B = 22, max = Math.max(1, ...points.map((p) => Math.max(p.opened, p.closed)));
  const x = (i) => L + (points.length === 1 ? 0 : (i * (W - L - 8)) / (points.length - 1));
  const y = (v) => 8 + (H - B - 8) * (1 - v / max);
  const ns = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(ns, "svg");
  svg.setAttribute("viewBox", "0 0 " + W + " " + H);
  svg.style.cssText = "width:100%;height:auto;display:block";
  const add = (tag, attrs, text) => {
    const n = document.createElementNS(ns, tag);
    for (const k in attrs) n.setAttribute(k, attrs[k]);
    if (text !== undefined) n.textContent = String(text);
    svg.appendChild(n);
  };
  for (const v of [0, max]) {
    add("line", { x1: L, x2: W - 8, y1: y(v), y2: y(v), stroke: "#e3e6ea" });
    add("text", { x: L - 6, y: y(v) + 4, "text-anchor": "end", "font-size": 11, fill: "#8a93a3" }, v);
  }
  for (const i of [0, points.length - 1]) {
    add("text", { x: x(i), y: H - 6, "text-anchor": i === 0 ? "start" : "end", "font-size": 11, fill: "#8a93a3" }, points[i].date);
  }
  for (const [key, colour] of [["opened", "#1565c0"], ["closed", "#2e7d32"]]) {
    add("polyline", { points: points.map((p, i) => x(i) + "," + y(p[key])).join(" "), fill: "none", stroke: colour, "stroke-width": 2 });
  }
  c.appendChild(svg);
  const legend = el("div", "d-card-sub");
  legend.style.margin = "8px 0 0";
  legend.textContent = "Blue: opened (" + points.reduce((s, p) => s + p.opened, 0) + ") · Green: closed (" + points.reduce((s, p) => s + p.closed, 0) + ")";
  c.appendChild(legend);
}
`;

/** The global names DASHBOARD_KIT_SOURCE defines, in order. */
export const DASHBOARD_KIT_NAMES = [
  "STYLE",
  "COLOURS",
  "SERIES",
  "el",
  "human",
  "ago",
  "chip",
  "header",
  "kpis",
  "grid",
  "card",
  "bars",
  "table",
  "trend",
] as const;
