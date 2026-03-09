type ConnectorState = "checking" | "online" | "offline";

type ReaderSummary = {
  count: number;
  primaryName: string | null;
  cardPresent: boolean;
};

type ConnectorMeta = {
  version: string | null;
  buildTime: string | null;
  driver: string | null;
};

type ConnectorEvent = {
  type: string;
  status?: string;
  at: string;
  payload?: Record<string, unknown>;
};

type CardSummary = {
  sessionId?: string;
  reader?: string;
  operation?: string;
  uid?: string;
  atr?: string;
  mediaType?: string;
  payload?: Record<string, unknown>;
  details?: Record<string, string>;
  error?: string;
};

type SessionSummary = {
  id: string | null;
  readerName: string | null;
};

type WriteSummary = {
  accepted: boolean;
  payloadType: string | null;
  ndefBytes: number | null;
  pagesWritten: number | null;
  error: string | null;
};

type JsonScalar = string | number | boolean | null;
type JsonObject = Record<string, JsonScalar>;

type ActionOptions = {
  suppressErrorState?: boolean;
};

type ConnectorReader = {
  name?: string;
  cardPresent?: boolean;
};

async function getConnectorTicket(scope: "all" | "read" | "write" | "events") {
  return $fetch<{ token: string; expiresAt: number; scope: string }>("/api/connector-ticket", {
    method: "POST",
    body: { scope },
  });
}

