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

import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import GenerateCodeDashboardRoute from "./GenerateCodeDashboardRoute";

vi.mock("@features/csm-custom-dashboards/pages/GenerateCodeDashboardPage", () => ({
  default: () => <div>code generate page</div>,
}));

function renderAt(path: string): void {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/dashboard/my/new-code" element={<GenerateCodeDashboardRoute />} />
        <Route path="/dashboard/my/:dashboardId" element={<div>my dashboards page</div>} />
        <Route path="/404" element={<div>not found page</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("GenerateCodeDashboardRoute", () => {
  const original = window.config;
  afterEach(() => {
    window.config = original;
  });

  it("sends to the 404 page while the code approach is off (the default)", () => {
    window.config = { ...original, DASHBOARD_GENERATE_CODE_ENABLED: undefined };
    renderAt("/dashboard/my/new-code");
    expect(screen.getByText("not found page")).toBeInTheDocument();
    expect(screen.queryByText("code generate page")).not.toBeInTheDocument();
    expect(screen.queryByText("my dashboards page")).not.toBeInTheDocument();
  });

  it("opens the code generate page when DASHBOARD_GENERATE_CODE_ENABLED is true", () => {
    window.config = { ...original, DASHBOARD_GENERATE_CODE_ENABLED: true };
    renderAt("/dashboard/my/new-code");
    expect(screen.getByText("code generate page")).toBeInTheDocument();
  });
});
