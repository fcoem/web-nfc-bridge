<script setup lang="ts">
type JsonScalar = string | number | boolean | null;
type JsonObject = Record<string, JsonScalar>;

const defaultJsonContent: JsonObject = {
  documentNo: "MO-20260309-001",
  itemCode: "FG-1001",
  workstation: "PACK-01",
  quantity: 24,
  status: "ready",
};

const jsonEditor = ref(JSON.stringify(defaultJsonContent, null, 2));
const isReading = ref(false);
const isWriting = ref(false);
const actionError = ref<string | null>(null);
const actionHint = ref("按下按鈕後再把卡片靠上讀卡機。");
const {
  state,
  readerState,
  readerSummary,
  connectorMeta,
  cardSummary,
  lastWrite,
  lastReadAt,
  lastError,
  isRefreshing,
  refresh,
  readCard,
  writeJsonContent,
} = useConnectorStatus();

const { recommended: recommendedDownload, others: otherDownloads, version: connectorVersion } = useConnectorDownload();
const connectorRepo = useRuntimeConfig().public.connectorRepo as string;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function sleep(ms: number) {
  return new Promise((resolve) => {
    setTimeout(resolve, ms);
  });
}

function isRetriableTapError(message: string) {
  const normalized = message.toLowerCase();
  return (
    normalized.includes("smart card") ||
    normalized.includes("reader") ||
    normalized.includes("session") ||
    normalized.includes("card")
  );
}

const editorError = computed(() => {
  const raw = jsonEditor.value.trim();
  if (!raw) {
    return "請輸入 JSON 內容";
  }

  try {
    const parsed = JSON.parse(raw) as unknown;
    if (!isRecord(parsed)) {
      return 'JSON 內容必須是單層物件，例如 { "documentNo": "..." }';
    }

    const keys = Object.keys(parsed);
    if (keys.length === 0) {
      return "JSON 內容至少要有一個欄位";
    }

    if (keys.length > 8) {
      return "JSON 內容最多只能有 8 個欄位";
    }

    for (const [key, value] of Object.entries(parsed)) {
      if (!key.trim()) {
        return "JSON 欄位名稱不可為空白";
      }

      if (Array.isArray(value) || (typeof value === "object" && value !== null)) {
        return `JSON 欄位 ${key} 只能使用字串、數字、布林或 null`;
      }
    }

    return null;
  } catch {
    return "JSON 格式不正確";
  }
});

const parsedEditorJson = computed<JsonObject | null>(() => {
  if (editorError.value) {
    return null;
  }

  return JSON.parse(jsonEditor.value) as JsonObject;
});

const cardPayload = computed(() => {
  if (!isRecord(cardSummary.value?.payload)) {
    return null;
  }

  return cardSummary.value.payload;
});

const cardJsonContent = computed<Record<string, unknown> | null>(() => {
  const payload = cardPayload.value;
  if (!payload) {
    return null;
  }

  if (isRecord(payload.content)) {
    return payload.content;
  }

  return payload;
});

const readJsonPretty = computed(() => {
  if (!cardJsonContent.value) {
    return "";
  }

  return JSON.stringify(cardJsonContent.value, null, 2);
});

const readJsonHint = computed(() => {
  if (readJsonPretty.value) {
    return null;
  }

  const ndefReadError = cardSummary.value?.details?.ndefReadError;
  if (ndefReadError) {
    return `已讀到 UID，但 NDEF JSON 讀取失敗：${ndefReadError}`;
  }

  if (cardSummary.value?.uid) {
    return "已讀到 UID，但卡片上目前沒有可解析的 JSON。";
  }

  return null;
});

const readStatusLabel = computed(() => {
  if (isReading.value) {
    return "等待卡片貼上";
  }

  if (cardSummary.value?.uid) {
    return "已讀到卡片";
  }

  return "尚未讀卡";
});

