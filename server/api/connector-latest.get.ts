import { defineCachedEventHandler } from "nitropack/runtime";

export default defineCachedEventHandler(
  async (event) => {
    const config = useRuntimeConfig(event);
    const repo = config.public.connectorRepo as string;

    const release = await $fetch<{ tag_name: string }>(
      `https://api.github.com/repos/${repo}/releases/latest`,
      {
        headers: { Accept: "application/vnd.github+json" },
      },
    );

    const version = release.tag_name.replace(/^v/, "");

    return { version };
  },
  { maxAge: 300, name: "connector-latest" },
);
