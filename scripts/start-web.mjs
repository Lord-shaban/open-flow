import { cpSync, existsSync, mkdirSync } from "node:fs";
import { join, resolve } from "node:path";
import { spawn } from "node:child_process";

const web = resolve(import.meta.dirname, "../apps/web");
const standalone = join(web, ".next/standalone/apps/web");
const server = join(standalone, "server.js");
if (!existsSync(server))
  throw new Error("Build the web app with pnpm build first.");
mkdirSync(join(standalone, ".next"), { recursive: true });
cpSync(join(web, ".next/static"), join(standalone, ".next/static"), {
  recursive: true,
});
if (existsSync(join(web, "public")))
  cpSync(join(web, "public"), join(standalone, "public"), { recursive: true });
const child = spawn(process.execPath, [server], {
  stdio: "inherit",
  env: {
    ...process.env,
    HOSTNAME: "127.0.0.1",
    PORT: process.env.PORT ?? "3000",
  },
});
for (const signal of ["SIGINT", "SIGTERM"])
  process.on(signal, () => child.kill(signal));
child.on("error", (error) => {
  console.error(error);
  process.exitCode = 1;
});
child.on("exit", (code) => {
  process.exitCode = code ?? 1;
});