const writeStatusLabel = computed(() => {
  if (isWriting.value) {
    return "等待卡片貼上";
  }

  if (lastWrite.value?.accepted) {
    return "已寫入卡片";
  }

  return "尚未寫卡";
});

const canRead = computed(() => {
  return state.value === "online" && readerState.value === "reader-ready" && !isReading.value;
});

const canWrite = computed(() => {
  return (
    state.value === "online" &&
    readerState.value === "reader-ready" &&
    parsedEditorJson.value !== null &&
    !editorError.value &&
    !isWriting.value
  );
});

const compactStatus = computed(() => {
  if (state.value !== "online") {
    return "Connector 未連線";
  }

  if (readerState.value !== "reader-ready") {
    return "尚未偵測到讀卡機";
  }

  return readerSummary.value.cardPresent ? "卡片已在感應區" : "讀卡機已就緒";
});

const visibleLastError = computed(() => {
  if (actionError.value) {
    return actionError.value;
  }

  if (isReading.value || isWriting.value) {
    return null;
  }

  return lastError.value;
});

const erpResultPreview = computed(() => {
  return JSON.stringify(
    {
      uid: cardSummary.value?.uid ?? null,
      reader: cardSummary.value?.reader ?? readerSummary.value.primaryName ?? null,
      json: cardJsonContent.value,
      readAt: lastReadAt.value,
      driver: connectorMeta.value.driver,
    },
    null,
    2,
  );
});

onMounted(() => {
  refresh();
});

async function waitForCardPresence<T>(action: () => Promise<T>) {
  const timeoutAt = Date.now() + 15000;
  let lastMessage = "等待卡片逾時，請按下按鈕後 15 秒內把卡片靠上讀卡機";

  while (Date.now() < timeoutAt) {
    try {
      return await action();
    } catch (error) {
      const message = error instanceof Error ? error.message : lastMessage;
      if (!isRetriableTapError(message)) {
        throw error;
      }

      lastMessage = message;
      await sleep(500);
    }
  }

  throw new Error(lastMessage);
}

function loadSampleJson() {
  jsonEditor.value = JSON.stringify(defaultJsonContent, null, 2);
}

function loadReadJson() {
  if (!cardJsonContent.value) {
    return;
  }

  jsonEditor.value = JSON.stringify(cardJsonContent.value, null, 2);
}

async function handleReadCard() {
  actionError.value = null;
  isReading.value = true;
  actionHint.value = "請在 15 秒內把卡片靠上讀卡機，系統會自動開始讀取。";

  try {
    await waitForCardPresence(async () => readCard({ suppressErrorState: true }));
    actionHint.value = "讀取完成。可以直接查看 UID 與 JSON。";
  } catch (error) {
    actionError.value = error instanceof Error ? error.message : "讀卡失敗，請再試一次";
    actionHint.value = error instanceof Error ? error.message : "讀卡失敗，請再試一次";
  } finally {
    isReading.value = false;
  }
}

async function handleWriteCard() {
  const content = parsedEditorJson.value;
  if (!content) {
    return;
  }

  actionError.value = null;
  isWriting.value = true;
  actionHint.value = "請在 15 秒內把卡片靠上讀卡機，系統會自動寫入 JSON。";

  try {
    await waitForCardPresence(async () => writeJsonContent(content, { suppressErrorState: true }));
    actionHint.value = "寫入完成。若要確認內容，可再按一次讀取。";
  } catch (error) {
    actionError.value = error instanceof Error ? error.message : "寫卡失敗，請再試一次";
    actionHint.value = error instanceof Error ? error.message : "寫卡失敗，請再試一次";
  } finally {
    isWriting.value = false;
  }
}
</script>

