type PlatformKey = "macos" | "windows-x64" | "windows-arm64" | "linux-x64" | "unknown";

type DownloadEntry = {
  platformKey: PlatformKey;
  label: string;
  fileName: string;
  url: string;
};

function detectPlatform(): PlatformKey {
  if (import.meta.server) return "unknown";

  const ua = navigator.userAgent;
  if (/Macintosh|Mac OS X/i.test(ua)) return "macos";
  if (/Windows/i.test(ua) && /ARM64|WOA/i.test(ua)) return "windows-arm64";
  if (/Windows/i.test(ua)) return "windows-x64";
  if (/Linux/i.test(ua) && !/Android/i.test(ua)) return "linux-x64";
  return "unknown";
}

export function useConnectorDownload() {
  const config = useRuntimeConfig();
  const platform = useState<PlatformKey>("detected-platform", () => "unknown");

  onMounted(() => {
    platform.value = detectPlatform();
  });

  const version = config.public.connectorVersion as string;
  const repo = config.public.connectorRepo as string;

  const allDownloads = computed<DownloadEntry[]>(() => {
    const v = version;
    const base = `https://github.com/${repo}/releases/download/v${v}`;
    return [
      { platformKey: "macos", label: "macOS (Apple Silicon)", fileName: `connector-macos-arm64-${v}.pkg`, url: `${base}/connector-macos-arm64-${v}.pkg` },
      { platformKey: "windows-x64", label: "Windows x64", fileName: `connector-windows-x64-${v}.msi`, url: `${base}/connector-windows-x64-${v}.msi` },
      { platformKey: "windows-arm64", label: "Windows ARM64", fileName: `connector-windows-arm64-${v}.msi`, url: `${base}/connector-windows-arm64-${v}.msi` },
      { platformKey: "linux-x64", label: "Linux x64 (.deb)", fileName: `connector-linux-x64-${v}.deb`, url: `${base}/connector-linux-x64-${v}.deb` },
    ];
  });

  const recommended = computed<DownloadEntry>(() =>
    allDownloads.value.find((d) => d.platformKey === platform.value) ?? allDownloads.value[0]!,
  );

  const others = computed(() =>
    allDownloads.value.filter((d) => d.platformKey !== recommended.value!.platformKey),
  );

  return { platform, recommended, others, allDownloads, version };
}
