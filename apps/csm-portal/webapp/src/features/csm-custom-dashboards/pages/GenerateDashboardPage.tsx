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

import { Box, Button, Tooltip, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft, LayoutDashboard, MessageSquare } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useState } from "react";
import { useNavigate } from "react-router";
import type { BeDashboard } from "@api/backend/types";
import DashboardWidgetGrid from "@features/csm-dashboard/components/DashboardWidgetGrid";
import { useGenerateDashboard } from "@features/csm-custom-dashboards/api/useGenerateDashboard";
import DiscardGeneratedDashboardDialog from "@features/csm-custom-dashboards/components/DiscardGeneratedDashboardDialog";
import GenerateChatPanel, {
  type GenerateChatMessage,
} from "@features/csm-custom-dashboards/components/GenerateChatPanel";

const MY_DASHBOARDS_PATH = "/custom-dashboards/my";

/**
 * Custom Dashboards > My Dashboards > new (`/custom-dashboards/my/new`), opened
 * by the Generate Dashboard button: the generated dashboard's preview on the
 * left, the chat with the model on the right.
 *
 * The page owns the chat (its messages, whether the panel is open, whether a
 * reply is pending) and the draft: the latest dashboard a message produced.
 * Each message goes to `useGenerateDashboard` with the current draft; the reply
 * is added to the chat and the returned dashboard replaces the draft, drawn on
 * the left by Dashboard's own widget grid with live data. Closing the panel
 * (its ✕) gives the preview the full width; "Open chat" brings it back with
 * the conversation intact. Nothing is kept after leaving the page.
 *
 * Leaving: Back and Discard both go through `requestLeave`, which asks first
 * (DiscardGeneratedDashboardDialog) when there is anything to lose -- a draft
 * or any message -- and leaves straight away otherwise. Other ways out (the
 * menu, a refresh, closing the tab) are not guarded: this app's router is not
 * a data router, so `useBlocker` is not available. Save is shown but disabled
 * until custom dashboards can be stored.
 *
 * Back names its destination because this page is only ever opened from My
 * Dashboards (the webapp CLAUDE.md's fixed-destination exception).
 */
export default function GenerateDashboardPage(): JSX.Element {
  const navigate = useNavigate();
  const [messages, setMessages] = useState<GenerateChatMessage[]>([]);
  const [chatOpen, setChatOpen] = useState(true);
  const [draft, setDraft] = useState<BeDashboard | null>(null);
  const [busy, setBusy] = useState(false);
  const [confirmDiscardOpen, setConfirmDiscardOpen] = useState(false);
  const generate = useGenerateDashboard();

  const hasWork = draft !== null || messages.length > 0;
  const leave = (): void => {
    void navigate(MY_DASHBOARDS_PATH);
  };
  const requestLeave = (): void => {
    if (hasWork) setConfirmDiscardOpen(true);
    else leave();
  };

  const addMessage = (role: GenerateChatMessage["role"], text: string): void => {
    setMessages((prev) => [...prev, { id: crypto.randomUUID(), role, text }]);
  };

  const handleSend = async (text: string): Promise<void> => {
    addMessage("user", text);
    setBusy(true);
    try {
      const result = await generate(text, draft);
      setDraft(result.dashboard);
      addMessage("assistant", result.reply);
    } catch {
      // The draft is left as it was, so a failed turn never loses the preview.
      // With a draft on screen the message was a change to it, not a new dashboard.
      addMessage(
        "assistant",
        draft ? "This dashboard cannot be modified with your input." : "The dashboard cannot be generated from the input.",
      );
    } finally {
      setBusy(false);
    }
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Button
        variant="text"
        size="small"
        startIcon={<ArrowLeft size={16} />}
        onClick={requestLeave}
        sx={{ alignSelf: "flex-start" }}
      >
        Back to My Dashboards
      </Button>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
        <Typography variant="h5">New dashboard</Typography>
        <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
          {!chatOpen && (
            <Button
              variant="outlined"
              size="small"
              startIcon={<MessageSquare size={16} />}
              onClick={() => setChatOpen(true)}
            >
              Open chat
            </Button>
          )}
          <Button variant="outlined" size="small" onClick={requestLeave}>
            Discard
          </Button>
          {/* A disabled button fires no events, so the tooltip sits on a span. */}
          <Tooltip title="Saving comes in a later step">
            <span>
              <Button variant="contained" color="primary" size="small" disabled>
                Save
              </Button>
            </span>
          </Tooltip>
        </Box>
      </Box>

      {/* A bounded height, so the chat's message list scrolls inside the panel. */}
      <Box sx={{ display: "flex", gap: 2, alignItems: "stretch", height: "calc(100dvh - 240px)", minHeight: 520 }}>
        {/* Left: the generated dashboard's preview, or a placeholder until the first reply. */}
        {draft ? (
          <Box
            sx={{
              flex: 1,
              minWidth: 0,
              border: 1,
              borderColor: "divider",
              borderRadius: 1,
              p: 2,
              overflowY: "auto",
              display: "flex",
              flexDirection: "column",
              gap: 2,
            }}
          >
            <Box>
              <Typography variant="caption" color="text.secondary">
                Preview
              </Typography>
              <Typography variant="h6">{draft.displayName}</Typography>
            </Box>
            <DashboardWidgetGrid widgets={draft.widgets} />
          </Box>
        ) : (
          <Box
            sx={{
              flex: 1,
              minWidth: 0,
              border: 1,
              borderColor: "divider",
              borderRadius: 1,
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              justifyContent: "center",
              gap: 1,
              p: 3,
              textAlign: "center",
            }}
          >
            <LayoutDashboard size={32} />
            <Typography variant="h6">Your dashboard will appear here</Typography>
            <Typography variant="body2" color="text.secondary">
              Describe the dashboard you want in the chat, and a preview with live data shows here.
            </Typography>
          </Box>
        )}

        {/* Right: the chat with the model. */}
        {chatOpen && (
          <Box
            sx={{
              width: 400,
              flexShrink: 0,
              border: 1,
              borderColor: "divider",
              borderRadius: 1,
              overflow: "hidden",
            }}
          >
            <GenerateChatPanel
              messages={messages}
              onSend={(text) => void handleSend(text)}
              onClose={() => setChatOpen(false)}
              busy={busy}
            />
          </Box>
        )}
      </Box>

      <DiscardGeneratedDashboardDialog
        open={confirmDiscardOpen}
        onCancel={() => setConfirmDiscardOpen(false)}
        onConfirm={leave}
      />
    </Box>
  );
}
