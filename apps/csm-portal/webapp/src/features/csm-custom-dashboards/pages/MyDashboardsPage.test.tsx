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

import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

interface ListState {
  data?: { id: string; name: string; createdOn: string; updatedOn: string }[];
  isLoading?: boolean;
  isFetching?: boolean;
  isError?: boolean;
}
interface DetailState {
  data?: unknown;
  isLoading?: boolean;
  isError?: boolean;
  error?: Error;
}
let listState: ListState;
let detailState: DetailState;
const detailIds: (string | undefined)[] = [];
const deleteMock = vi.fn();
let deletePending = false;
const showSuccess = vi.fn();
const showError = vi.fn();

vi.mock("@features/csm-custom-dashboards/api/useCustomDashboards", () => ({
  useGetCustomDashboards: () => listState,
  useGetCustomDashboard: (id: string | undefined) => {
    detailIds.push(id);
    return detailState;
  },
  useDeleteCustomDashboard: () => ({ mutateAsync: deleteMock, isPending: deletePending }),
}));
vi.mock("@context/success-banner/SuccessBannerContext", () => ({ useSuccessBanner: () => ({ showSuccess }) }));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({ useErrorBanner: () => ({ showError }) }));
// The widgets fetch their own data; the grid is not under test here.
vi.mock("@features/csm-dashboard/components/DashboardWidgetGrid", () => ({
  default: ({ widgets }: { widgets: { widgetId: string }[] }) => (
    <div>Grid: {widgets.map((w) => w.widgetId).join(", ")}</div>
  ),
}));

import MyDashboardsPage from "@features/csm-custom-dashboards/pages/MyDashboardsPage";

const A = { id: "5e636a12-0000-4000-8000-00000000000a", name: "Alpha", createdOn: "x", updatedOn: "y" };
const B = { id: "5e636a12-0000-4000-8000-00000000000b", name: "Beta", createdOn: "x", updatedOn: "y" };

function savedWith(widgetId: string) {
  return { dashboard: { id: "any", displayName: "any", isDefault: false, isTeamBased: false, widgets: [{ widgetId }] }, createdOn: "x", updatedOn: "y" };
}

