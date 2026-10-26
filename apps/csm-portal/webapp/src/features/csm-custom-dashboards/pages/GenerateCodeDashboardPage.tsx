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

import { Box, Button, Chip, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft, Code, MessageSquare, RefreshCw } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useState } from "react";
import { useNavigate } from "react-router";
import { useGenerateCodeDashboard } from "@features/csm-custom-dashboards/api/useGenerateCodeDashboard";
import CodeDashboardFrame from "@features/csm-custom-dashboards/components/CodeDashboardFrame";
import DiscardGeneratedDashboardDialog from "@features/csm-custom-dashboards/components/DiscardGeneratedDashboardDialog";
import GenerateChatPanel, {
  type GenerateChatMessage,
} from "@features/csm-custom-dashboards/components/GenerateChatPanel";

const MY_DASHBOARDS_PATH = "/custom-dashboards/my";

/**
 * Custom Dashboards > My Dashboards > new (code)
 * (`/custom-dashboards/my/new-code`), opened by the experimental "Generate AI
 * Code" button: the code-based counterpart of GenerateDashboardPage, where the
 * model writes the dashboard as code that will run in a sandbox and read live
 * data through a fixed set of portal APIs.
 *
 * Same shape as GenerateDashboardPage, reusing its chat panel and discard
 * dialog unchanged: each message goes to `useGenerateCodeDashboard` with the
 * current source; the reply is added to the chat and the returned source
 * replaces the draft. The left side has two tabs: Preview runs the source in
 * a sandboxed iframe (CodeDashboardFrame, with the viewer's real data) and
 * Code shows it as plain text. Refresh (Preview only) runs the same source
 * again in a fresh iframe, so the dashboard reloads with current data and the
 * model is not called. Nothing is kept after leaving the page.
 *
 * Back names its destination because this page is only ever opened from My
 * Dashboards (the webapp CLAUDE.md's fixed-destination exception).
 */
export default function GenerateCodeDashboardPage(): JSX.Element {
  const navigate = useNavigate();
  const [messages, setMessages] = useState<GenerateChatMessage[]>([]);
  const [chatOpen, setChatOpen] = useState(true);
  const [source, setSource] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [confirmDiscardOpen, setConfirmDiscardOpen] = useState(false);
  const [view, setView] = useState<"preview" | "code">("preview");
  // Bumped by Refresh: a new key gives the preview a fresh iframe, so the same
  // source runs again and its portal.* calls read current data (no model call).
  const [runCount, setRunCount] = useState(0);
  const generate = useGenerateCodeDashboard();

  const hasWork = source !== null || messages.length > 0;
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
      const result = await generate(text, source);
      setSource(result.source);
      addMessage("assistant", result.reply);
    } catch {
      // The source is left as it was, so a failed turn never loses the draft.
      // With a source on screen the message was a change to it, not a new dashboard.
      addMessage(
        "assistant",
        source ? "This dashboard cannot be modified with your input." : "The dashboard cannot be generated from the input.",
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
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Typography variant="h5">New dashboard (code)</Typography>
          <Chip label="Experimental" size="small" color="warning" variant="outlined" />
        </Box>
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
        </Box>
      </Box>

      {/* A bounded height, so the chat's message list scrolls inside the panel. */}
      <Box sx={{ display: "flex", gap: 2, alignItems: "stretch", height: "calc(100dvh - 240px)", minHeight: 520 }}>
        {/* Left: the generated dashboard (Preview) or its source (Code), or a placeholder until the first reply. */}
        {source !== null ? (
          <Box
            sx={{
              flex: 1,
              minWidth: 0,
              border: 1,
              borderColor: "divider",
              borderRadius: 1,
              p: 2,
              display: "flex",
              flexDirection: "column",
              gap: 1,
              minHeight: 0,
            }}
          >
            <Box sx={{ display: "flex", alignItems: "center", borderBottom: 1, borderColor: "divider" }}>
              <Tabs
                value={view}
                onChange={(_, value: "preview" | "code") => setView(value)}
                sx={{ minHeight: 0, flex: 1 }}
              >
                <Tab label="Preview" value="preview" />
                <Tab label="Code" value="code" />
              </Tabs>
              {view === "preview" && (
                <Button
                  variant="text"
                  size="small"
                  startIcon={<RefreshCw size={16} />}
                  onClick={() => setRunCount((n) => n + 1)}
                >
                  Refresh
                </Button>
              )}
            </Box>
            {view === "preview" ? (
              <CodeDashboardFrame key={`${runCount}:${source}`} source={source} />
            ) : (
              <Box
                component="pre"
                aria-label="Generated code"
                sx={{
                  m: 0,
                  flex: 1,
                  minHeight: 0,
                  overflow: "auto",
                  p: 1.5,
                  borderRadius: 1,
                  bgcolor: "action.hover",
                  fontFamily: "monospace",
                  fontSize: 13,
                }}
              >
                {source}
              </Box>
            )}
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
            <Code size={32} />
            <Typography variant="h6">The generated dashboard will appear here</Typography>
            <Typography variant="body2" color="text.secondary">
              Describe the dashboard you want in the chat. It runs here with your real data.
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
