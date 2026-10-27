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

import {
  Alert,
  Avatar,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  InputAdornment,
  Skeleton,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Search, X } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useMemo, useState } from "react";
import { BackendApiError } from "@api/backend/client";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { initialsOf } from "@utils/userClaims";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import { INTERNAL_USER_ROLES } from "@features/csm-users/types/csmUsers";
import {
  useDeleteCustomDashboardShare,
  useGetCustomDashboardShares,
  usePutCustomDashboardShare,
} from "@features/csm-custom-dashboards/api/useDashboardShares";

interface ShareCustomDashboardDialogProps {
  open: boolean;
  /** The dashboard being shared: one of the caller's own. */
  dashboard: { id: string; name: string };
  /** The signed-in user's email, left out of the search results. */
  currentUserEmail?: string;
  onClose: () => void;
}

/**
 * What to tell the owner when a share or unshare is refused: the backend's
 * message for a refusal they can act on (4xx, e.g. not internal staff), as a
 * sentence; anything else is generic.
 */
function shareErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof BackendApiError && err.status >= 400 && err.status < 500 && err.message) {
    const msg = err.message.charAt(0).toUpperCase() + err.message.slice(1);
    return /[.!?]$/.test(msg) ? msg : `${msg}.`;
  }
  return fallback;
}

/**
 * Shares one of the caller's saved dashboards, view only. A search over
 * internal staff (the Assign engineer dialog's search: `POST /users/search`,
 * internal roles, active only), where clicking a person shares with them;
 * below it, who it is shared with, each with a remove button. The caller and
 * people it is already shared with are left out of the results. A refusal
 * shows in the dialog; the backend is the real check.
 */
