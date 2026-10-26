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

import { useMemo } from "react";
import { useBackendApi } from "@api/backend/client";
import type {
  BeCaseSearchResponse,
  BeCaseSearchView,
  BeCaseSortField,
  BeCaseVolumeTrendResponse,
} from "@api/backend/types";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";

/** A fixed portal API a code dashboard may call: takes the call's args, returns its result. */
export type CodeDashboardPortalApi = (args: unknown) => unknown;

/**
 * The only backend endpoints a code dashboard can reach, all read-only: each
 * reads (a search, a count, a trend) and changes nothing, even where it is a
 * POST to carry its filters. `readOnlyPost` refuses any other path, so a
 * portal API that writes cannot be added by accident; adding an endpoint
 * here is a deliberate decision that it only reads.
 */
export const READ_ONLY_ENDPOINTS = [
  "/cases/aggregate",
  "/cases/search",
  "/cases/volume-trend",
  "/incidents/aggregate",
  "/incidents/search",
  "/change-requests/aggregate",
  "/change-requests/search",
] as const;
type ReadOnlyEndpoint = (typeof READ_ONLY_ENDPOINTS)[number];

/** The client call the portal APIs use (the backend client's `post`). */
type PostFn = <Req, Res>(path: string, body: Req) => Promise<Res>;

/** Calls a READ_ONLY_ENDPOINTS endpoint, or throws for any other path. */
export function readOnlyPost<Req, Res>(post: PostFn, path: ReadOnlyEndpoint, body: Req): Promise<Res> {
  if (!(READ_ONLY_ENDPOINTS as readonly string[]).includes(path)) {
    return Promise.reject(new Error(`${path} is not a read-only endpoint a dashboard may call`));
  }
  return post<Req, Res>(path, body);
}

/** The fields `POST /cases/aggregate` can group by (backend `CaseAggregatePayload.groupBy`). */
const CASE_GROUP_BY_FIELDS = ["account", "state", "severity", "type"] as const;
type CaseGroupByField = (typeof CASE_GROUP_BY_FIELDS)[number];

/** Body of POST /cases/aggregate. */
interface CaseAggregateRequest {
  filters: { filters: unknown[] };
  groupBy: CaseGroupByField;
  maxGroups?: number;
}

/** Response of POST /cases/aggregate (backend `AggregateResponse`). */
interface CaseAggregateResponse {
  groups?: { key: string; label: string; count: number }[];
  othersCount?: number;
  totalRecords?: number;
}

/** What `portal.countCases` returns to the code: `groups` only when it asked for a `groupBy`. */
export interface CountCasesResult {
  total: number;
  groups?: { label: string; count: number }[];
}

/**
 * Turns `portal.countCases(args)` into a `POST /cases/aggregate` body, or
 * throws (the code sees the message) when the args are not `{filters?, groupBy?, maxGroups?}`.
 * The endpoint always needs a `groupBy`; a plain count groups by `state`, which every case has,
 * and keeps only the total.
 */
export function buildCountCasesRequest(args: unknown): { body: CaseAggregateRequest; grouped: boolean } {
  const { filters, groupBy, maxGroups } = (args ?? {}) as { filters?: unknown; groupBy?: unknown; maxGroups?: unknown };
  if (filters !== undefined && !Array.isArray(filters)) {
    throw new Error("countCases: filters must be an array of {field, op, values}");
  }
  if (groupBy !== undefined && !CASE_GROUP_BY_FIELDS.includes(groupBy as CaseGroupByField)) {
    throw new Error(`countCases: groupBy must be one of ${CASE_GROUP_BY_FIELDS.join(", ")}`);
  }
  if (maxGroups !== undefined && !(Number.isInteger(maxGroups) && (maxGroups as number) > 0)) {
    throw new Error("countCases: maxGroups must be a positive integer");
  }
  return {
    body: {
      filters: { filters: filters ?? [] },
      groupBy: (groupBy as CaseGroupByField | undefined) ?? "state",
      ...(maxGroups !== undefined ? { maxGroups: maxGroups as number } : {}),
    },
    grouped: groupBy !== undefined,
  };
}

