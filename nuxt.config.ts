export default defineNuxtConfig({
  compatibilityDate: "2026-03-09",
  devtools: { enabled: true },
  modules: ["@nuxt/ui"],
  css: ["~/assets/css/main.css"],
  nitro: {
    cloudflare: {
      deployConfig: true,
    },
  },
  runtimeConfig: {
    connectorSharedSecret: "development-shared-secret",
    public: {
      appName: "Web NFC Bridge",
      connectorBaseUrl: "http://127.0.0.1:42619",
      siteOrigin: "http://localhost:3000",
    },
  },
  ui: {
    theme: {
      colors: ["primary", "secondary", "success", "info", "warning", "error", "neutral"],
      transitions: true,
      defaultVariants: {
        color: "primary",
        size: "md",
      },
    },
  },
  app: {
    head: {
      title: "Web NFC Bridge",
      meta: [
        { charset: "utf-8" },
        { name: "viewport", content: "width=device-width, initial-scale=1" },
        {
          name: "description",
          content:
            "HTTPS-first NFC bridge console for ACR1252U-M1 with Nuxt UI and localhost connector detection.",
        },
      ],
    },
  },
  typescript: {
    strict: true,
    typeCheck: true,
  },
});