<template>
  <main class="erp-demo-shell min-h-screen">
    <div class="mx-auto flex min-h-screen max-w-5xl flex-col gap-5 px-5 py-6 lg:px-8 lg:py-8">
      <section class="erp-panel rounded-2xl border border-slate-200/80 p-6 lg:p-8">
        <div class="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
          <div class="space-y-3">
            <p class="eyebrow text-xs text-sky-700">NFC 讀寫器 Demo</p>
            <h1 class="max-w-3xl text-3xl font-semibold tracking-tight text-slate-950 sm:text-4xl">
              點一下，再靠卡。
            </h1>
            <p class="max-w-2xl text-base leading-7 text-slate-600">
              這個 Demo 只保留最少流程：讀取 UID、讀取 JSON、寫入
              JSON。<br>讀卡與寫卡都改成先按按鈕，再把卡片靠上讀卡機。
            </p>
          </div>

          <div
            v-if="state === 'online'"
            class="rounded-xl border border-slate-200 bg-white/80 px-4 py-3 text-sm text-slate-600"
          >
            <p><strong>狀態</strong>：{{ compactStatus }}</p>
            <p><strong>Reader</strong>：{{ readerSummary.primaryName ?? "尚未選定" }}</p>
            <p><strong>Driver</strong>：{{ connectorMeta.driver ?? "n/a" }}</p>
          </div>
        </div>
      </section>

      <section v-if="state === 'offline'" class="rounded-2xl border border-amber-200 bg-amber-50/80 px-5 py-4 text-sm">
        <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div class="space-y-1">
            <p class="font-semibold text-amber-900">
              <UIcon name="i-lucide-loader-circle" class="mr-1 inline-block animate-spin align-text-bottom" />
              Connector 未連線，正在嘗試連線⋯
            </p>
            <p class="text-amber-800">
              若尚未安裝，請下載 Connector<template v-if="connectorVersion"> v{{ connectorVersion }}</template>，安裝後會自動連線。
            </p>
          </div>
          <ClientOnly>
            <div v-if="recommendedDownload" class="flex flex-wrap items-center gap-3 shrink-0">
              <UButton
                :to="recommendedDownload.url"
                target="_blank"
                color="primary"
                size="md"
                trailing-icon="i-lucide-download"
              >
                下載 {{ recommendedDownload.label }}
              </UButton>
              <a
                v-for="dl in otherDownloads"
                :key="dl.platformKey"
                :href="dl.url"
                target="_blank"
                class="text-xs text-sky-700 underline underline-offset-2 hover:text-sky-900"
              >
                {{ dl.label }}
              </a>
            </div>
            <UButton
              v-else
              :to="`https://github.com/${connectorRepo}/releases`"
              target="_blank"
              color="primary"
              size="md"
              trailing-icon="i-lucide-external-link"
            >
              前往 GitHub 下載
            </UButton>
          </ClientOnly>
        </div>
      </section>

      <section class="grid gap-5 lg:grid-cols-[0.92fr_1.08fr]">
        <UCard class="erp-panel border border-slate-200/80 shadow-none ring-0">
          <template #header>
            <div class="space-y-1">
              <p class="eyebrow text-xs text-slate-500">Read</p>
              <h2 class="text-2xl font-semibold text-slate-950">讀取卡片</h2>
            </div>
          </template>

          <div class="space-y-4 text-sm text-slate-700">
            <p class="leading-6 text-slate-600">
              操作方式：先按「開始讀取」，再把卡片貼上讀卡機。<br>系統會等待 15 秒。
            </p>

            <div class="grid gap-3 sm:grid-cols-2">
              <UButton
                :loading="isRefreshing"
                color="neutral"
                variant="soft"
                size="lg"
                @click="refresh"
              >
                重新偵測
              </UButton>
              <UButton
                :loading="isReading"
                color="info"
                size="lg"
                :disabled="!canRead"
                @click="handleReadCard"
              >
                {{ isReading ? "等待靠卡..." : "開始讀取" }}
              </UButton>
            </div>

            <div class="rounded-xl border border-slate-200 bg-slate-50 p-4">
              <p><strong>讀取狀態</strong>：{{ readStatusLabel }}</p>
              <p><strong>Connector</strong>：{{ state }}</p>
              <p><strong>卡片在位</strong>：{{ readerSummary.cardPresent ? "是" : "否" }}</p>
              <p v-if="visibleLastError" class="mt-2 text-rose-600">
                <strong>最近錯誤</strong>：{{ visibleLastError }}
              </p>
            </div>

            <div class="rounded-xl bg-slate-950 p-4 text-slate-100">
              <p class="font-semibold">UID</p>
              <p class="mt-2 text-xl font-semibold">{{ cardSummary?.uid ?? "尚未讀到" }}</p>
              <p class="mt-3 text-xs text-slate-300">{{ actionHint }}</p>
            </div>

            <div class="rounded-xl border border-slate-200 bg-white p-4">
              <p class="font-semibold text-slate-950">讀到的 JSON</p>
              <pre class="erp-code mt-3">{{ readJsonPretty || "尚未讀到 JSON 內容" }}</pre>
              <p v-if="readJsonHint" class="mt-3 text-sm text-amber-700">
                {{ readJsonHint }}
              </p>
            </div>
          </div>
        </UCard>

        <UCard class="erp-panel border border-slate-200/80 shadow-none ring-0">
          <template #header>
            <div class="space-y-1">
              <p class="eyebrow text-xs text-slate-500">Write</p>
              <h2 class="text-2xl font-semibold text-slate-950">寫入 JSON</h2>
            </div>
          </template>

          <div class="space-y-4 text-sm text-slate-700">
            <p class="leading-6 text-slate-600">
              修改下方 JSON 後，按「開始寫入」，再把卡片貼上讀卡機。系統會用受控 application/json
              NDEF 寫入。
            </p>

            <div class="flex flex-wrap gap-2">
              <UButton color="neutral" variant="soft" size="sm" @click="loadSampleJson">
                範例 JSON
              </UButton>
              <UButton
                color="neutral"
                variant="soft"
                size="sm"
                :disabled="!cardJsonContent"
                @click="loadReadJson"
              >
                載入剛讀到的 JSON
              </UButton>
            </div>

            <textarea
              v-model="jsonEditor"
              class="erp-textarea"
              spellcheck="false"
              placeholder='{&#10;  "documentNo": "MO-20260309-001",&#10;  "itemCode": "FG-1001"&#10;}'
            />

            <p v-if="editorError" class="text-rose-600">{{ editorError }}</p>
            <p v-else class="text-slate-500">限制：單層物件、最多 8 個欄位、不可放敏感資料。</p>

            <div class="grid gap-3 sm:grid-cols-[1fr_auto] sm:items-start">
              <div class="rounded-xl border border-slate-200 bg-slate-50 p-4">
                <p><strong>寫入狀態</strong>：{{ writeStatusLabel }}</p>
                <p><strong>Payload type</strong>：{{ lastWrite?.payloadType ?? "n/a" }}</p>
                <p><strong>NDEF bytes</strong>：{{ lastWrite?.ndefBytes ?? "n/a" }}</p>
              </div>
              <UButton
                :loading="isWriting"
                color="warning"
                size="lg"
                class="sm:min-w-40 sm:justify-center"
                :disabled="!canWrite"
                @click="handleWriteCard"
              >
                {{ isWriting ? "等待靠卡..." : "開始寫入" }}
              </UButton>
            </div>
          </div>
        </UCard>
      </section>

      <section class="erp-panel rounded-2xl border border-slate-200/80 p-5">
        <p class="eyebrow text-xs text-slate-500">ERP Output</p>
        <p class="mt-2 text-sm leading-6 text-slate-600">
          讀完卡後，可直接把這個物件送回 ERP：保留 UID 與卡片上的 JSON 即可。
        </p>
        <pre class="erp-code mt-4">{{ erpResultPreview }}</pre>
      </section>

      <div class="space-y-2">
        <details class="text-sm text-slate-500">
          <summary class="cursor-pointer select-none hover:text-slate-700">暫時停用 / 重新啟用 Connector</summary>
          <div class="mt-3 space-y-3 rounded-xl border border-slate-200 bg-white p-4 text-slate-600">
            <div>
              <p class="font-semibold text-slate-700">macOS</p>
              <ul class="mt-1 list-inside list-disc space-y-0.5">
                <li>停用：<code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">launchctl unload ~/Library/LaunchAgents/com.webnfcbridge.connector.plist</code></li>
                <li>啟用：<code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">launchctl load ~/Library/LaunchAgents/com.webnfcbridge.connector.plist</code></li>
              </ul>
            </div>
            <div>
              <p class="font-semibold text-slate-700">Windows（以系統管理員身分開啟終端機）</p>
              <ul class="mt-1 list-inside list-disc space-y-0.5">
                <li>停用：<code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">taskkill /IM nfc-connector.exe /F</code> 並 <code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">reg delete HKLM\Software\Microsoft\Windows\CurrentVersion\Run /v NFCToolConnector /f</code></li>
                <li>啟用：<code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">reg add HKLM\Software\Microsoft\Windows\CurrentVersion\Run /v NFCToolConnector /d "\"%ProgramFiles%\Web NFC Bridge Connector\nfc-connector.exe\" --watchdog"</code> 並 <code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">Start-Process "%ProgramFiles%\Web NFC Bridge Connector\nfc-connector.exe" --watchdog</code></li>
              </ul>
            </div>
            <div>
              <p class="font-semibold text-slate-700">Linux</p>
              <ul class="mt-1 list-inside list-disc space-y-0.5">
                <li>停用：<code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">sudo systemctl stop web-nfc-bridge-connector && sudo systemctl disable web-nfc-bridge-connector</code></li>
                <li>啟用：<code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">sudo systemctl enable web-nfc-bridge-connector && sudo systemctl start web-nfc-bridge-connector</code></li>
              </ul>
            </div>
          </div>
        </details>

        <details class="text-sm text-slate-500">
          <summary class="cursor-pointer select-none hover:text-slate-700">完全移除 Connector</summary>
          <div class="mt-3 space-y-3 rounded-xl border border-slate-200 bg-white p-4 text-slate-600">
            <div>
              <p class="font-semibold text-slate-700">macOS</p>
              <ol class="mt-1 list-inside list-decimal space-y-0.5">
                <li>終端機執行 <code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">launchctl unload ~/Library/LaunchAgents/com.webnfcbridge.connector.plist</code></li>
                <li>刪除檔案 <code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">rm ~/Library/LaunchAgents/com.webnfcbridge.connector.plist</code></li>
                <li>刪除程式 <code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">sudo rm /usr/local/bin/nfc-connector</code></li>
                <li>清除安裝紀錄 <code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">sudo pkgutil --forget com.webnfcbridge.connector</code></li>
              </ol>
            </div>
            <div>
              <p class="font-semibold text-slate-700">Windows</p>
              <ol class="mt-1 list-inside list-decimal space-y-0.5">
                <li>開啟「設定  &gt; 應用程式 &gt; 已安裝的應用程式」</li>
                <li>搜尋「Web NFC Bridge Connector」，點擊「解除安裝」</li>
              </ol>
            </div>
            <div>
              <p class="font-semibold text-slate-700">Linux</p>
              <ol class="mt-1 list-inside list-decimal space-y-0.5">
                <li>執行 <code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">sudo systemctl stop web-nfc-bridge-connector && sudo systemctl disable web-nfc-bridge-connector</code></li>
                <li>執行 <code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">sudo dpkg -r web-nfc-bridge-connector</code> 或 <code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">sudo apt remove web-nfc-bridge-connector</code></li>
              </ol>
            </div>
          </div>
        </details>
      </div>
    </div>
  </main>
</template>