/** Reshapes a `POST /cases/aggregate` response into what `portal.countCases` returns. */
export function toCountCasesResult(res: CaseAggregateResponse, grouped: boolean): CountCasesResult {
  const total = res.totalRecords ?? 0;
  if (!grouped) return { total };
  const groups = (res.groups ?? []).map(({ label, count }) => ({ label, count }));
  if (res.othersCount) groups.push({ label: "Others", count: res.othersCount });
  return { total, groups };
}

/** The fields `POST /cases/search` can sort by (`BeCaseSortField`). */
const CASE_SORT_FIELDS: readonly BeCaseSortField[] = ["createdOn", "updatedOn", "severity", "state", "assignee"];

/** How many cases `portal.searchCases` returns when the code does not say. */
const SEARCH_CASES_DEFAULT_LIMIT = 10;

/** Body of POST /cases/search, as `portal.searchCases` sends it: always the first page. */
interface CaseSearchRequest {
  filters: { filters: unknown[] };
  sortBy?: { field: BeCaseSortField; order: "asc" | "desc" };
  pagination: { offset: 0; limit: number };
}

/** One case as `portal.searchCases` returns it: plain values, related records by name. */
export interface CodeDashboardCase {
  id: string;
  number: string | null;
  subject: string | null;
  type: string | null;
  severity: string | null;
  state: string | null;
  escalationLevel: string | null;
  createdOn: string | null;
  updatedOn: string | null;
  account: string | null;
  project: string | null;
  assignee: string | null;
}

/** What `portal.searchCases` returns to the code. */
export interface SearchCasesResult {
  total: number;
  items: CodeDashboardCase[];
}

/**
 * Turns `portal.searchCases(args)` into a `POST /cases/search` body, or throws
 * (the code sees the message) when the args are not
 * `{filters?, sortBy?: {field, order?}, limit?}`. `limit` is 1 to the backend's
 * page cap (50) and defaults to 10; only the first page is read.
 */
export function buildSearchCasesRequest(args: unknown): CaseSearchRequest {
  const { filters, sortBy, limit } = (args ?? {}) as { filters?: unknown; sortBy?: unknown; limit?: unknown };
  if (filters !== undefined && !Array.isArray(filters)) {
    throw new Error("searchCases: filters must be an array of {field, op, values}");
  }
  if (limit !== undefined && !(Number.isInteger(limit) && (limit as number) >= 1 && (limit as number) <= BE_MAX_PAGE_LIMIT)) {
    throw new Error(`searchCases: limit must be an integer from 1 to ${BE_MAX_PAGE_LIMIT}`);
  }
  let sort: CaseSearchRequest["sortBy"];
  if (sortBy !== undefined) {
    const { field, order } = (sortBy ?? {}) as { field?: unknown; order?: unknown };
    if (!CASE_SORT_FIELDS.includes(field as BeCaseSortField)) {
      throw new Error(`searchCases: sortBy.field must be one of ${CASE_SORT_FIELDS.join(", ")}`);
    }
    if (order !== undefined && order !== "asc" && order !== "desc") {
      throw new Error('searchCases: sortBy.order must be "asc" or "desc"');
    }
    sort = { field: field as BeCaseSortField, order: (order as "asc" | "desc" | undefined) ?? "desc" };
  }
  return {
    filters: { filters: filters ?? [] },
    ...(sort ? { sortBy: sort } : {}),
    pagination: { offset: 0, limit: (limit as number | undefined) ?? SEARCH_CASES_DEFAULT_LIMIT },
  };
}

/**
 * Reshapes a `POST /cases/search` response into what `portal.searchCases`
 * returns. Only the listed fields are passed on (no description or other
 * free-text bodies), so the code gets what a list or table needs and nothing more.
 */
