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

import { Box } from "@wso2/oxygen-ui";
import { type JSX, Suspense, useMemo } from "react";
import { Outlet } from "react-router";
import RouteSuspenseFallback from "@components/route-fallback/RouteSuspenseFallback";
import SectionTabs from "@components/section-tabs/SectionTabs";
import { featureState } from "@config/featureFlags";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import { useRouteTabs } from "@hooks/useSectionTabs";

/**
 * Dashboards shell — the tab strip above each kind of dashboard (System, My,
 * Shared). The tabs come from the navigation tree (the "dashboard" section),
 * like Customers. No title of its own: each tab's page keeps its own heading
 * (System Dashboards is CsmDashboardPage, unchanged).
 *
 * `useRouteTabs` applies only the deployment's feature overrides, not a tab's
 * own `requires`, so this drops the tabs the signed-in user cannot open (My
 * and Shared Dashboards for anyone without `canWrite`) -- the same narrowing
 * CsmAdminLayout does for its Dashboards tab. FeatureRouteGuard still refuses
 * those routes; this only keeps the strip from offering them.
 */
export default function CsmDashboardsLayout(): JSX.Element {
  const access = usePortalAccess();
  const allTabs = useRouteTabs("dashboard");
  const tabs = useMemo(() => {
    const visible = allTabs.tabs.filter((tab) => featureState(tab.node.id, access) !== "hidden");
    // `allTabs.activeKey` was resolved against the unfiltered list; never hand
    // `<Tabs>` a value with no matching `<Tab>`.
    const activeKey = visible.some((tab) => tab.key === allTabs.activeKey)
      ? allTabs.activeKey
      : (visible[0]?.key ?? "");
    return { ...allTabs, tabs: visible, activeKey };
  }, [allTabs, access]);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <SectionTabs {...tabs} ariaLabel="Dashboards tabs" scrollable />

      <Suspense fallback={<RouteSuspenseFallback />}>
        <Outlet />
      </Suspense>
    </Box>
  );
}
