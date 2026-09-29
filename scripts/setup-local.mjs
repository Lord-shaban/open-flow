import { randomBytes } from "node:crypto";
import { existsSync, readFileSync, writeFileSync } from "node:fs";
if (existsSync(".env")) {
  console.log(".env already exists; kept your existing configuration.");
  process.exit(0);
}
const values = {
  OPEN_FLOW_OWNER_TOKEN: randomBytes(32).toString("base64url"),
  OPEN_FLOW_ENCRYPTION_KEYS: `'{"1":"${randomBytes(32).toString("base64")}"}'`,
  OPEN_FLOW_ENCRYPTION_VERSION: "1",
  OPEN_FLOW_OWNER_ID: "00000000-0000-4000-8000-000000000001",
  OPEN_FLOW_S3_ACCESS_KEY: "open-flow-local",
  OPEN_FLOW_S3_SECRET_KEY: randomBytes(32).toString("base64url"),
};
let template = readFileSync(".env.example", "utf8");
for (const [key, value] of Object.entries(values)) {
  const pattern = new RegExp(`^${key}=.*$`, "m");
  if (pattern.test(template))
    template = template.replace(pattern, `${key}=${value}`);
  else template += `\n${key}=${value}\n`;
}
writeFileSync(".env", template, { flag: "wx", mode: 0o600 });
console.log(
  "Created .env with local owner access and encryption keys. Copy OPEN_FLOW_OWNER_TOKEN from this file to unlock the workspace. No provider account, card, or paid service was created.",
);