export function toSearchCasesResult(res: Partial<BeCaseSearchResponse>): SearchCasesResult {
  const items = (res.cases ?? []).map(
    (c: BeCaseSearchView): CodeDashboardCase => ({
      id: c.id,
      number: c.number ?? null,
      subject: c.subject ?? null,
      type: c.type ?? null,
      severity: c.severity ?? null,
      state: c.state ?? null,
      escalationLevel: c.escalationLevel ?? null,
      createdOn: c.createdOn ?? null,
      updatedOn: c.updatedOn ?? null,
      account: c.account?.name ?? null,
      project: c.project?.name ?? null,
      assignee: c.assignedEngineer?.name ?? null,
    }),
  );
  return { total: res.total ?? items.length, items };
}

/** The most days `POST /cases/volume-trend` accepts, and its default. */
const VOLUME_TREND_MAX_DAYS = 90;
const VOLUME_TREND_DEFAULT_DAYS = 30;

/** Body of POST /cases/volume-trend. */
interface CaseVolumeTrendRequest {
  filters: { filters: unknown[] };
  days: number;
}

/** What `portal.caseVolumeTrend` returns: one point per day, oldest first. */
export interface CaseVolumeTrendResult {
  days: number;
  points: { date: string; opened: number; closed: number }[];
  totals: { opened: number; closed: number };
}

/**
 * Turns `portal.caseVolumeTrend(args)` into a `POST /cases/volume-trend`
 * body, or throws when the args are not `{filters?, days?}`. `days` is 1 to
 * 90 (the endpoint's cap) and defaults to 30.
 */
export function buildCaseVolumeTrendRequest(args: unknown): CaseVolumeTrendRequest {
  const { filters, days } = (args ?? {}) as { filters?: unknown; days?: unknown };
  if (filters !== undefined && !Array.isArray(filters)) {
    throw new Error("caseVolumeTrend: filters must be an array of {field, op, values}");
  }
  if (days !== undefined && !(Number.isInteger(days) && (days as number) >= 1 && (days as number) <= VOLUME_TREND_MAX_DAYS)) {
    throw new Error(`caseVolumeTrend: days must be an integer from 1 to ${VOLUME_TREND_MAX_DAYS}`);
  }
  return { filters: { filters: filters ?? [] }, days: (days as number | undefined) ?? VOLUME_TREND_DEFAULT_DAYS };
}

/**
 * Reshapes a `POST /cases/volume-trend` response (two series, "Opened" and
 * "Closed", each a list of `{date, count}`) into one point per date with both
 * counts, oldest first, plus the totals. A date missing from a series counts 0.
 */
export function toCaseVolumeTrendResult(res: Partial<BeCaseVolumeTrendResponse>): CaseVolumeTrendResult {
  const byDate = new Map<string, { opened: number; closed: number }>();
  for (const series of res.series ?? []) {
    const key = series.label.toLowerCase() === "closed" ? "closed" : series.label.toLowerCase() === "opened" ? "opened" : null;
    if (!key) continue;
    for (const { date, count } of series.points ?? []) {
      const point = byDate.get(date) ?? { opened: 0, closed: 0 };
      point[key] = count;
      byDate.set(date, point);
    }
  }
  const points = [...byDate.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([date, counts]) => ({ date, ...counts }));
  const totals = points.reduce(
    (sum, p) => ({ opened: sum.opened + p.opened, closed: sum.closed + p.closed }),
    { opened: 0, closed: 0 },
  );
  return { days: res.days ?? points.length, points, totals };
}

/** Incident priorities, the values of the search's top-level `priorities` list. */
const INCIDENT_PRIORITIES = ["CRITICAL", "HIGH", "MODERATE", "LOW", "PLANNING"] as const;

/** Fields `POST /incidents/search` sorts by. */
const INCIDENT_SORT_FIELDS = ["createdOn", "updatedOn", "openedOn"] as const;

