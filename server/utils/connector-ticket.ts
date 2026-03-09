type ConnectorTicketScope = "all" | "read" | "write" | "events";

const encoder = new TextEncoder();

function bytesToBase64Url(bytes: Uint8Array) {
  let binary = "";
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }

  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

async function signHmac(payload: string, secret: string) {
  const key = await crypto.subtle.importKey(
    "raw",
    encoder.encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );

  const signature = await crypto.subtle.sign("HMAC", key, encoder.encode(payload));
  return bytesToBase64Url(new Uint8Array(signature));
}

export async function signConnectorTicket(
  origin: string,
  scope: ConnectorTicketScope,
  secret: string,
) {
  const payload = {
    origin,
    scope,
    exp: Math.floor(Date.now() / 1000) + 60,
  };

  const encodedPayload = bytesToBase64Url(encoder.encode(JSON.stringify(payload)));
  const signature = await signHmac(encodedPayload, secret);

  return {
    token: `${encodedPayload}.${signature}`,
    expiresAt: payload.exp,
    scope,
  };
}
