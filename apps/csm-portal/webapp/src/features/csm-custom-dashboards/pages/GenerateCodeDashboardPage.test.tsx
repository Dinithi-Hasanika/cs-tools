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
import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const generateMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    payload?: { message?: string; errorCode?: string };
    constructor(status: number, message: string, payload?: { message?: string; errorCode?: string }) {
      super(message);
      this.status = status;
      this.payload = payload;
    }
  },
}));
vi.mock("@features/csm-custom-dashboards/api/useGenerateCodeDashboard", () => ({
  useGenerateCodeDashboard: () => generateMock,
}));
// The frame runs the code against the portal; it is tested on its own.
vi.mock("@features/csm-custom-dashboards/components/CodeDashboardFrame", () => ({
  default: ({ source }: { source: string }) => <div>frame running {source.length} characters</div>,
}));

import { BackendApiError } from "@api/backend/client";
import GenerateCodeDashboardPage from "@features/csm-custom-dashboards/pages/GenerateCodeDashboardPage";

const SOURCE = "async function render(portal, root) { root.textContent = 'hi'; }\n";

function renderPage() {
  render(
    <MemoryRouter initialEntries={["/dashboard/my/new-code"]}>
      <Routes>
        <Route path="/dashboard/my/new-code" element={<GenerateCodeDashboardPage />} />
        <Route path="/dashboard/my" element={<div>My Dashboards</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

async function send(text: string, reply: string) {
  fireEvent.change(screen.getByRole("textbox", { name: "Message" }), { target: { value: text } });
  fireEvent.click(screen.getByRole("button", { name: "Send" }));
  await screen.findByText(reply);
}

describe("GenerateCodeDashboardPage", () => {
  beforeEach(() => {
    generateMock.mockReset();
  });

  it("runs the generated code in the preview, shows it under Code, and sends it back with a follow-up", async () => {
    generateMock.mockResolvedValue({ source: SOURCE, reply: "Here it is." });
    renderPage();
    expect(screen.getByText("The generated dashboard will appear here")).toBeInTheDocument();

    await send("My cases", "Here it is.");
    expect(generateMock).toHaveBeenCalledWith("My cases", null);
    expect(screen.getByText(`frame running ${SOURCE.length} characters`)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "Code" }));
    expect(screen.getByLabelText("Generated code")).toHaveTextContent("root.textContent");

    generateMock.mockResolvedValue({ source: SOURCE, reply: "Changed." });
    await send("Add a title", "Changed.");
    expect(generateMock).toHaveBeenLastCalledWith("Add a title", SOURCE);
  });

  it("says why a message failed and keeps the code it had", async () => {
    generateMock.mockResolvedValueOnce({ source: SOURCE, reply: "Here it is." });
    renderPage();
    await send("My cases", "Here it is.");

    generateMock.mockRejectedValueOnce(
      new BackendApiError(422, "declined", { message: "Revenue isn't part of the case data.", errorCode: "generate_declined" }),
    );
    fireEvent.change(screen.getByRole("textbox", { name: "Message" }), { target: { value: "Revenue by month" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(await screen.findByText(/Revenue isn't part of the case data\./)).toBeInTheDocument();
    expect(screen.getByText(/This dashboard cannot be modified with your input\./)).toBeInTheDocument();
    expect(screen.getByText(`frame running ${SOURCE.length} characters`)).toBeInTheDocument();
  });

  it("asks before leaving with work on the page, and leaves straight away without", async () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Back to My Dashboards" }));
    expect(await screen.findByText("My Dashboards")).toBeInTheDocument();
  });

  it("asks before discarding a generated dashboard", async () => {
    generateMock.mockResolvedValue({ source: SOURCE, reply: "Here it is." });
    renderPage();
    await send("My cases", "Here it is.");
    fireEvent.click(screen.getByRole("button", { name: "Discard" }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.queryByText("My Dashboards")).not.toBeInTheDocument();
  });
});
