export default defineEventHandler(async (event) => {
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

  return { version };
});
