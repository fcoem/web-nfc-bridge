export default defineEventHandler(async (event) => {
  const config = useRuntimeConfig(event);
  const body = await readBody<{ scope?: "all" | "read" | "write" | "events" }>(event);
  const origin = getRequestHeader(event, "origin") ?? config.public.siteOrigin;

  if (!origin) {
    throw createError({
      statusCode: 400,
      message: "Missing request origin",
    });
  }

  const scope = body?.scope ?? "all";
  return signConnectorTicket(origin, scope, config.connectorSharedSecret);
});