export function useConnectorStatus() {
  const config = useRuntimeConfig();
  const state = useState<ConnectorState>("connector-state", () => "checking");
  const readerSummary = useState<ReaderSummary>("connector-reader-summary", () => ({
    count: 0,
    primaryName: null,
    cardPresent: false,
  }));
  const connectorMeta = useState<ConnectorMeta>("connector-meta", () => ({
    version: null,
    buildTime: null,
    driver: null,
  }));
  const lastCheckedAt = useState<string | null>("connector-last-checked-at", () => null);
  const lastReadAt = useState<string | null>("connector-last-read-at", () => null);
  const lastError = useState<string | null>("connector-last-error", () => null);
  const isRefreshing = useState<boolean>("connector-refreshing", () => false);
  const events = useState<ConnectorEvent[]>("connector-events", () => []);
  const cardSummary = useState<CardSummary | null>("connector-card-summary", () => null);
  const session = useState<SessionSummary>("connector-session", () => ({
    id: null,
    readerName: null,
  }));
  const lastWrite = useState<WriteSummary | null>("connector-last-write", () => null);
  const socket = useState<WebSocket | null>("connector-event-socket", () => null);

  function selectPrimaryReader(readers: ConnectorReader[]) {
    return (
      readers.find((reader) => reader.name?.includes("PICC")) ??
      readers.find((reader) => reader.cardPresent) ??
      readers[0] ??
      null
    );
  }

  function pushEvent(event: ConnectorEvent) {
    events.value = [event, ...events.value].slice(0, 12);
  }

  async function openEventsChannel() {
    if (import.meta.server || socket.value) {
      return;
    }

    try {
      const ticket = await getConnectorTicket("events");
      const url = new URL(
        config.public.connectorBaseUrl.replace("http://", "ws://").replace("https://", "wss://"),
      );
      url.pathname = "/events";
      url.searchParams.set("token", ticket.token);

      const ws = new WebSocket(url.toString());
      ws.addEventListener("message", (message) => {
        const event = JSON.parse(message.data) as ConnectorEvent;
        pushEvent(event);
      });
      ws.addEventListener("close", () => {
        socket.value = null;
      });
      socket.value = ws;
    } catch (error) {
      lastError.value = error instanceof Error ? error.message : "Unable to open events channel";
    }
  }

  async function withConnectorHeaders(scope: "all" | "read" | "write") {
    const ticket = await getConnectorTicket(scope);
    return {
      "X-Bridge-Token": ticket.token,
    };
  }

  async function refresh() {
    if (isRefreshing.value) {
      return;
    }

    isRefreshing.value = true;
    lastError.value = null;
    state.value = "checking";

    try {
      const headers = await withConnectorHeaders("all");
      const healthResponse = await fetch(`${config.public.connectorBaseUrl}/health`, {
        method: "GET",
      });

      if (!healthResponse.ok) {
        throw new Error(`Connector health returned ${healthResponse.status}`);
      }

      state.value = "online";
      const healthPayload = (await healthResponse.json()) as {
        version?: string;
        buildTime?: string;
        driver?: string;
      };
      connectorMeta.value = {
        version: healthPayload.version ?? null,
        buildTime: healthPayload.buildTime ?? null,
        driver: healthPayload.driver ?? null,
      };

      const readersResponse = await fetch(`${config.public.connectorBaseUrl}/readers`, {
        method: "GET",
        headers,
      });

      if (readersResponse.ok) {
        const payload = (await readersResponse.json()) as
          | { readers?: ConnectorReader[] }
          | ConnectorReader[];
        const readers = Array.isArray(payload) ? payload : (payload.readers ?? []);
        const primaryReader = selectPrimaryReader(readers);

        readerSummary.value = {
          count: readers.length,
          primaryName: primaryReader?.name ?? null,
          cardPresent: primaryReader?.cardPresent ?? false,
        };
      } else {
        readerSummary.value = {
          count: 0,
          primaryName: null,
          cardPresent: false,
        };
      }
    } catch (error) {
      state.value = "offline";
      connectorMeta.value = {
        version: null,
        buildTime: null,
        driver: null,
      };
      readerSummary.value = {
        count: 0,
        primaryName: null,
        cardPresent: false,
      };
      lastError.value = error instanceof Error ? error.message : "Unknown connector error";
    } finally {
      lastCheckedAt.value = new Date().toISOString();
      isRefreshing.value = false;
    }
  }

  async function connectSession(options: ActionOptions = {}) {
    try {
      const headers = await withConnectorHeaders("all");
      const response = await fetch(`${config.public.connectorBaseUrl}/session/connect`, {
        method: "POST",
        headers: {
          ...headers,
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ readerName: readerSummary.value.primaryName }),
      });

      const payload = (await response.json()) as {
        id?: string;
        readerName?: string;
        error?: string;
      };
      if (!response.ok) {
        throw new Error(payload.error ?? "Unable to create session");
      }

      session.value = {
        id: payload.id ?? null,
        readerName: payload.readerName ?? null,
      };
      lastError.value = null;
      pushEvent({
        type: "ui.session.connected",
        status: "ok",
        at: new Date().toISOString(),
        payload,
      });
      return session.value;
    } catch (error) {
      const message = error instanceof Error ? error.message : "Unable to create session";
      if (!options.suppressErrorState) {
        lastError.value = message;
        pushEvent({
          type: "ui.session.connected",
          status: "error",
          at: new Date().toISOString(),
          payload: { message },
        });
      }
      throw error;
    }
  }

  async function readCard(options: ActionOptions = {}) {
    try {
      if (!session.value.id) {
        await connectSession(options);
      }

      const headers = await withConnectorHeaders("read");
      const response = await fetch(`${config.public.connectorBaseUrl}/card/read`, {
        method: "POST",
        headers: {
          ...headers,
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          sessionId: session.value.id,
          operation: "summary",
        }),
      });

      const payload = (await response.json()) as CardSummary;
      if (!response.ok) {
        throw new Error(payload.error ?? "Unable to read card");
      }

      cardSummary.value = payload;
      lastReadAt.value = new Date().toISOString();
      lastError.value = null;
      pushEvent({
        type: "ui.read.requested",
        status: "ok",
        at: new Date().toISOString(),
        payload,
      });
      return payload;
    } catch (error) {
      const message = error instanceof Error ? error.message : "Unable to read card";
      lastReadAt.value = null;
      if (!options.suppressErrorState) {
        lastError.value = message;
        pushEvent({
          type: "ui.read.requested",
          status: "error",
          at: new Date().toISOString(),
          payload: { message },
        });
      }
      throw error;
    }
  }

  function deriveDemoLabel(content: JsonObject) {
    const labelCandidates = [
      content.documentNo,
      content.recordId,
      content.itemCode,
      content.batchNo,
      content.status,
    ];

    for (const candidate of labelCandidates) {
      if (typeof candidate === "string" && candidate.trim().length > 0) {
        return candidate.trim().slice(0, 96);
      }
    }

    return "ERP Demo Payload";
  }

  async function writeJsonContent(content: JsonObject, options: ActionOptions = {}) {
    try {
      if (!session.value.id) {
        await connectSession(options);
      }

      const headers = await withConnectorHeaders("write");
      const response = await fetch(`${config.public.connectorBaseUrl}/card/write`, {
        method: "POST",
        headers: {
          ...headers,
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          sessionId: session.value.id,
          operation: "ndef-v1",
          payload: {
            version: 1,
            type: "nfc-tool/demo",
            label: deriveDemoLabel(content),
            content,
            updatedAt: new Date().toISOString(),
          },
        }),
      });

      const payload = (await response.json()) as Record<string, unknown> & {
        error?: string;
        details?: Record<string, string>;
      };
      const details = payload.details;
      if (!response.ok && response.status !== 409) {
        throw new Error(payload.error ?? "Unable to write card");
      }

      lastWrite.value = {
        accepted: Boolean(payload.accepted),
        payloadType:
          typeof payload.payloadType === "string"
            ? payload.payloadType
            : typeof details?.payloadType === "string"
              ? details.payloadType
              : null,
        ndefBytes:
          typeof payload.ndefBytes === "number"
            ? payload.ndefBytes
            : typeof details?.ndefBytes === "string"
              ? Number(details.ndefBytes)
              : null,
        pagesWritten:
          typeof payload.pagesWritten === "number"
            ? payload.pagesWritten
            : typeof details?.pagesWritten === "string"
              ? Number(details.pagesWritten)
              : null,
        error: typeof payload.error === "string" ? payload.error : null,
      };
      lastError.value = response.ok
        ? null
        : typeof payload.error === "string"
          ? payload.error
          : null;
      if (response.ok) {
        cardSummary.value = {
          ...cardSummary.value,
          sessionId: session.value.id ?? undefined,
          reader: session.value.readerName ?? readerSummary.value.primaryName ?? undefined,
          operation: "summary",
          mediaType: "application/json",
          payload: {
            version: 1,
            type: "nfc-tool/demo",
            label: deriveDemoLabel(content),
            content,
            updatedAt: new Date().toISOString(),
          },
        };
        session.value = {
          id: null,
          readerName: readerSummary.value.primaryName,
        };
      }
      pushEvent({
        type: "ui.write.requested",
        status: response.ok ? "ok" : "blocked",
        at: new Date().toISOString(),
        payload: {
          ...payload,
          operation: "ndef-v1",
        },
      });
      return payload;
    } catch (error) {
      const message = error instanceof Error ? error.message : "Unable to write card";
      if (!options.suppressErrorState) {
        lastError.value = message;
        lastWrite.value = {
          accepted: false,
          payloadType: null,
          ndefBytes: null,
          pagesWritten: null,
          error: message,
        };
        pushEvent({
          type: "ui.write.requested",
          status: "error",
          at: new Date().toISOString(),
          payload: { message },
        });
      }
      throw error;
    }
  }

  const readerState = computed(() => {
    if (state.value !== "online") {
      return "waiting-for-connector";
    }

    if (readerSummary.value.count === 0) {
      return "reader-missing";
    }

    return "reader-ready";
  });

  return {
    state,
    readerState,
    readerSummary,
    connectorMeta,
    cardSummary,
    lastWrite,
    session,
    events,
    lastCheckedAt,
    lastReadAt,
    lastError,
    isRefreshing,
    refresh,
    openEventsChannel,
    connectSession,
    readCard,
    writeJsonContent,
  };
}
