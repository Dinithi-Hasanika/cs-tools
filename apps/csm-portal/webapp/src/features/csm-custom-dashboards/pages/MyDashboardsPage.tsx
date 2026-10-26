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

import { Box, Button, FormControl, IconButton, MenuItem, Select, Skeleton, Tooltip, Typography } from "@wso2/oxygen-ui";
import { Code, LayoutDashboard, Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useState } from "react";
import { Navigate, useNavigate, useParams } from "react-router";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import DashboardWidgetGrid from "@features/csm-dashboard/components/DashboardWidgetGrid";
import {
  type CustomDashboardSummary,
  useDeleteCustomDashboard,
  useGetCustomDashboard,
  useGetCustomDashboards,
} from "@features/csm-custom-dashboards/api/useCustomDashboards";
import DeleteCustomDashboardDialog from "@features/csm-custom-dashboards/components/DeleteCustomDashboardDialog";

const BASE_PATH = "/dashboard/my";

/**
 * Same props as Create case (CsmCasesPage). With dashboards it sits in the
 * header next to the switcher; with none it is the centred call to action of
 * the empty state. Opens the generate page (`/dashboard/my/new`).
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
 * (`/dashboard/my/new-code`), where the model writes the dashboard as
 * code instead of a JSON definition. Shown beside GenerateDashboardButton so
 * the two approaches can be compared; removed or promoted once one is chosen.
 * Hidden while the JSON approach ships first (phase 1): set
 * SHOW_CODE_DASHBOARD_BUTTON to true to bring it back. The page itself stays
 * reachable at its URL.
 */
const SHOW_CODE_DASHBOARD_BUTTON = false;

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
 * One saved dashboard (`GET /dashboards/custom/{id}`), drawn with Dashboard's
 * own widget grid (reused unchanged), the same grid the generate page's
 * preview uses, so a saved dashboard looks exactly as it did when it was
 * saved. Not `AgentsLandingPagePilot`: that one loads its dashboard from the
 * system registry (`GET /dashboards/{id}`), which a saved dashboard is not in.
 *
 * A dashboard saved under rules that have since changed comes back as an
 * error whose message asks the user to generate it again; it is shown as is.
 */
function SavedDashboardView({ id }: { id: string }): JSX.Element {
  const saved = useGetCustomDashboard(id);

  const frame = { border: 1, borderColor: "divider", borderRadius: 1, p: 2 };
  if (saved.isLoading) {
    return (
      <Box sx={frame}>
        <Skeleton variant="rounded" height={200} />
      </Box>
    );
  }
  if (saved.isError || !saved.data) {
    return (
      <Box sx={frame}>
        <Typography variant="body2" color="text.secondary">
          {saved.isError
            ? saved.error.message || "Could not load this dashboard."
            : "This dashboard no longer exists."}
        </Typography>
      </Box>
    );
  }
  return (
    <Box sx={frame}>
      <DashboardWidgetGrid widgets={saved.data.dashboard.widgets} />
    </Box>
  );
}

/**
 * Dashboards > My Dashboards (`/dashboard/my`,
 * `/dashboard/my/:dashboardId`): the dashboards the user saved from the
 * generate page (`GET /dashboards/custom`, only their own), with a switcher
 * over them in the header. No id in the URL, or one that is not in the list,
 * opens the first one; while the list is being re-read (e.g. just after a
 * save) an unknown id waits for it instead, so a dashboard saved a moment ago
 * is opened rather than skipped.
 *
 * Each entry of the switcher has its own delete icon, as Saved views has
 * (SavedViewsMenu); clicking it closes the menu without switching and asks
 * first (DeleteCustomDashboardDialog), since a deleted dashboard cannot be
 * brought back. Deleting the open dashboard opens `/dashboard/my` (the first
 * one left, or the empty state); deleting another keeps the open one. A failed
 * delete shows the error banner and leaves the dashboard as it is.
 */
export default function MyDashboardsPage(): JSX.Element {
  const { dashboardId } = useParams<{ dashboardId?: string }>();
  const navigate = useNavigate();
  const dashboardList = useGetCustomDashboards();
  const customDashboards = dashboardList.data ?? [];
  const deleteDashboard = useDeleteCustomDashboard();
  const { showSuccess } = useSuccessBanner();
  const { showError } = useErrorBanner();
  const [menuOpen, setMenuOpen] = useState(false);
  // The dashboard the delete dialog asks about. Kept after the dialog closes,
  // so its name does not vanish during the closing animation.
  const [deleteTarget, setDeleteTarget] = useState<CustomDashboardSummary | null>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);

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

  const selected = customDashboards.find((d) => d.id === dashboardId?.toLowerCase());

  if (dashboardList.isLoading || (dashboardId && !selected && dashboardList.isFetching)) {
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
            {SHOW_CODE_DASHBOARD_BUTTON && <GenerateCodeDashboardButton />}
          </Box>
        </Box>
      </Box>
    );
  }

  if (!selected) {
    // No id, or one that is not a custom dashboard: open the first one.
    return <Navigate to={`${BASE_PATH}/${encodeURIComponent(customDashboards[0].id)}`} replace />;
  }

  const handleDelete = async (target: CustomDashboardSummary): Promise<void> => {
    try {
      await deleteDashboard.mutateAsync(target.id);
      setDeleteOpen(false);
      showSuccess(`"${target.name}" was deleted.`);
      if (target.id === selected.id) void navigate(BASE_PATH);
    } catch (err) {
      setDeleteOpen(false);
      showError("The dashboard couldn't be deleted. Try again in a moment.", err);
    }
  };

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
            {selected.name}
          </Typography>
        </Box>
        <Box sx={{ display: "flex", gap: 1, alignItems: "center", flexWrap: "wrap" }}>
          <FormControl size="small" sx={{ minWidth: 200 }}>
            <Select
              value={selected.id}
              open={menuOpen}
              onOpen={() => setMenuOpen(true)}
              onClose={() => setMenuOpen(false)}
              onChange={(e) => navigate(`${BASE_PATH}/${encodeURIComponent(e.target.value)}`)}
              // The closed field shows the name only, not the entry's delete icon.
              renderValue={() => selected.name}
              // On the element screen readers announce: a plain aria-label
              // lands on Select's hidden input instead.
              SelectDisplayProps={{ "aria-label": "Select dashboard" }}
            >
              {customDashboards.map((d) => (
                <MenuItem key={d.id} value={d.id} sx={{ justifyContent: "space-between", gap: 2 }}>
                  <Typography component="span" variant="inherit" noWrap>
                    {d.name}
                  </Typography>
                  <Tooltip title="Delete dashboard">
                    <IconButton
                      size="small"
                      edge="end"
                      aria-label={`Delete dashboard ${d.name}`}
                      onClick={(e) => {
                        // Not a selection: keep the open dashboard.
                        e.stopPropagation();
                        setMenuOpen(false);
                        setDeleteTarget(d);
                        setDeleteOpen(true);
                      }}
                    >
                      <Trash2 size={15} />
                    </IconButton>
                  </Tooltip>
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          <GenerateDashboardButton />
          {SHOW_CODE_DASHBOARD_BUTTON && <GenerateCodeDashboardButton />}
        </Box>
      </Box>
      <SavedDashboardView id={selected.id} />
      <DeleteCustomDashboardDialog
        open={deleteOpen}
        name={deleteTarget?.name ?? ""}
        deleting={deleteDashboard.isPending}
        onCancel={() => setDeleteOpen(false)}
        onConfirm={() => {
          if (deleteTarget) void handleDelete(deleteTarget);
        }}
      />
    </Box>
  );
}
