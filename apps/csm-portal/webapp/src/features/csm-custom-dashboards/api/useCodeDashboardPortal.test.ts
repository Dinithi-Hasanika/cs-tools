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
  READ_ONLY_ENDPOINTS,
  buildCaseVolumeTrendRequest,
  buildCountCasesRequest,
  buildCountChangeRequestsRequest,
  buildCountIncidentsRequest,
  buildSearchCasesRequest,
  buildSearchChangeRequestsRequest,
  buildSearchIncidentsRequest,
  readOnlyPost,
  toCaseVolumeTrendResult,
  toCountCasesResult,
  toSearchCasesResult,
  toSearchChangeRequestsResult,
  toSearchIncidentsResult,
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

describe("readOnlyPost", () => {
  it("allows only these read-only endpoints (adding one is a deliberate change)", () => {
    expect([...READ_ONLY_ENDPOINTS]).toEqual([
      "/cases/aggregate",
      "/cases/search",
      "/cases/volume-trend",
      "/incidents/aggregate",
      "/incidents/search",
      "/change-requests/aggregate",
      "/change-requests/search",
    ]);
  });

  it("calls an allowed endpoint", async () => {
    const post = vi.fn().mockResolvedValue({ ok: true });
    await expect(readOnlyPost(post, "/cases/search", { a: 1 })).resolves.toEqual({ ok: true });
    expect(post).toHaveBeenCalledWith("/cases/search", { a: 1 });
  });

  it("refuses any other path without calling it", async () => {
    const post = vi.fn();
    for (const path of ["/cases", "/cases/123/comments", "/time-cards/search", "/cases/search/../123"]) {
      await expect(readOnlyPost(post, path as never, {})).rejects.toThrow(/is not a read-only endpoint/);
    }
    expect(post).not.toHaveBeenCalled();
  });
});

describe("buildCaseVolumeTrendRequest", () => {
  it("wraps the filters and passes days through", () => {
    expect(buildCaseVolumeTrendRequest({ filters: myOpenCases, days: 14 })).toEqual({
      filters: { filters: myOpenCases },
      days: 14,
    });
  });

  it("defaults to 30 days and no filters", () => {
    expect(buildCaseVolumeTrendRequest(undefined)).toEqual({ filters: { filters: [] }, days: 30 });
  });

  it.each([
    [{ filters: { field: "state" } }, /filters must be an array/],
    [{ days: 0 }, /days must be an integer from 1 to 90/],
    [{ days: 91 }, /days must be an integer from 1 to 90/],
    [{ days: 7.5 }, /days must be an integer from 1 to 90/],
  ])("refuses %j", (args, message) => {
    expect(() => buildCaseVolumeTrendRequest(args)).toThrow(message);
  });
});

describe("toCaseVolumeTrendResult", () => {
  it("zips the two series into one point per day, oldest first, with totals", () => {
    expect(
      toCaseVolumeTrendResult({
        days: 3,
        series: [
          { label: "Opened", points: [{ date: "2026-10-19", count: 2 }, { date: "2026-10-17", count: 1 }, { date: "2026-10-18", count: 0 }] },
          { label: "Closed", points: [{ date: "2026-10-17", count: 0 }, { date: "2026-10-18", count: 3 }, { date: "2026-10-19", count: 1 }] },
        ],
      }),
    ).toEqual({
      days: 3,
      points: [
        { date: "2026-10-17", opened: 1, closed: 0 },
        { date: "2026-10-18", opened: 0, closed: 3 },
        { date: "2026-10-19", opened: 2, closed: 1 },
      ],
      totals: { opened: 3, closed: 4 },
    });
  });

  it("counts a date missing from one series as 0, and reads an empty response as empty", () => {
    expect(
      toCaseVolumeTrendResult({ days: 1, series: [{ label: "Opened", points: [{ date: "2026-10-19", count: 5 }] }] }).points,
    ).toEqual([{ date: "2026-10-19", opened: 5, closed: 0 }]);
    expect(toCaseVolumeTrendResult({})).toEqual({ days: 0, points: [], totals: { opened: 0, closed: 0 } });
  });
});

const openIncidents = [{ field: "state", op: "in", values: ["NEW", "IN_PROGRESS"] }];

