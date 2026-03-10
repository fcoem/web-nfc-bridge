type PlatformKey = "macos" | "windows-x64" | "windows-arm64" | "linux-x64" | "unknown";

type DownloadEntry = {
  platformKey: PlatformKey;
  label: string;
  fileName: string;
  url: string;
};

function detectPlatformFromUA(): PlatformKey {
  const ua = navigator.userAgent;
  if (/Macintosh|Mac OS X/i.test(ua)) return "macos";
  if (/Windows/i.test(ua) && /ARM64|WOA/i.test(ua)) return "windows-arm64";
  if (/Windows/i.test(ua)) return "windows-x64";
  if (/Linux/i.test(ua) && !/Android/i.test(ua)) return "linux-x64";
  return "unknown";
}

async function detectPlatform(): Promise<PlatformKey> {
  if (import.meta.server) return "unknown";

  const uaResult = detectPlatformFromUA();

  // UA-based detection may misidentify Windows ARM as x64,
  // use User-Agent Client Hints API for accurate architecture detection
  if (uaResult === "windows-x64" && navigator.userAgentData) {
    try {
      const hints = await navigator.userAgentData.getHighEntropyValues(["architecture"]);
      if (hints.architecture === "arm") return "windows-arm64";
    } catch {
      // Fall through to UA-based result
    }
  }

  return uaResult;
}

function buildDownloads(version: string, repo: string): DownloadEntry[] {
  const base = `https://github.com/${repo}/releases/download/v${version}`;
  return [
    { platformKey: "macos", label: "macOS (Apple Silicon)", fileName: `connector-macos-arm64-${version}.pkg`, url: `${base}/connector-macos-arm64-${version}.pkg` },
    { platformKey: "windows-x64", label: "Windows x64", fileName: `connector-windows-x64-${version}.msi`, url: `${base}/connector-windows-x64-${version}.msi` },
    { platformKey: "windows-arm64", label: "Windows ARM64", fileName: `connector-windows-arm64-${version}.msi`, url: `${base}/connector-windows-arm64-${version}.msi` },
    { platformKey: "linux-x64", label: "Linux x64 (.deb)", fileName: `connector-linux-x64-${version}.deb`, url: `${base}/connector-linux-x64-${version}.deb` },
  ];
}

export function useConnectorDownload() {
  const config = useRuntimeConfig();
  const platform = useState<PlatformKey>("detected-platform", () => "unknown");

  const repo = config.public.connectorRepo as string;
  const fallbackVersion = config.public.connectorVersion as string;

  const { data: latestData } = useAsyncData("connector-latest-version", () =>
    $fetch<{ version: string }>("/api/connector-latest").catch(() => ({ version: fallbackVersion })),
  );

  onMounted(async () => {
    platform.value = await detectPlatform();
  });

  const resolvedVersion = computed(() => latestData.value?.version ?? fallbackVersion);

  const allDownloads = computed<DownloadEntry[]>(() =>
    buildDownloads(resolvedVersion.value, repo),
  );

  const recommended = computed<DownloadEntry>(() =>
    allDownloads.value.find((d) => d.platformKey === platform.value) ?? allDownloads.value[0]!,
  );

  const others = computed(() =>
    allDownloads.value.filter((d) => d.platformKey !== recommended.value!.platformKey),
  );

  return { platform, recommended, others, allDownloads, version: resolvedVersion };
}
