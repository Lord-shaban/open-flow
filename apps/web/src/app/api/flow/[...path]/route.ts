import {
  authenticated,
  limitedBody,
  ownerToken,
  sameOrigin,
} from "@/lib/session";
export const dynamic = "force-dynamic";
const allowed =
  /^(credentials(?:\/[a-f0-9-]{36}(?:\/test)?)?|models|generations(?:\/[a-f0-9-]{36})?|artifacts\/[a-f0-9-]{36}\/(download|content))$/;
type Context = { params: Promise<{ path: string[] }> };
async function proxy(request: Request, context: Context) {
  if (!(await authenticated()))
    return Response.json(
      { error: { message: "Unlock your workspace first" } },
      { status: 401 },
    );
  if (request.method !== "GET" && !sameOrigin(request))
    return Response.json(
      { error: { message: "Invalid request origin" } },
      { status: 403 },
    );
  const path = (await context.params).path.join("/");
  if (!allowed.test(path))
    return Response.json(
      { error: { message: "Route not found" } },
      { status: 404 },
    );
  const base = process.env.OPEN_FLOW_API_URL ?? "http://127.0.0.1:8080";
  const url = new URL(`/v1/${path}`, base);
  url.search = new URL(request.url).search;
  const headers: Record<string, string> = {
    Authorization: `Bearer ${ownerToken()}`,
    "Content-Type": "application/json",
  };
  const key = request.headers.get("idempotency-key");
  if (key) headers["Idempotency-Key"] = key;
  try {
    const body =
      request.method === "GET" || request.method === "DELETE"
        ? undefined
        : await limitedBody(request);
    const response = await fetch(url, {
      method: request.method,
      headers,
      body,
      cache: "no-store",
      signal: AbortSignal.timeout(12000),
      redirect: "error",
    });
    return new Response(response.body, {
      status: response.status,
      headers: {
        "Content-Type":
          response.headers.get("content-type") ?? "application/json",
        "Cache-Control": "private, no-store",
        "X-Content-Type-Options": "nosniff",
        ...(response.headers.has("content-disposition")
          ? {
              "Content-Disposition": response.headers.get(
                "content-disposition",
              )!,
            }
          : {}),
      },
    });
  } catch {
    return Response.json(
      {
        error: {
          message:
            "The local API is unavailable. Start the stack and try again.",
        },
      },
      { status: 503 },
    );
  }
}
export const GET = proxy;
export const POST = proxy;
export const PUT = proxy;
export const DELETE = proxy;
