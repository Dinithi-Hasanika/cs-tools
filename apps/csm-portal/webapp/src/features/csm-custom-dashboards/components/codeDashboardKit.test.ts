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

import { describe, expect, it } from "vitest";
import {
  DASHBOARD_KIT_NAMES,
  DASHBOARD_KIT_SOURCE,
} from "@features/csm-custom-dashboards/components/codeDashboardKit";

describe("the code dashboard drawing kit", () => {
  it("offers the names the code catalogue documents", () => {
    // Same list as the backend's catalogue_test.go: change both together.
    expect([...DASHBOARD_KIT_NAMES]).toEqual([
      "STYLE", "COLOURS", "SERIES",
      "el", "human", "ago", "chip", "header", "kpis", "grid", "card", "bars", "table", "trend",
    ]);
  });

  it("defines every listed name at the top level", () => {
    for (const name of DASHBOARD_KIT_NAMES) {
      expect(DASHBOARD_KIT_SOURCE).toMatch(new RegExp(`^(function ${name}\\(|const ${name} = )`, "m"));
    }
  });

  it("is safe to place inside the runner's script element", () => {
    expect(DASHBOARD_KIT_SOURCE).not.toMatch(/<\/script/i);
    expect(DASHBOARD_KIT_SOURCE).not.toContain("`");
  });
});
