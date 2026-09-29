import {
  authenticated,
  configured,
  equal,
  limitedBody,
  login,
  logout,
  ownerToken,
  sameOrigin,
} from "@/lib/session";
export const dynamic = "force-dynamic";
export async function GET() {
  return Response.json(
    { authenticated: await authenticated(), configured: configured() },
    { headers: { "Cache-Control": "no-store" } },
  );
}
export async function POST(request: Request) {
  if (!sameOrigin(request))
    return Response.json(
      { error: { message: "Invalid request origin" } },
      { status: 403 },
    );
  if (!configured())
    return Response.json(
      {
        error: {
          message: "Set up the local stack first. See the setup guide.",
        },
      },
      { status: 503 },
    );
  try {
    const data = JSON.parse(await limitedBody(request));
    if (typeof data.token !== "string" || !equal(data.token, ownerToken()))
      return Response.json(
        { error: { message: "Access token does not match" } },
        { status: 401 },
      );
    await login();
    return Response.json({ authenticated: true });
  } catch {
    return Response.json(
      { error: { message: "Invalid access request" } },
      { status: 400 },
    );
  }
}
export async function DELETE(request: Request) {
  if (!sameOrigin(request))
    return Response.json(
      { error: { message: "Invalid request origin" } },
      { status: 403 },
    );
  await logout();
  return Response.json({ authenticated: false });
}
