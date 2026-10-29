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
import type { BeCaseSearchResponse, BeCaseSearchView, BeCaseSortField } from "@api/backend/types";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";

/** A fixed portal API a code dashboard may call: takes the call's args, returns its result. */
export type CodeDashboardPortalApi = (args: unknown) => unknown;

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

/**
 * Experimental (POC): the fixed `portal.*` APIs CodeDashboardFrame answers.
 * `currentUser` is the signed-in user; `countCases` calls `POST /cases/aggregate`
 * and `searchCases` `POST /cases/search`, both with the viewer's own login, so
 * the backend's normal permissions apply. Any name not here is refused by the frame.
 */
export function useCodeDashboardPortal(): Record<string, CodeDashboardPortalApi> {
  const api = useBackendApi();
  const { user } = useCurrentUser();

  return useMemo(
    () => ({
      currentUser: () => {
        if (!user?.id) throw new Error("currentUser: the signed-in user is not loaded");
        return { id: user.id, name: [user.firstName, user.lastName].filter(Boolean).join(" "), email: user.email };
      },
      countCases: async (args: unknown) => {
        const { body, grouped } = buildCountCasesRequest(args);
        const res = await api.post<CaseAggregateRequest, CaseAggregateResponse>("/cases/aggregate", body);
        return toCountCasesResult(res, grouped);
      },
      searchCases: async (args: unknown) => {
        const body = buildSearchCasesRequest(args);
        const res = await api.post<CaseSearchRequest, BeCaseSearchResponse>("/cases/search", body);
        return toSearchCasesResult(res);
      },
    }),
    [api, user],
  );
}
