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

import { Box, Button, FormControl, MenuItem, Select, Skeleton, Typography } from "@wso2/oxygen-ui";
import { Code, LayoutDashboard, Plus } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useMemo } from "react";
import { Navigate, useNavigate, useParams } from "react-router";
import { useDashboardList } from "@features/csm-dashboard/api/useDashboardList";
import AgentsLandingPagePilot from "@features/csm-dashboard/components/AgentsLandingPagePilot";

const BASE_PATH = "/custom-dashboards/my";

/**
 * Same props as Create case (CsmCasesPage). With dashboards it sits in the
 * header next to the switcher; with none it is the centred call to action of
 * the empty state. Opens the generate page (`/custom-dashboards/my/new`).
 */
function GenerateDashboardButton(): JSX.Element {
  const navigate = useNavigate();
  return (
    <Button
      variant="contained"
      color="primary"
      size="small"
      startIcon={<Plus size={16} />}
      onClick={() => navigate(`${BASE_PATH}/new`)}
    >
      Generate Dashboard
    </Button>
  );
}

/**
 * Experimental: opens the code-based generate page
 * (`/custom-dashboards/my/new-code`), where the model writes the dashboard as
 * code instead of a JSON definition. Shown beside GenerateDashboardButton so
 * the two approaches can be compared; removed or promoted once one is chosen.
 */
function GenerateCodeDashboardButton(): JSX.Element {
  const navigate = useNavigate();
  return (
    <Button
      variant="outlined"
      size="small"
      startIcon={<Code size={16} />}
      onClick={() => navigate(`${BASE_PATH}/new-code`)}
    >
      Generate AI Code (Experimental)
    </Button>
  );
}

/**
 * Custom Dashboards > My Dashboards (`/custom-dashboards/my`,
 * `/custom-dashboards/my/:dashboardId`): the dashboards (for now every one
 * `GET /dashboards` lists; later only the stored custom ones), drawn exactly
 * as Dashboard draws its own -- the same `GET /dashboards` list and the same
 * `AgentsLandingPagePilot` (card, date range, widget grid), both reused
 * unchanged. Only the header is this page's own: a title and a switcher over
 * them. No id in the URL opens the first one.
 */
export default function MyDashboardsPage(): JSX.Element {
  const { dashboardId } = useParams<{ dashboardId?: string }>();
  const navigate = useNavigate();
  const dashboardList = useDashboardList();

  // Every dashboard `GET /dashboards` lists, built-in ones included, until
  // custom dashboards are stored: then this lists only the stored ones.
  const customDashboards = useMemo(() => dashboardList.data ?? [], [dashboardList.data]);

  if (dashboardList.isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Typography variant="h5">My Dashboards</Typography>
        <Typography variant="body2" color="text.secondary">
          Could not load the dashboard list.
        </Typography>
      </Box>
    );
  }

  if (dashboardList.isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rounded" height={32} width={240} />
        <Skeleton variant="rounded" height={200} />
      </Box>
    );
  }

  if (customDashboards.length === 0) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Typography variant="h5">My Dashboards</Typography>
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            gap: 1,
            py: 8,
            textAlign: "center",
          }}
        >
          <LayoutDashboard size={32} />
          <Typography variant="h6">You have no dashboards yet</Typography>
          <Typography variant="body2" color="text.secondary">
            Dashboards you generate from a description will appear here. Only you can see them.
          </Typography>
          <Box sx={{ mt: 2, display: "flex", gap: 1 }}>
            <GenerateDashboardButton />
            <GenerateCodeDashboardButton />
          </Box>
        </Box>
      </Box>
    );
  }

  const selected = customDashboards.find((d) => d.id === dashboardId);
  if (!selected) {
    // No id, or one that is not a custom dashboard: open the first one.
    return <Navigate to={`${BASE_PATH}/${encodeURIComponent(customDashboards[0].id)}`} replace />;
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 2,
          flexWrap: "wrap",
        }}
      >
        <Box>
          <Typography variant="h5">My Dashboards</Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            {selected.displayName}
          </Typography>
        </Box>
        <Box sx={{ display: "flex", gap: 1, alignItems: "center", flexWrap: "wrap" }}>
          <FormControl size="small" sx={{ minWidth: 200 }}>
            <Select
              value={selected.id}
              onChange={(e) => navigate(`${BASE_PATH}/${encodeURIComponent(e.target.value)}`)}
              aria-label="Select dashboard"
            >
              {customDashboards.map((d) => (
                <MenuItem key={d.id} value={d.id}>
                  {d.displayName}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          <GenerateDashboardButton />
          <GenerateCodeDashboardButton />
        </Box>
      </Box>
      <AgentsLandingPagePilot dashboardId={selected.id} />
    </Box>
  );
}
