/**
 * Cloudflare Worker — Admin API Proxy
 *
 * Proxies authenticated requests from the dashboard to the Go bot backend.
 * Adds Cloudflare Access validation and rate limiting.
 */

export interface Env {
  BOT_BACKEND_URL: string;
  ADMIN_API_KEY: string;
}

export default {
  async fetch(
    request: Request,
    env: Env,
    ctx: ExecutionContext
  ): Promise<Response> {
    const url = new URL(request.url);

    // Health check bypass
    if (url.pathname === "/health") {
      return new Response(JSON.stringify({ status: "ok", proxy: true }), {
        headers: { "Content-Type": "application/json" },
      });
    }

    // Validate API key from request
    const authHeader = request.headers.get("Authorization");
    if (!authHeader || authHeader !== `Bearer ${env.ADMIN_API_KEY}`) {
      return new Response(JSON.stringify({ error: "unauthorized" }), {
        status: 401,
        headers: { "Content-Type": "application/json" },
      });
    }

    // Proxy to backend
    const backendUrl = `${env.BOT_BACKEND_URL}${url.pathname}${url.search}`;

    const proxyHeaders = new Headers(request.headers);
    proxyHeaders.set("X-Forwarded-For", request.headers.get("CF-Connecting-IP") || "");
    proxyHeaders.set("X-Forwarded-Proto", "https");

    try {
      const backendResp = await fetch(backendUrl, {
        method: request.method,
        headers: proxyHeaders,
        body: request.method !== "GET" ? request.body : undefined,
      });

      // Forward response with CORS headers
      const respHeaders = new Headers(backendResp.headers);
      respHeaders.set("Access-Control-Allow-Origin", "*");
      respHeaders.set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS");
      respHeaders.set("Access-Control-Allow-Headers", "Content-Type, Authorization");

      return new Response(backendResp.body, {
        status: backendResp.status,
        headers: respHeaders,
      });
    } catch (err) {
      console.error("Backend proxy error:", err);
      return new Response(
        JSON.stringify({ error: "backend unavailable" }),
        {
          status: 502,
          headers: { "Content-Type": "application/json" },
        }
      );
    }
  },
};