/**
 * Filter fields the incident endpoints accept but do not apply (the entity
 * service validates `productName` and then ignores it), so a dashboard would
 * silently count every incident. Refused here instead.
 */
const INCIDENT_IGNORED_FILTER_FIELDS = ["productName"];

/** The `filters` object both incident endpoints take. */
interface IncidentFilters {
  filters: unknown[];
  priorities?: string[];
}

/** Checks the `filters` and `priorities` args shared by the incident APIs. */
function buildIncidentFilters(fn: string, filters: unknown, priorities: unknown): IncidentFilters {
  if (filters !== undefined && !Array.isArray(filters)) {
    throw new Error(`${fn}: filters must be an array of {field, op, values}`);
  }
  for (const f of (filters as { field?: unknown }[] | undefined) ?? []) {
    if (INCIDENT_IGNORED_FILTER_FIELDS.includes(String(f?.field))) {
      throw new Error(`${fn}: filtering by ${String(f.field)} is not supported for incidents`);
    }
  }
  if (
    priorities !== undefined &&
    !(Array.isArray(priorities) && priorities.length > 0 && priorities.every((p) => INCIDENT_PRIORITIES.includes(p)))
  ) {
    throw new Error(`${fn}: priorities must be a non-empty array of ${INCIDENT_PRIORITIES.join(", ")}`);
  }
  return { filters: filters ?? [], ...(priorities !== undefined ? { priorities: priorities as string[] } : {}) };
}

/** Body of POST /incidents/aggregate. */
interface IncidentAggregateRequest {
  filters: IncidentFilters;
  groupBy: "state";
}

/**
 * Turns `portal.countIncidents(args)` into a `POST /incidents/aggregate`
 * body, or throws when the args are not `{filters?, priorities?, groupBy?}`.
 * The only grouping offered is `state`: on this data source `assignmentGroup`
 * and `priority` are refused, and `businessService` groups come back as raw
 * ids. A plain count groups by `state` and keeps only the total.
 */
export function buildCountIncidentsRequest(args: unknown): { body: IncidentAggregateRequest; grouped: boolean } {
  const { filters, priorities, groupBy } = (args ?? {}) as { filters?: unknown; priorities?: unknown; groupBy?: unknown };
  if (groupBy !== undefined && groupBy !== "state") {
    throw new Error('countIncidents: groupBy must be "state"');
  }
  return {
    body: { filters: buildIncidentFilters("countIncidents", filters, priorities), groupBy: "state" },
    grouped: groupBy !== undefined,
  };
}

/** Body of POST /incidents/search, as `portal.searchIncidents` sends it: always the first page. */
interface IncidentSearchRequest {
  filters: IncidentFilters;
  sortBy?: { field: (typeof INCIDENT_SORT_FIELDS)[number]; order: "asc" | "desc" };
  pagination: { offset: 0; limit: number };
}

/**
 * Turns `portal.searchIncidents(args)` into a `POST /incidents/search` body,
 * or throws when the args are not `{filters?, priorities?, sortBy?, limit?}`.
 * `limit` is 1 to 50 and defaults to 10; only the first page is read.
 */
export function buildSearchIncidentsRequest(args: unknown): IncidentSearchRequest {
  const { filters, priorities, sortBy, limit } = (args ?? {}) as {
    filters?: unknown;
    priorities?: unknown;
    sortBy?: unknown;
    limit?: unknown;
  };
  if (limit !== undefined && !(Number.isInteger(limit) && (limit as number) >= 1 && (limit as number) <= BE_MAX_PAGE_LIMIT)) {
    throw new Error(`searchIncidents: limit must be an integer from 1 to ${BE_MAX_PAGE_LIMIT}`);
  }
  let sort: IncidentSearchRequest["sortBy"];
  if (sortBy !== undefined) {
    const { field, order } = (sortBy ?? {}) as { field?: unknown; order?: unknown };
    if (!(INCIDENT_SORT_FIELDS as readonly unknown[]).includes(field)) {
      throw new Error(`searchIncidents: sortBy.field must be one of ${INCIDENT_SORT_FIELDS.join(", ")}`);
    }
    if (order !== undefined && order !== "asc" && order !== "desc") {
      throw new Error('searchIncidents: sortBy.order must be "asc" or "desc"');
    }
    sort = { field: field as (typeof INCIDENT_SORT_FIELDS)[number], order: (order as "asc" | "desc" | undefined) ?? "desc" };
  }
  return {
    filters: buildIncidentFilters("searchIncidents", filters, priorities),
    ...(sort ? { sortBy: sort } : {}),
    pagination: { offset: 0, limit: (limit as number | undefined) ?? SEARCH_CASES_DEFAULT_LIMIT },
  };
}

