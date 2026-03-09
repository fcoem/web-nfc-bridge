import installersManifest from "../../public/downloads/manifest.json";

type InstallerStatus = "available" | "planned";

type InstallerEntry = {
  platform: string;
  label: string;
  href: string | null;
  fileName: string | null;
  status: InstallerStatus;
};

function normalizeInstallers(value: unknown): InstallerEntry[] {
  if (!value || typeof value !== "object" || !("installers" in value)) {
    return [];
  }

  const installers = (value as { installers?: unknown }).installers;
  if (!Array.isArray(installers)) {
    return [];
  }

  return installers.flatMap((entry) => {
    if (!entry || typeof entry !== "object") {
      return [];
    }

    const candidate = entry as Partial<InstallerEntry>;
    if (
      typeof candidate.platform !== "string" ||
      typeof candidate.label !== "string" ||
      (candidate.href !== null && typeof candidate.href !== "string") ||
      (candidate.fileName !== null && typeof candidate.fileName !== "string") ||
      (candidate.status !== "available" && candidate.status !== "planned")
    ) {
      return [];
    }

    return [candidate as InstallerEntry];
  });
}

export default defineEventHandler((event) => {
  const config = useRuntimeConfig(event);
  const installers = normalizeInstallers(installersManifest);

  return {
    connectorBaseUrl: config.public.connectorBaseUrl,
    installers,
    browserPolicy: {
      strategy: "HTTPS website with localhost connector",
      note: "Browser-native Web NFC and WebUSB are not primary support paths.",
    },
    businessRules: {
      read: {
        enabled: true,
      },
      write: {
        enabled: true,
        profileRequired: true,
        defaultProfile: "ndef-v1",
        mediaType: "application/json",
        maxPayloadBytes: 256,
        allowedTypes: ["nfc-tool/demo", "nfc-tool/ref"],
      },
    },
  };
});
