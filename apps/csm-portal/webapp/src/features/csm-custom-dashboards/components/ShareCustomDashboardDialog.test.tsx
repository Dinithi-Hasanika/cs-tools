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
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

interface User {
  id: string;
  userName: string;
  name: string;
  email: string;
  userType?: string;
  active?: boolean;
}
let searchResults: { data?: { users: User[] }; isFetching?: boolean; isError?: boolean };
let sharesState: { data?: unknown; isLoading?: boolean; isError?: boolean };
const shareMock = vi.fn();
const unshareMock = vi.fn();

vi.mock("@features/csm-users/api/useSearchUsers", () => ({
  useSearchUsers: () => searchResults,
}));
vi.mock("@features/csm-custom-dashboards/api/useDashboardShares", () => ({
  useGetCustomDashboardShares: () => sharesState,
  usePutCustomDashboardShare: () => ({ mutateAsync: shareMock, isPending: false }),
  useDeleteCustomDashboardShare: () => ({ mutateAsync: unshareMock, isPending: false, variables: undefined }),
}));

import { BackendApiError } from "@api/backend/client";
import ShareCustomDashboardDialog from "@features/csm-custom-dashboards/components/ShareCustomDashboardDialog";

const DASHBOARD = { id: "5e636a12-0000-4000-8000-000000000001", name: "Live Save Test" };
const JANE: User = { id: "jane", userName: "jane", name: "Jane Doe", email: "jane.doe@example.com", userType: "internal" };
const ALICE: User = { id: "alice", userName: "alice", name: "Alice Perera", email: "alice.perera@example.com", userType: "internal" };
const BEN: User = { id: "ben", userName: "ben", name: "Ben Bee", email: "ben@example.com", userType: "internal" };
const CUSTOMER: User = { id: "cust", userName: "cust", name: "John Smith", email: "john@example.com", userType: "customer" };

function renderDialog(onClose = vi.fn()) {
  render(<ShareCustomDashboardDialog open dashboard={DASHBOARD} currentUserEmail="Jane.Doe@example.com" onClose={onClose} />);
  return onClose;
}

function resultNames(): string[] {
  return within(screen.getByLabelText("People to share with"))
    .queryAllByRole("button")
    .map((b) => b.getAttribute("aria-label") ?? "");
}

describe("ShareCustomDashboardDialog", () => {
  beforeEach(() => {
    searchResults = { data: { users: [JANE, ALICE, BEN, CUSTOMER] } };
    sharesState = { data: [] };
    shareMock.mockReset();
    unshareMock.mockReset();
  });

  it("names the dashboard and says nobody has it yet", () => {
    renderDialog();
    expect(screen.getByText("Share “Live Save Test”")).toBeInTheDocument();
    expect(screen.getByText("Not shared with anyone yet.")).toBeInTheDocument();
    expect(screen.getByText(/can't edit it/)).toBeInTheDocument();
  });

  it("offers internal staff only, without the caller or people who already have it", () => {
    sharesState = { data: [{ user: { id: "ben", email: "ben@example.com", name: "Ben Bee" }, sharedOn: "a" }] };
    renderDialog();
    expect(resultNames()).toEqual(["Share with Alice Perera"]);
  });

  it("shares with the person clicked", async () => {
    shareMock.mockResolvedValue([]);
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Share with Alice Perera" }));
    await waitFor(() => expect(shareMock).toHaveBeenCalledWith({ dashboardId: DASHBOARD.id, userId: "alice" }));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("shows the backend's refusal as a sentence", async () => {
    shareMock.mockRejectedValue(new BackendApiError(400, "dashboards can only be shared with internal staff"));
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Share with Alice Perera" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Dashboards can only be shared with internal staff.");
  });

  it("is generic when sharing fails for another reason", async () => {
    shareMock.mockRejectedValue(new Error("network down"));
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Share with Alice Perera" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("The dashboard couldn't be shared. Try again in a moment.");
  });

  it("lists who has it, and stops sharing with one of them", async () => {
    unshareMock.mockResolvedValue(null);
    sharesState = { data: [{ user: { id: "alice", email: "alice.perera@example.com", name: "Alice Perera" }, sharedOn: "a" }] };
    renderDialog();
    const shared = screen.getByLabelText("Shared with");
    expect(within(shared).getByText("Alice Perera")).toBeInTheDocument();
    expect(within(shared).getByText("alice.perera@example.com")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Stop sharing with Alice Perera" }));
    await waitFor(() => expect(unshareMock).toHaveBeenCalledWith({ dashboardId: DASHBOARD.id, userId: "alice" }));
  });

  it("says so when the share list can't be loaded", () => {
    sharesState = { isError: true };
    renderDialog();
    expect(screen.getByText("Could not load who this dashboard is shared with.")).toBeInTheDocument();
  });

  it("says so when the search fails or finds nobody", () => {
    searchResults = { isError: true };
    renderDialog();
    expect(screen.getByText("Could not search people. Try again.")).toBeInTheDocument();
  });

  it("closes with Done", () => {
    const onClose = renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Done" }));
    expect(onClose).toHaveBeenCalled();
  });
});
