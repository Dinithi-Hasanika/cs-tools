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

import { act, render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import CodeDashboardFrame from "./CodeDashboardFrame";

// The portal the frame answers from: one allowed call, and a property that is
// on every object but is not a portal API.
const countCases = vi.fn(async () => ({ total: 3 }));
vi.mock("@features/csm-custom-dashboards/api/useCodeDashboardPortal", () => ({
  useCodeDashboardPortal: () => ({ countCases }),
}));

/** Renders the frame and returns its window, with postMessage spied on. */
function renderFrame() {
  const { container } = render(<CodeDashboardFrame source="function render(portal, root) {}" />);
  const win = container.querySelector("iframe")!.contentWindow!;
  const post = vi.spyOn(win, "postMessage").mockImplementation(() => {});
  return { win, post };
}

/** Sends a message to the page as if from source. */
async function sendFrom(source: MessageEventSource, data: unknown): Promise<void> {
  await act(async () => {
    window.dispatchEvent(new MessageEvent("message", { data, source }));
    await new Promise((r) => setTimeout(r, 0));
  });
}

describe("CodeDashboardFrame", () => {
  it("runs the code in a scripts-only sandbox whose document blocks fetch and subresource requests", () => {
    const { container } = render(<CodeDashboardFrame source="function render(portal, root) {}" />);
    const frame = container.querySelector("iframe");
    expect(frame).not.toBeNull();
    expect(frame?.getAttribute("sandbox")).toBe("allow-scripts");

    const doc = new DOMParser().parseFromString(frame?.getAttribute("srcdoc") ?? "", "text/html");
    const csp = doc.querySelector('meta[http-equiv="Content-Security-Policy"]')?.getAttribute("content") ?? "";
    const directives = new Map(
      csp.split(";").map((d) => {
        const [name, ...values] = d.trim().split(/\s+/);
        return [name, values.join(" ")] as const;
      }),
    );
    // Nothing may leave the frame: no fetch / XHR / WebSocket (connect-src),
    // no images from a URL, no frames, fonts or forms; all fall back to
    // default-src 'none'.
    expect(directives.get("default-src")).toBe("'none'");
    expect(directives.get("img-src")).toBe("data:");
    expect(directives.get("form-action")).toBe("'none'");
    for (const name of ["connect-src", "frame-src", "font-src", "media-src", "object-src"]) {
      expect(directives.has(name)).toBe(false);
    }
    // The runner itself still has to run inline.
    expect(directives.get("script-src")).toContain("'unsafe-inline'");
  });

  it("answers an allowed portal call from its own frame with the result", async () => {
    const { win, post } = renderFrame();
    await sendFrom(win, { type: "call", id: 1, fn: "countCases", args: { groupBy: "state" } });
    expect(countCases).toHaveBeenCalledWith({ groupBy: "state" });
    expect(post).toHaveBeenCalledWith({ type: "result", id: 1, value: { total: 3 } }, "*");
  });

  it("refuses a name that is not a portal API, even one every object has", async () => {
    const { win, post } = renderFrame();
    await sendFrom(win, { type: "call", id: 2, fn: "toString" });
    await sendFrom(win, { type: "call", id: 3, fn: "deleteCase" });
    expect(post).toHaveBeenCalledWith({ type: "result", id: 2, error: "portal.toString is not available" }, "*");
    expect(post).toHaveBeenCalledWith({ type: "result", id: 3, error: "portal.deleteCase is not available" }, "*");
  });

  it("ignores a call from any window other than its own frame", async () => {
    countCases.mockClear();
    const { post } = renderFrame();
    const other = document.createElement("iframe");
    document.body.appendChild(other);
    await sendFrom(other.contentWindow!, { type: "call", id: 4, fn: "countCases" });
    await sendFrom(window, { type: "call", id: 5, fn: "countCases" });
    expect(countCases).not.toHaveBeenCalled();
    expect(post).not.toHaveBeenCalled();
    other.remove();
  });
});
