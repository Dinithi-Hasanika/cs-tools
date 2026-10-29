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

import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import CodeDashboardFrame from "./CodeDashboardFrame";

vi.mock("@features/csm-custom-dashboards/api/useCodeDashboardPortal", () => ({
  useCodeDashboardPortal: () => ({}),
}));

describe("CodeDashboardFrame", () => {
  it("runs the code in a scripts-only sandbox whose document blocks outbound requests", () => {
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
});
