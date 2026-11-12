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

import { describe, expect, it, vi } from "vitest";

vi.mock("@api/backend/client", () => ({ useBackendApi: () => ({ post: vi.fn() }) }));
vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));

import {
  buildCountCasesRequest,
  buildSearchCasesRequest,
  toCountCasesResult,
  toSearchCasesResult,
} from "@features/csm-custom-dashboards/api/useCodeDashboardPortal";

const myOpenCases = [
  { field: "assignedUserId", op: "in", values: ["00000000-0000-0000-0000-000000000001"] },
  { field: "state", op: "notIn", values: ["closed"] },
];

describe("buildCountCasesRequest", () => {
  it("wraps the filters and passes groupBy through", () => {
    expect(buildCountCasesRequest({ filters: myOpenCases, groupBy: "severity" })).toEqual({
      body: { filters: { filters: myOpenCases }, groupBy: "severity" },
      grouped: true,
    });
  });

  it("groups a plain count by state and marks it ungrouped", () => {
    expect(buildCountCasesRequest({ filters: myOpenCases })).toEqual({
      body: { filters: { filters: myOpenCases }, groupBy: "state" },
      grouped: false,
    });
  });

  it("accepts no args at all", () => {
    expect(buildCountCasesRequest(undefined).body).toEqual({ filters: { filters: [] }, groupBy: "state" });
  });

  it("passes a positive maxGroups through", () => {
    expect(buildCountCasesRequest({ groupBy: "account", maxGroups: 5 }).body.maxGroups).toBe(5);
  });

  it.each([
    [{ groupBy: "assignedUserId" }, /groupBy must be one of/],
    [{ filters: { field: "state" } }, /filters must be an array/],
    [{ groupBy: "state", maxGroups: 0 }, /maxGroups must be a positive integer/],
  ])("refuses %j", (args, message) => {
    expect(() => buildCountCasesRequest(args)).toThrow(message);
  });
});

describe("toCountCasesResult", () => {
  const response = {
    groups: [
      { key: "high", label: "high", count: 3 },
      { key: "low", label: "low", count: 1 },
    ],
    othersCount: 0,
    totalRecords: 4,
  };

  it("keeps label and count per group and the total", () => {
    expect(toCountCasesResult(response, true)).toEqual({
      total: 4,
      groups: [
        { label: "high", count: 3 },
        { label: "low", count: 1 },
      ],
    });
  });

  it("adds an Others group when buckets were folded", () => {
    expect(toCountCasesResult({ ...response, othersCount: 2, totalRecords: 6 }, true).groups).toContainEqual({
      label: "Others",
      count: 2,
    });
  });

  it("returns only the total for an ungrouped count", () => {
    expect(toCountCasesResult(response, false)).toEqual({ total: 4 });
  });

  it("reads an empty response as zero", () => {
    expect(toCountCasesResult({}, true)).toEqual({ total: 0, groups: [] });
  });
});

describe("buildSearchCasesRequest", () => {
  const escalated = [{ field: "escalation", op: "isNotEmpty" }];

  it("wraps the filters, passes sortBy through and reads the first page", () => {
    expect(
      buildSearchCasesRequest({ filters: escalated, sortBy: { field: "createdOn", order: "asc" }, limit: 5 }),
    ).toEqual({
      filters: { filters: escalated },
      sortBy: { field: "createdOn", order: "asc" },
      pagination: { offset: 0, limit: 5 },
    });
  });

  it("defaults to 10 cases, no sort and no filters", () => {
    expect(buildSearchCasesRequest(undefined)).toEqual({
      filters: { filters: [] },
      pagination: { offset: 0, limit: 10 },
    });
  });

  it("sorts descending when no order is given", () => {
    expect(buildSearchCasesRequest({ sortBy: { field: "updatedOn" } }).sortBy).toEqual({
      field: "updatedOn",
      order: "desc",
    });
  });

  it.each([
    [{ filters: { field: "state" } }, /filters must be an array/],
    [{ limit: 0 }, /limit must be an integer from 1 to 50/],
    [{ limit: 51 }, /limit must be an integer from 1 to 50/],
    [{ limit: 2.5 }, /limit must be an integer from 1 to 50/],
    [{ sortBy: { field: "subject" } }, /sortBy.field must be one of/],
    [{ sortBy: "createdOn" }, /sortBy.field must be one of/],
    [{ sortBy: { field: "createdOn", order: "up" } }, /sortBy.order must be "asc" or "desc"/],
  ])("refuses %j", (args, message) => {
    expect(() => buildSearchCasesRequest(args)).toThrow(message);
  });
});

describe("toSearchCasesResult", () => {
  it("flattens each case to plain values, related records by name", () => {
    const result = toSearchCasesResult({
      total: 6,
      cases: [
        {
          id: "00000000-0000-0000-0000-000000000013",
          number: "CS0000013",
          subject: "Gateway rejecting requests",
          description: "<p>Long body that must not be passed on</p>",
          type: "case",
          severity: "catastrophic",
          state: "work_in_progress",
          escalationLevel: "1",
          createdOn: "2026-10-20T12:25:20Z",
          account: { id: "a1", name: "Acme Corp" },
          project: { id: "p1", name: "Acme Gateway" },
          assignedEngineer: { id: "u1", email: "jane.doe@example.com", name: "Jane Doe" },
        },
      ],
    });
    expect(result).toEqual({
      total: 6,
      items: [
        {
          id: "00000000-0000-0000-0000-000000000013",
          number: "CS0000013",
          subject: "Gateway rejecting requests",
          type: "case",
          severity: "catastrophic",
          state: "work_in_progress",
          escalationLevel: "1",
          createdOn: "2026-10-20T12:25:20Z",
          updatedOn: null,
          account: "Acme Corp",
          project: "Acme Gateway",
          assignee: "Jane Doe",
        },
      ],
    });
  });

  it("reads missing related records and an empty response as null / empty", () => {
    expect(toSearchCasesResult({ cases: [{ id: "c1", assignedEngineer: null }] }).items[0]).toMatchObject({
      number: null,
      account: null,
      project: null,
      assignee: null,
    });
    expect(toSearchCasesResult({})).toEqual({ total: 0, items: [] });
  });
});
