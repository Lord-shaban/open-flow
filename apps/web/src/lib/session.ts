import "server-only";
import { createHmac, randomBytes, timingSafeEqual } from "node:crypto";
import { cookies } from "next/headers";
const cookieName = "open-flow-session";
export function ownerToken() {
  return process.env.OPEN_FLOW_OWNER_TOKEN ?? "";
}
export function configured() {
  return ownerToken().length >= 32;
}
function signature(value: string) {
  return createHmac("sha256", ownerToken())
    .update(`session:v1:${value}`)
    .digest("hex");
}
export function equal(a: string, b: string) {
  const aa = Buffer.from(a),
    bb = Buffer.from(b);
  return aa.length === bb.length && timingSafeEqual(aa, bb);
}
export async function authenticated() {
  if (!configured()) return false;
  const value = (await cookies()).get(cookieName)?.value ?? "";
  const [expiry, nonce, sig] = value.split(".");
  return (
    !!nonce &&
    /^\d+$/.test(expiry ?? "") &&
    Number(expiry) > Date.now() &&
    Number(expiry) <= Date.now() + 8 * 60 * 60 * 1000 &&
    equal(sig ?? "", signature(`${expiry}.${nonce}`))
  );
}
export async function login() {
  const expires = Date.now() + 8 * 60 * 60 * 1000;
  const value = `${expires}.${randomBytes(24).toString("hex")}`;
  (await cookies()).set(cookieName, `${value}.${signature(value)}`, {
    httpOnly: true,
    sameSite: "strict",
    secure: process.env.OPEN_FLOW_SECURE_COOKIE === "true",
    maxAge: 8 * 60 * 60,
    path: "/",
  });
}
export async function logout() {
  (await cookies()).delete(cookieName);
}
export function sameOrigin(request: Request) {
  // Next may normalize the URL hostname to localhost. Host still describes
  // the browser's target authority, including its port; ignore forwarded hosts.
  const origin = request.headers.get("origin"),
    host = request.headers.get("host");
  if (!origin || !host) return false;
  try {
    const parsed = new URL(origin);
    return (
      parsed.origin === origin &&
      parsed.host === host &&
      parsed.protocol === new URL(request.url).protocol
    );
  } catch {
    return false;
  }
}
export async function limitedBody(request: Request) {
  const reader = request.body?.getReader();
  if (!reader) return "";
  const chunks: Uint8Array[] = [];
  let size = 0;
  while (true) {
    const { value, done } = await reader.read();
    if (done) break;
    size += value.byteLength;
    if (size > 16 * 1024) {
      await reader.cancel();
      throw new Error("body too large");
    }
    chunks.push(value);
  }
  return Buffer.concat(chunks).toString("utf8");
}