/** One incident as `POST /incidents/search` returns it (the fields read here). */
interface IncidentSearchView {
  id: string;
  number?: string | null;
  subject?: string | null;
  priority?: string | null;
  state?: string | null;
  category?: string | null;
  openedOn?: string | null;
  createdOn?: string | null;
  updatedOn?: string | null;
  assignmentGroup?: { name?: string } | null;
  assignedTo?: { name?: string } | null;
}

/** Response of POST /incidents/search. */
interface IncidentSearchResponse {
  incidents?: IncidentSearchView[];
  total?: number;
}

/** One incident as `portal.searchIncidents` returns it: plain values, people and groups by name. */
export interface CodeDashboardIncident {
  id: string;
  number: string | null;
  subject: string | null;
  priority: string | null;
  state: string | null;
  category: string | null;
  openedOn: string | null;
  createdOn: string | null;
  updatedOn: string | null;
  assignmentGroup: string | null;
  assignee: string | null;
}

/**
 * Reshapes a `POST /incidents/search` response into what
 * `portal.searchIncidents` returns. The caller (often a customer contact),
 * createdBy / updatedBy (emails) and parent references are not passed on.
 */
export function toSearchIncidentsResult(res: IncidentSearchResponse): { total: number; items: CodeDashboardIncident[] } {
  const items = (res.incidents ?? []).map(
    (i): CodeDashboardIncident => ({
      id: i.id,
      number: i.number ?? null,
      subject: i.subject ?? null,
      priority: i.priority ?? null,
      state: i.state ?? null,
      category: i.category ?? null,
      openedOn: i.openedOn ?? null,
      createdOn: i.createdOn ?? null,
      updatedOn: i.updatedOn ?? null,
      assignmentGroup: i.assignmentGroup?.name ?? null,
      assignee: i.assignedTo?.name ?? null,
    }),
  );
  return { total: res.total ?? items.length, items };
}

/** Change request states (lowercase, as the endpoints take and return them). */
const CHANGE_REQUEST_STATES = [
  "new", "assess", "authorize", "customer_approval", "scheduled", "implement",
  "review", "customer_review", "rollback", "closed", "canceled",
] as const;

/** Change request impacts. */
const CHANGE_REQUEST_IMPACTS = ["high", "medium", "low"] as const;

/** Fields `POST /change-requests/aggregate` can group by on this data source. */
const CHANGE_REQUEST_GROUP_BY_FIELDS = ["state", "impact", "category", "risk"] as const;

/** Fields `POST /change-requests/search` sorts by. */
const CHANGE_REQUEST_SORT_FIELDS = ["createdOn", "updatedOn"] as const;

/**
 * The only `filters` fields offered for change requests. The entity service
 * also accepts `assignedUserId` and (for counts) `assignmentGroupId` but never
 * applies them, so a dashboard would silently show every change request;
 * only fields that are applied are let through.
 */
const CHANGE_REQUEST_FILTER_FIELDS = ["createdOn", "approval"];

/** The `filters` object both change request endpoints take. */
interface ChangeRequestFilters {
  states?: string[];
  impacts?: string[];
  closedStartDate?: string;
  closedEndDate?: string;
  filters: unknown[];
}

