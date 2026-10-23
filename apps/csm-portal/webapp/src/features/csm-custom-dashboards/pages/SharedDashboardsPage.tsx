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

import { Box, Typography } from "@wso2/oxygen-ui";
import { UsersRound } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";

/**
 * Custom Dashboards > Shared Dashboards (`/custom-dashboards/shared`): custom
 * dashboards other users share. Nothing can be shared yet, so the page shows
 * its empty state.
 */
export default function SharedDashboardsPage(): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Typography variant="h5">Shared Dashboards</Typography>
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
        <UsersRound size={32} />
        <Typography variant="h6">No shared dashboards yet</Typography>
        <Typography variant="body2" color="text.secondary">
          Custom dashboards that others share will appear here.
        </Typography>
      </Box>
    </Box>
  );
}
