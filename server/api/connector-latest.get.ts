let cached: { version: string; fetchedAt: number } | null = null;
const CACHE_TTL_MS = 5 * 60 * 1000;

export default defineEventHandler(async (event) => {
  if (cached && Date.now() - cached.fetchedAt < CACHE_TTL_MS) {
    return { version: cached.version };
  }

  const config = useRuntimeConfig(event);
  const repo = config.public.connectorRepo as string;

  const response = await fetch(
    `https://api.github.com/repos/${repo}/releases/latest`,
    {
      headers: {
        Accept: "application/vnd.github+json",
        "User-Agent": "web-nfc-bridge",
      },
    },
  );

  if (!response.ok) {
    throw createError({ statusCode: 502, statusMessage: "Failed to fetch latest release" });
  }

  const release = (await response.json()) as { tag_name: string };
  const version = release.tag_name.replace(/^v/, "");

  cached = { version, fetchedAt: Date.now() };

  return { version };
});