/** An ISO timestamp for `value`, or throws: the endpoint needs a full RFC 3339 time. */
function toTimestamp(fn: string, name: string, value: unknown): string {
  const t = typeof value === "string" ? new Date(value) : new Date(NaN);
  if (Number.isNaN(t.getTime())) throw new Error(`${fn}: ${name} must be a date, e.g. "2026-01-01"`);
  return t.toISOString();
}

/** Checks the filter args shared by the change request APIs. */
function buildChangeRequestFilters(fn: string, args: Record<string, unknown>): ChangeRequestFilters {
  const { states, impacts, closedFrom, closedTo, filters } = args;
  const listOf = (name: string, value: unknown, allowed: readonly string[]): string[] | undefined => {
    if (value === undefined) return undefined;
    if (!(Array.isArray(value) && value.length > 0 && value.every((v) => allowed.includes(v)))) {
      throw new Error(`${fn}: ${name} must be a non-empty array of ${allowed.join(", ")}`);
    }
    return value as string[];
  };
  if (filters !== undefined && !Array.isArray(filters)) {
    throw new Error(`${fn}: filters must be an array of {field, op, values}`);
  }
  for (const f of (filters as { field?: unknown }[] | undefined) ?? []) {
    if (!CHANGE_REQUEST_FILTER_FIELDS.includes(String(f?.field))) {
      throw new Error(`${fn}: filters may only use ${CHANGE_REQUEST_FILTER_FIELDS.join(", ")}, not ${String(f?.field)}`);
    }
  }
  const out: ChangeRequestFilters = { filters: filters ?? [] };
  const s = listOf("states", states, CHANGE_REQUEST_STATES);
  const i = listOf("impacts", impacts, CHANGE_REQUEST_IMPACTS);
  if (s) out.states = s;
  if (i) out.impacts = i;
  if (closedFrom !== undefined) out.closedStartDate = toTimestamp(fn, "closedFrom", closedFrom);
  if (closedTo !== undefined) out.closedEndDate = toTimestamp(fn, "closedTo", closedTo);
  return out;
}

/** Body of POST /change-requests/aggregate. */
interface ChangeRequestAggregateRequest {
  filters: ChangeRequestFilters;
  groupBy: (typeof CHANGE_REQUEST_GROUP_BY_FIELDS)[number];
  maxGroups?: number;
}

/**
 * Turns `portal.countChangeRequests(args)` into a `POST /change-requests/aggregate`
 * body, or throws. `groupBy` is `state`, `impact`, `category` or `risk`
 * (`assignmentGroup` and `type` are refused on this data source). A plain
 * count groups by `state` and keeps only the total.
 */
export function buildCountChangeRequestsRequest(args: unknown): { body: ChangeRequestAggregateRequest; grouped: boolean } {
  const a = (args ?? {}) as Record<string, unknown>;
  if (a.groupBy !== undefined && !(CHANGE_REQUEST_GROUP_BY_FIELDS as readonly unknown[]).includes(a.groupBy)) {
    throw new Error(`countChangeRequests: groupBy must be one of ${CHANGE_REQUEST_GROUP_BY_FIELDS.join(", ")}`);
  }
  if (a.maxGroups !== undefined && !(Number.isInteger(a.maxGroups) && (a.maxGroups as number) > 0)) {
    throw new Error("countChangeRequests: maxGroups must be a positive integer");
  }
  return {
    body: {
      filters: buildChangeRequestFilters("countChangeRequests", a),
      groupBy: (a.groupBy as ChangeRequestAggregateRequest["groupBy"] | undefined) ?? "state",
      ...(a.maxGroups !== undefined ? { maxGroups: a.maxGroups as number } : {}),
    },
    grouped: a.groupBy !== undefined,
  };
}

/** Body of POST /change-requests/search, as `portal.searchChangeRequests` sends it: always the first page. */
interface ChangeRequestSearchRequest {
  filters: ChangeRequestFilters;
  sortBy?: { field: (typeof CHANGE_REQUEST_SORT_FIELDS)[number]; order: "asc" | "desc" };
  pagination: { offset: 0; limit: number };
}

