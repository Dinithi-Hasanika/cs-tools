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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useParams } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { BeDashboard } from "@api/backend/types";

const generateMock = vi.fn();
const saveMock = vi.fn();
const showSuccess = vi.fn();
const showError = vi.fn();
let savePending = false;

vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));
vi.mock("@features/csm-custom-dashboards/api/useGenerateDashboard", () => ({
  useGenerateDashboard: () => generateMock,
}));
vi.mock("@features/csm-custom-dashboards/api/useCustomDashboards", () => ({
  usePutCustomDashboard: () => ({ mutateAsync: saveMock, isPending: savePending }),
}));
vi.mock("@context/success-banner/SuccessBannerContext", () => ({ useSuccessBanner: () => ({ showSuccess }) }));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({ useErrorBanner: () => ({ showError }) }));
// The preview's widgets fetch their own data; the grid is not under test here.
vi.mock("@features/csm-dashboard/components/DashboardWidgetGrid", () => ({
  default: ({ widgets }: { widgets: unknown[] }) => <div>{widgets.length} widgets</div>,
}));

import { BackendApiError } from "@api/backend/client";
import GenerateDashboardPage from "@features/csm-custom-dashboards/pages/GenerateDashboardPage";

function dashboard(name: string): BeDashboard {
  return { id: "5e636a12-0000-4000-8000-000000000001", displayName: name, isDefault: false, isTeamBased: false, widgets: [] };
}

function OpenedDashboard() {
  const { dashboardId } = useParams();
  return <div>Opened {dashboardId}</div>;
}

function renderPage() {
  render(
    <MemoryRouter initialEntries={["/dashboard/my/new"]}>
      <Routes>
        <Route path="/dashboard/my/new" element={<GenerateDashboardPage />} />
        <Route path="/dashboard/my/:dashboardId" element={<OpenedDashboard />} />
      </Routes>
    </MemoryRouter>,
  );
}

// Sends a chat message and waits for its reply to be shown.
async function sendMessage(text: string, reply: string) {
  fireEvent.change(screen.getByRole("textbox", { name: "Message" }), { target: { value: text } });
  fireEvent.click(screen.getByRole("button", { name: "Send" }));
  await screen.findByText(reply);
}

describe("GenerateDashboardPage: Save", () => {
  beforeEach(() => {
    generateMock.mockReset();
    saveMock.mockReset();
    showSuccess.mockReset();
    showError.mockReset();
    savePending = false;
  });

  it("is disabled until there is a dashboard to save", () => {
    renderPage();
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  it("saves the draft, says so, and opens the saved dashboard", async () => {
    const first = dashboard("My Cases");
    generateMock.mockResolvedValueOnce({ dashboard: first, reply: "Here it is." });
    saveMock.mockResolvedValue({ dashboard: first, createdOn: "a", updatedOn: "b" });
    renderPage();

    await sendMessage("My cases", "Here it is.");
    fireEvent.click(await screen.findByRole("button", { name: "Save" }));

    expect(saveMock).toHaveBeenCalledWith(first);
    expect(await screen.findByText(`Opened ${first.id}`)).toBeInTheDocument();
    expect(showSuccess).toHaveBeenCalledWith('"My Cases" was saved to My Dashboards.');
  });

  it("explains a name that is already taken, and keeps Save available", async () => {
    generateMock.mockResolvedValueOnce({ dashboard: dashboard("My Cases"), reply: "Here it is." });
    const err = new BackendApiError(409, "you already have a dashboard named: My Cases");
    saveMock.mockRejectedValue(err);
    renderPage();

    await sendMessage("My cases", "Here it is.");
    fireEvent.click(await screen.findByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(showError).toHaveBeenCalledWith(
        'You already have a dashboard named "My Cases". Ask for a different name in the chat, then save again.',
        err,
      ),
    );
    expect(showSuccess).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
  });

  it("shows Saving… while the save is in flight", () => {
    savePending = true;
    renderPage();
    expect(screen.getByRole("button", { name: "Saving…" })).toBeDisabled();
  });
});