function renderAt(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/dashboard/my" element={<MyDashboardsPage />} />
        <Route path="/dashboard/my/:dashboardId" element={<MyDashboardsPage />} />
        <Route path="/dashboard/my/new" element={<div>Generate page</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("MyDashboardsPage", () => {
  beforeEach(() => {
    listState = { data: [A, B] };
    detailState = { data: savedWith("w1") };
    detailIds.length = 0;
    deleteMock.mockReset();
    deletePending = false;
    showSuccess.mockReset();
    showError.mockReset();
  });

  it("shows the empty state with the Generate Dashboard button when nothing is saved", () => {
    listState = { data: [] };
    renderAt("/dashboard/my");
    expect(screen.getByText("You have no dashboards yet")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Generate Dashboard" })).toBeInTheDocument();
  });

  it("opens the first saved dashboard when the URL names none", () => {
    renderAt("/dashboard/my");
    expect(screen.getByText("Alpha", { selector: "p" })).toBeInTheDocument();
    expect(detailIds).toContain(A.id);
  });

  it("draws the dashboard the URL names, with its widgets", () => {
    detailState = { data: savedWith("open-cases") };
    renderAt(`/dashboard/my/${B.id}`);
    expect(screen.getByText("Beta", { selector: "p" })).toBeInTheDocument();
    expect(screen.getByText("Grid: open-cases")).toBeInTheDocument();
    expect(detailIds).toContain(B.id);
  });

  it("waits for the list to be re-read before giving up on an unknown id", () => {
    listState = { data: [A], isFetching: true };
    renderAt(`/dashboard/my/${B.id}`);
    expect(detailIds).not.toContain(A.id);
    expect(screen.queryByText("Alpha", { selector: "p" })).not.toBeInTheDocument();
  });

  it("opens the first dashboard for an id that is not in the list", () => {
    listState = { data: [A] };
    renderAt(`/dashboard/my/${B.id}`);
    expect(screen.getByText("Alpha", { selector: "p" })).toBeInTheDocument();
  });

  it("shows the backend's message when a dashboard can't be shown", () => {
    detailState = { isError: true, error: new Error("This saved dashboard can't be shown any more. Generate it again.") };
    renderAt(`/dashboard/my/${A.id}`);
    expect(screen.getByText("This saved dashboard can't be shown any more. Generate it again.")).toBeInTheDocument();
  });

  it("says so when the dashboard no longer exists", () => {
    detailState = { data: null };
    renderAt(`/dashboard/my/${A.id}`);
    expect(screen.getByText("This dashboard no longer exists.")).toBeInTheDocument();
  });

  it("says so when the list can't be loaded", () => {
    listState = { isError: true };
    renderAt("/dashboard/my");
    expect(screen.getByText("Could not load the dashboard list.")).toBeInTheDocument();
  });

  describe("Delete, from the switcher's entries", () => {
    // Opens the switcher and clicks the delete icon of the named entry.
    function deleteFromMenu(name: string) {
      fireEvent.mouseDown(screen.getByRole("combobox", { name: "Select dashboard" }));
      fireEvent.click(screen.getByRole("button", { name: `Delete dashboard ${name}` }));
    }

    it("has a delete icon on each entry, and none in the header", () => {
      renderAt(`/dashboard/my/${A.id}`);
      expect(screen.queryByRole("button", { name: "Delete" })).not.toBeInTheDocument();
      fireEvent.mouseDown(screen.getByRole("combobox", { name: "Select dashboard" }));
      expect(screen.getByRole("button", { name: "Delete dashboard Alpha" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Delete dashboard Beta" })).toBeInTheDocument();
    });

    it("asks first without switching, and Cancel deletes nothing", async () => {
      renderAt(`/dashboard/my/${A.id}`);
      deleteFromMenu("Beta");

      expect(screen.getByText("Delete this dashboard?")).toBeInTheDocument();
      expect(screen.getByText(/“Beta” will be removed from My Dashboards/)).toBeInTheDocument();
      // Still on Alpha: the icon is not a selection. (The open dialog hides the
      // page from the accessibility tree, hence `hidden`.)
      expect(screen.getByRole("combobox", { name: "Select dashboard", hidden: true })).toHaveTextContent("Alpha");

      fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
      await waitFor(() => expect(screen.queryByText("Delete this dashboard?")).not.toBeInTheDocument());
      expect(deleteMock).not.toHaveBeenCalled();
    });

    it("deleting the open dashboard goes back to My Dashboards", async () => {
      deleteMock.mockResolvedValue(null);
      renderAt(`/dashboard/my/${B.id}`);
      deleteFromMenu("Beta");
      fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete" }));

      await waitFor(() => expect(showSuccess).toHaveBeenCalledWith('"Beta" was deleted.'));
      expect(deleteMock).toHaveBeenCalledWith(B.id);
      // Back at /dashboard/my, which opens the first one in the list.
      await waitFor(() => expect(screen.getByText("Alpha", { selector: "p" })).toBeInTheDocument());
    });

    it("deleting another dashboard keeps the open one", async () => {
      deleteMock.mockResolvedValue(null);
      renderAt(`/dashboard/my/${B.id}`);
      deleteFromMenu("Alpha");
      fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete" }));

      await waitFor(() => expect(showSuccess).toHaveBeenCalledWith('"Alpha" was deleted.'));
      expect(deleteMock).toHaveBeenCalledWith(A.id);
      expect(screen.getByText("Beta", { selector: "p" })).toBeInTheDocument();
    });

    it("shows the error banner when the delete fails", async () => {
      const err = new Error("boom");
      deleteMock.mockRejectedValue(err);
      renderAt(`/dashboard/my/${A.id}`);
      deleteFromMenu("Alpha");
      fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete" }));

      await waitFor(() =>
        expect(showError).toHaveBeenCalledWith("The dashboard couldn't be deleted. Try again in a moment.", err),
      );
      expect(showSuccess).not.toHaveBeenCalled();
    });

    it("disables the dialog's buttons while the delete is in flight", () => {
      deletePending = true;
      renderAt(`/dashboard/my/${A.id}`);
      deleteFromMenu("Alpha");
      const dialog = screen.getByRole("dialog");
      expect(within(dialog).getByRole("button", { name: "Deleting…" })).toBeDisabled();
      expect(within(dialog).getByRole("button", { name: "Cancel" })).toBeDisabled();
    });
  });
});