/**
 * Turns `portal.searchChangeRequests(args)` into a `POST /change-requests/search`
 * body, or throws. `limit` is 1 to 50 and defaults to 10; only the first page is read.
 */
export function buildSearchChangeRequestsRequest(args: unknown): ChangeRequestSearchRequest {
  const a = (args ?? {}) as Record<string, unknown>;
  const { sortBy, limit } = a;
  if (limit !== undefined && !(Number.isInteger(limit) && (limit as number) >= 1 && (limit as number) <= BE_MAX_PAGE_LIMIT)) {
    throw new Error(`searchChangeRequests: limit must be an integer from 1 to ${BE_MAX_PAGE_LIMIT}`);
  }
  let sort: ChangeRequestSearchRequest["sortBy"];
  if (sortBy !== undefined) {
    const { field, order } = (sortBy ?? {}) as { field?: unknown; order?: unknown };
    if (!(CHANGE_REQUEST_SORT_FIELDS as readonly unknown[]).includes(field)) {
      throw new Error(`searchChangeRequests: sortBy.field must be one of ${CHANGE_REQUEST_SORT_FIELDS.join(", ")}`);
    }
    if (order !== undefined && order !== "asc" && order !== "desc") {
      throw new Error('searchChangeRequests: sortBy.order must be "asc" or "desc"');
    }
    sort = { field: field as (typeof CHANGE_REQUEST_SORT_FIELDS)[number], order: (order as "asc" | "desc" | undefined) ?? "desc" };
  }
  return {
    filters: buildChangeRequestFilters("searchChangeRequests", a),
    ...(sort ? { sortBy: sort } : {}),
    pagination: { offset: 0, limit: (limit as number | undefined) ?? SEARCH_CASES_DEFAULT_LIMIT },
  };
}

/** One change request as `POST /change-requests/search` returns it (the fields read here). */
interface ChangeRequestSearchView {
  id: string;
  number?: string | null;
  subject?: string | null;
  type?: string | null;
  state?: string | null;
  impact?: string | null;
  onHold?: boolean | null;
  plannedStartOn?: string | null;
  plannedEndOn?: string | null;
  createdOn?: string | null;
  updatedOn?: string | null;
  project?: { name?: string } | null;
  deployment?: { name?: string } | null;
  deployedProduct?: { name?: string } | null;
  product?: { name?: string } | null;
  assignedEngineer?: { name?: string } | null;
  assignedTeam?: { name?: string } | null;
}

/** Response of POST /change-requests/search. */
interface ChangeRequestSearchResponse {
  changeRequests?: ChangeRequestSearchView[];
  total?: number;
}

/** One change request as `portal.searchChangeRequests` returns it: plain values, related records by name. */
export interface CodeDashboardChangeRequest {
  id: string;
  number: string | null;
  subject: string | null;
  type: string | null;
  state: string | null;
  impact: string | null;
  onHold: boolean;
  plannedStartOn: string | null;
  plannedEndOn: string | null;
  createdOn: string | null;
  updatedOn: string | null;
  project: string | null;
  deployment: string | null;
  product: string | null;
  assignee: string | null;
  team: string | null;
}

/**
 * Reshapes a `POST /change-requests/search` response into what
 * `portal.searchChangeRequests` returns. The description, the on-hold reason
 * (free text) and the linked case are not passed on.
 */