export default function ShareCustomDashboardDialog({
  open,
  dashboard,
  currentUserEmail,
  onClose,
}: ShareCustomDashboardDialogProps): JSX.Element {
  const [input, setInput] = useState("");
  const [error, setError] = useState<string | null>(null);
  const shares = useGetCustomDashboardShares(open ? dashboard.id : undefined);
  const share = usePutCustomDashboardShare();
  const unshare = useDeleteCustomDashboardShare();

  const search = useDebouncedValue(input.trim(), 300);
  const results = useSearchUsers(
    {
      filters: {
        ...(search.length > 0 && { searchQuery: search }),
        roleIds: INTERNAL_USER_ROLES,
        active: true,
      },
      pagination: { limit: 8, offset: 0 },
    },
    open,
  );

  const sharedIds = useMemo(() => new Set((shares.data ?? []).map((s) => s.user.id)), [shares.data]);
  const me = currentUserEmail?.toLowerCase();
  const people = useMemo(
    () =>
      (results.data?.users ?? []).filter(
        (u) =>
          !!u.email &&
          u.active !== false &&
          (u.userType ? u.userType === "internal" : true) &&
          u.email.toLowerCase() !== me &&
          !sharedIds.has(u.id),
      ),
    [results.data, me, sharedIds],
  );

  const busy = share.isPending || unshare.isPending;

  const handleShare = async (userId: string): Promise<void> => {
    setError(null);
    try {
      await share.mutateAsync({ dashboardId: dashboard.id, userId });
    } catch (err) {
      setError(shareErrorMessage(err, "The dashboard couldn't be shared. Try again in a moment."));
    }
  };

  const handleUnshare = async (userId: string): Promise<void> => {
    setError(null);
    try {
      await unshare.mutateAsync({ dashboardId: dashboard.id, userId });
    } catch (err) {
      setError(shareErrorMessage(err, "The dashboard couldn't be unshared. Try again in a moment."));
    }
  };

  const handleClose = (): void => {
    setInput("");
    setError(null);
    onClose();
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="xs" fullWidth>
      <DialogTitle>Share &ldquo;{dashboard.name}&rdquo;</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <Typography variant="body2" color="text.secondary">
            People you share with can view this dashboard in their Shared Dashboards and see your later changes.
            They can&apos;t edit it.
          </Typography>

          {error && (
            <Alert severity="error" onClose={() => setError(null)}>
              {error}
            </Alert>
          )}

          <TextField
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Search people by name or email…"
            size="small"
            fullWidth
            autoFocus
            slotProps={{
              htmlInput: { "aria-label": "Search people" },
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={16} />
                  </InputAdornment>
                ),
              },
            }}
          />

          <Box sx={{ minHeight: 120 }} aria-label="People to share with">
            {results.isFetching ? (
              <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5, pt: 0.5 }}>
                {[0, 1, 2].map((i) => (
                  <Skeleton key={i} variant="rounded" height={36} />
                ))}
              </Box>
            ) : results.isError ? (
              <Typography variant="body2" color="error" sx={{ py: 1 }}>
                Could not search people. Try again.
              </Typography>
            ) : people.length === 0 ? (
              <Typography variant="body2" color="text.secondary" sx={{ py: 1 }}>
                No matching people.
              </Typography>
            ) : (
              <Box sx={{ display: "flex", flexDirection: "column" }}>
                {people.map((u) => {
                  const name = u.name.trim() || u.userName;
                  return (
                    <Button
                      key={u.id}
                      variant="text"
                      color="inherit"
                      disabled={busy}
                      onClick={() => void handleShare(u.id)}
                      aria-label={`Share with ${name}`}
                      sx={{ justifyContent: "flex-start", textTransform: "none", px: 1, py: 0.75, gap: 1.25 }}
                    >
                      <Avatar sx={{ width: 28, height: 28, fontSize: "0.75rem" }}>{initialsOf(name)}</Avatar>
                      <Box sx={{ minWidth: 0, textAlign: "left" }}>
                        <Typography variant="body2" sx={{ lineHeight: 1.2 }} noWrap>
                          {name}
                        </Typography>
                        <Typography variant="caption" color="text.secondary" noWrap sx={{ display: "block" }}>
                          {u.email}
                        </Typography>
                      </Box>
                    </Button>
                  );
                })}
              </Box>
            )}
          </Box>

          <Typography variant="subtitle2">Shared with</Typography>
          <Box aria-label="Shared with">
            {shares.isLoading ? (
              <Skeleton variant="rounded" height={36} />
            ) : shares.isError || shares.data === null ? (
              <Typography variant="body2" color="error">
                Could not load who this dashboard is shared with.
              </Typography>
            ) : (shares.data ?? []).length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                Not shared with anyone yet.
              </Typography>
            ) : (
              (shares.data ?? []).map((s) => {
                const name = s.user.name || s.user.email;
                return (
                  <Box key={s.user.id ?? s.user.email} sx={{ display: "flex", alignItems: "center", gap: 1.25, px: 1, py: 0.5 }}>
                    <Avatar sx={{ width: 28, height: 28, fontSize: "0.75rem" }}>{initialsOf(name)}</Avatar>
                    <Box sx={{ minWidth: 0, flex: 1 }}>
                      <Typography variant="body2" sx={{ lineHeight: 1.2 }} noWrap>
                        {name}
                      </Typography>
                      <Typography variant="caption" color="text.secondary" noWrap sx={{ display: "block" }}>
                        {s.user.email}
                      </Typography>
                    </Box>
                    {s.user.id && (
                      <Tooltip title="Stop sharing">
                        <span>
                          <IconButton
                            size="small"
                            disabled={busy}
                            aria-label={`Stop sharing with ${name}`}
                            onClick={() => void handleUnshare(s.user.id as string)}
                          >
                            {unshare.isPending && unshare.variables?.userId === s.user.id ? (
                              <CircularProgress size={14} color="inherit" />
                            ) : (
                              <X size={16} />
                            )}
                          </IconButton>
                        </span>
                      </Tooltip>
                    )}
                  </Box>
                );
              })
            )}
          </Box>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose}>Done</Button>
      </DialogActions>
    </Dialog>
  );
}