describe("buildCountIncidentsRequest", () => {
  it("wraps filters and priorities and groups by state", () => {
    expect(buildCountIncidentsRequest({ filters: openIncidents, priorities: ["CRITICAL"], groupBy: "state" })).toEqual({
      body: { filters: { filters: openIncidents, priorities: ["CRITICAL"] }, groupBy: "state" },
      grouped: true,
    });
  });

  it("groups a plain count by state and marks it ungrouped", () => {
    expect(buildCountIncidentsRequest(undefined)).toEqual({
      body: { filters: { filters: [] }, groupBy: "state" },
      grouped: false,
    });
  });

  it.each([
    [{ groupBy: "priority" }, /groupBy must be "state"/],
    [{ groupBy: "assignmentGroup" }, /groupBy must be "state"/],
    [{ filters: { field: "state" } }, /filters must be an array/],
    [{ priorities: ["critical"] }, /priorities must be a non-empty array of CRITICAL/],
    [{ priorities: [] }, /priorities must be a non-empty array/],
    [{ filters: [{ field: "productName", op: "in", values: ["API Manager"] }] }, /productName is not supported for incidents/],
  ])("refuses %j", (args, message) => {
    expect(() => buildCountIncidentsRequest(args)).toThrow(message);
  });
});

describe("buildSearchIncidentsRequest", () => {
  it("wraps filters and priorities, passes sortBy through and reads the first page", () => {
    expect(
      buildSearchIncidentsRequest({ filters: openIncidents, priorities: ["HIGH"], sortBy: { field: "openedOn" }, limit: 5 }),
    ).toEqual({
      filters: { filters: openIncidents, priorities: ["HIGH"] },
      sortBy: { field: "openedOn", order: "desc" },
      pagination: { offset: 0, limit: 5 },
    });
  });

  it("defaults to 10 incidents, no sort and no filters", () => {
    expect(buildSearchIncidentsRequest(undefined)).toEqual({ filters: { filters: [] }, pagination: { offset: 0, limit: 10 } });
  });

  it.each([
    [{ limit: 51 }, /limit must be an integer from 1 to 50/],
    [{ sortBy: { field: "priority" } }, /sortBy.field must be one of createdOn, updatedOn, openedOn/],
    [{ sortBy: { field: "createdOn", order: "up" } }, /sortBy.order must be "asc" or "desc"/],
    [{ filters: [{ field: "productName", op: "in", values: ["x"] }] }, /productName is not supported/],
  ])("refuses %j", (args, message) => {
    expect(() => buildSearchIncidentsRequest(args)).toThrow(message);
  });
});

describe("toSearchIncidentsResult", () => {
  it("flattens each incident and leaves out the caller and the editors", () => {
    const result = toSearchIncidentsResult({
      total: 3,
      incidents: [
        {
          id: "i1",
          number: "INC-1018",
          subject: "Elevated error rate",
          priority: "CRITICAL",
          state: "RESOLVED",
          category: "SOFTWARE",
          openedOn: "2026-10-20T10:10:59Z",
          createdOn: "2026-10-17T12:10:59Z",
          updatedOn: "2026-10-20T10:10:59Z",
          assignmentGroup: null,
          assignedTo: { name: "Nico Rivera" },
          caller: { id: "c1", name: "Jane Customer" },
          createdBy: "someone@example.com",
        } as never,
      ],
    });
    expect(result).toEqual({
      total: 3,
      items: [
        {
          id: "i1",
          number: "INC-1018",
          subject: "Elevated error rate",
          priority: "CRITICAL",
          state: "RESOLVED",
          category: "SOFTWARE",
          openedOn: "2026-10-20T10:10:59Z",
          createdOn: "2026-10-17T12:10:59Z",
          updatedOn: "2026-10-20T10:10:59Z",
          assignmentGroup: null,
          assignee: "Nico Rivera",
        },
      ],
    });
    expect(JSON.stringify(result)).not.toMatch(/Jane Customer|someone@example.com/);
  });

  it("reads an empty response as empty", () => {
    expect(toSearchIncidentsResult({})).toEqual({ total: 0, items: [] });
  });
});