export function toSearchChangeRequestsResult(res: ChangeRequestSearchResponse): {
  total: number;
  items: CodeDashboardChangeRequest[];
} {
  const items = (res.changeRequests ?? []).map(
    (c): CodeDashboardChangeRequest => ({
      id: c.id,
      number: c.number ?? null,
      subject: c.subject ?? null,
      type: c.type ?? null,
      state: c.state ?? null,
      impact: c.impact ?? null,
      onHold: c.onHold === true,
      plannedStartOn: c.plannedStartOn ?? null,
      plannedEndOn: c.plannedEndOn ?? null,
      createdOn: c.createdOn ?? null,
      updatedOn: c.updatedOn ?? null,
      project: c.project?.name ?? null,
      deployment: c.deployment?.name ?? null,
      product: c.deployedProduct?.name ?? c.product?.name ?? null,
      assignee: c.assignedEngineer?.name ?? null,
      team: c.assignedTeam?.name ?? null,
    }),
  );
  return { total: res.total ?? items.length, items };
}

/**
 * Experimental (POC): the fixed `portal.*` APIs CodeDashboardFrame answers.
 * `currentUser` is the signed-in user; `countCases` calls `POST /cases/aggregate`,
 * `searchCases` `POST /cases/search`, `caseVolumeTrend` `POST /cases/volume-trend`,
 * `countIncidents` `POST /incidents/aggregate`, `searchIncidents`
 * `POST /incidents/search`, `countChangeRequests` `POST /change-requests/aggregate`
 * and `searchChangeRequests` `POST /change-requests/search`, all through
 * `readOnlyPost` with the viewer's own login, so only read-only endpoints are
 * reachable and the backend's normal permissions apply (the incident and
 * change request APIs need Operations access: a 403 otherwise). Any name not
 * here is refused by the frame.
 */
export function useCodeDashboardPortal(): Record<string, CodeDashboardPortalApi> {
  const api = useBackendApi();
  const { user } = useCurrentUser();

  return useMemo(() => {
    const post: PostFn = (path, body) => api.post(path, body);
    return {
      currentUser: () => {
        if (!user?.id) throw new Error("currentUser: the signed-in user is not loaded");
        return { id: user.id, name: [user.firstName, user.lastName].filter(Boolean).join(" "), email: user.email };
      },
      countCases: async (args: unknown) => {
        const { body, grouped } = buildCountCasesRequest(args);
        const res = await readOnlyPost<CaseAggregateRequest, CaseAggregateResponse>(post, "/cases/aggregate", body);
        return toCountCasesResult(res, grouped);
      },
      searchCases: async (args: unknown) => {
        const body = buildSearchCasesRequest(args);
        const res = await readOnlyPost<CaseSearchRequest, BeCaseSearchResponse>(post, "/cases/search", body);
        return toSearchCasesResult(res);
      },
      caseVolumeTrend: async (args: unknown) => {
        const body = buildCaseVolumeTrendRequest(args);
        const res = await readOnlyPost<CaseVolumeTrendRequest, BeCaseVolumeTrendResponse>(post, "/cases/volume-trend", body);
        return toCaseVolumeTrendResult(res);
      },
      countIncidents: async (args: unknown) => {
        const { body, grouped } = buildCountIncidentsRequest(args);
        const res = await readOnlyPost<IncidentAggregateRequest, CaseAggregateResponse>(post, "/incidents/aggregate", body);
        return toCountCasesResult(res, grouped);
      },
      searchIncidents: async (args: unknown) => {
        const body = buildSearchIncidentsRequest(args);
        const res = await readOnlyPost<IncidentSearchRequest, IncidentSearchResponse>(post, "/incidents/search", body);
        return toSearchIncidentsResult(res);
      },
      countChangeRequests: async (args: unknown) => {
        const { body, grouped } = buildCountChangeRequestsRequest(args);
        const res = await readOnlyPost<ChangeRequestAggregateRequest, CaseAggregateResponse>(post, "/change-requests/aggregate", body);
        return toCountCasesResult(res, grouped);
      },
      searchChangeRequests: async (args: unknown) => {
        const body = buildSearchChangeRequestsRequest(args);
        const res = await readOnlyPost<ChangeRequestSearchRequest, ChangeRequestSearchResponse>(post, "/change-requests/search", body);
        return toSearchChangeRequestsResult(res);
      },
    };
  }, [api, user]);
}