describe("buildCountChangeRequestsRequest", () => {
  it("passes the named lists, filters and groupBy through", () => {
    const filters = [{ field: "approval", op: "eq", values: ["requested"] }];
    expect(
      buildCountChangeRequestsRequest({ states: ["scheduled", "implement"], impacts: ["high"], filters, groupBy: "impact", maxGroups: 3 }),
    ).toEqual({
      body: { filters: { states: ["scheduled", "implement"], impacts: ["high"], filters }, groupBy: "impact", maxGroups: 3 },
      grouped: true,
    });
  });

  it("turns closedFrom / closedTo into full timestamps", () => {
    const { body } = buildCountChangeRequestsRequest({ closedFrom: "2026-01-01", closedTo: "2026-01-31T23:59:59Z" });
    expect(body.filters.closedStartDate).toBe("2026-01-01T00:00:00.000Z");
    expect(body.filters.closedEndDate).toBe("2026-01-31T23:59:59.000Z");
  });

  it("groups a plain count by state and marks it ungrouped", () => {
    expect(buildCountChangeRequestsRequest(undefined)).toEqual({ body: { filters: { filters: [] }, groupBy: "state" }, grouped: false });
  });

  it.each([
    [{ groupBy: "assignmentGroup" }, /groupBy must be one of state, impact, category, risk/],
    [{ groupBy: "type" }, /groupBy must be one of/],
    [{ states: ["SCHEDULED"] }, /states must be a non-empty array of new, assess/],
    [{ impacts: ["critical"] }, /impacts must be a non-empty array of high, medium, low/],
    [{ closedFrom: "last month" }, /closedFrom must be a date/],
    [{ filters: [{ field: "assignedUserId", op: "in", values: ["u1"] }] }, /filters may only use createdOn, approval, not assignedUserId/],
    [{ filters: [{ field: "assignmentGroupId", op: "in", values: ["g1"] }] }, /not assignmentGroupId/],
    [{ maxGroups: 0, groupBy: "state" }, /maxGroups must be a positive integer/],
  ])("refuses %j", (args, message) => {
    expect(() => buildCountChangeRequestsRequest(args)).toThrow(message);
  });
});

describe("buildSearchChangeRequestsRequest", () => {
  it("passes sortBy and limit through and reads the first page", () => {
    expect(buildSearchChangeRequestsRequest({ states: ["scheduled"], sortBy: { field: "updatedOn" }, limit: 5 })).toEqual({
      filters: { states: ["scheduled"], filters: [] },
      sortBy: { field: "updatedOn", order: "desc" },
      pagination: { offset: 0, limit: 5 },
    });
  });

  it.each([
    [{ limit: 51 }, /limit must be an integer from 1 to 50/],
    [{ sortBy: { field: "plannedStartOn" } }, /sortBy.field must be one of createdOn, updatedOn/],
    [{ filters: [{ field: "assignedUserId", op: "in", values: ["u1"] }] }, /filters may only use createdOn, approval/],
  ])("refuses %j", (args, message) => {
    expect(() => buildSearchChangeRequestsRequest(args)).toThrow(message);
  });
});

describe("toSearchChangeRequestsResult", () => {
  it("flattens each change request and leaves out the free text and the case", () => {
    const result = toSearchChangeRequestsResult({
      total: 11,
      changeRequests: [
        {
          id: "c1",
          number: "CR-1015",
          subject: "Migrate deployment to new region",
          description: "Long description text",
          onHoldReason: "Waiting for the customer",
          case: { id: "k1", number: "CS0001" },
          state: "scheduled",
          impact: "medium",
          type: null,
          onHold: null,
          plannedStartOn: "2026-08-24T20:10:59Z",
          plannedEndOn: "2026-08-24T22:10:59Z",
          createdOn: "2026-08-21T12:10:59Z",
          updatedOn: "2026-08-22T20:10:59Z",
          project: { name: "Acme Gateway" },
          deployment: { name: "Production" },
          deployedProduct: { name: "API Manager 4.3.0" },
          assignedEngineer: { name: "Jane Doe" },
          assignedTeam: null,
        } as never,
      ],
    });
    expect(result).toEqual({
      total: 11,
      items: [
        {
          id: "c1",
          number: "CR-1015",
          subject: "Migrate deployment to new region",
          type: null,
          state: "scheduled",
          impact: "medium",
          onHold: false,
          plannedStartOn: "2026-08-24T20:10:59Z",
          plannedEndOn: "2026-08-24T22:10:59Z",
          createdOn: "2026-08-21T12:10:59Z",
          updatedOn: "2026-08-22T20:10:59Z",
          project: "Acme Gateway",
          deployment: "Production",
          product: "API Manager 4.3.0",
          assignee: "Jane Doe",
          team: null,
        },
      ],
    });
    expect(JSON.stringify(result)).not.toMatch(/Long description|Waiting for the customer|CS0001/);
  });

  it("reads an empty response as empty", () => {
    expect(toSearchChangeRequestsResult({})).toEqual({ total: 0, items: [] });
  });
});
