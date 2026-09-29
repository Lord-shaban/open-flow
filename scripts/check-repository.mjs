import { readFileSync, existsSync, readdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { parse } from "yaml";

const required = [
  "README.md",
  "LICENSE",
  "CONTRIBUTING.md",
  "CODE_OF_CONDUCT.md",
  "SECURITY.md",
  "CHANGELOG.md",
  ".env.example",
  "docs/architecture.md",
  "docs/project-plan.md",
];
for (const file of required)
  if (!existsSync(file)) throw new Error("Missing " + file);
const backlog = JSON.parse(readFileSync("docs/backlog.json", "utf8"));
const ids = new Set(backlog.issues.map((issue) => issue.id));
if (ids.size !== backlog.issues.length)
  throw new Error("Duplicate backlog IDs");
const milestones = new Set(
  backlog.milestones.map((milestone) => milestone.title),
);
const labels = new Set(backlog.labels.map((label) => label.name));
for (const issue of backlog.issues) {
  if (
    !milestones.has(issue.milestone) ||
    !issue.context ||
    !issue.acceptance.length
  )
    throw new Error("Incomplete issue " + issue.id);
  for (const dep of issue.dependencies)
    if (!ids.has(dep) || dep === issue.id)
      throw new Error("Invalid dependency " + dep);
  for (const label of issue.labels)
    if (!labels.has(label)) throw new Error("Unknown label " + label);
}
const visited = new Set();
const visiting = new Set();
function visit(id) {
  if (visiting.has(id)) throw new Error("Cyclic issue dependency " + id);
  if (visited.has(id)) return;
  visiting.add(id);
  const issue = backlog.issues.find((item) => item.id === id);
  for (const dep of issue.dependencies) visit(dep);
  visiting.delete(id);
  visited.add(id);
}
for (const id of ids) visit(id);
function walk(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const file = directory + "/" + entry.name;
    return entry.isDirectory()
      ? walk(file)
      : file.endsWith(".md")
        ? [file]
        : [];
  });
}
for (const file of [
  ...required.filter((file) => file.endsWith(".md")),
  ...walk("docs"),
]) {
  const content = readFileSync(file, "utf8");
  for (const match of content.matchAll(/\[[^\]]*\]\(([^)]+)\)/g)) {
    const target = match[1].split("#")[0];
    if (!target || /^(https?:|mailto:|\/)/.test(target)) continue;
    if (!existsSync(resolve(dirname(file), target)))
      throw new Error("Broken local link in " + file + ": " + target);
  }
}
const compose = parse(readFileSync("infra/compose.yaml", "utf8"));
for (const name of ["postgres", "kafka", "temporal", "temporal-ui", "worker"])
  if (!compose.services[name])
    throw new Error("Missing infrastructure service " + name);
for (const service of Object.values(compose.services)) {
  if (service.image?.endsWith(":latest")) throw new Error("Unpinned image");
  for (const port of service.ports ?? [])
    if (!port.startsWith("127.0.0.1:"))
      throw new Error("Development host port is not loopback-bound");
}
const api = parse(readFileSync("api/openapi.yaml", "utf8"));
if (api.openapi !== "3.1.0" || !api.paths["/healthz"] || !api.paths["/v1"])
  throw new Error("Invalid foundation OpenAPI");
console.log(
  "Repository checks passed: governance, backlog dependencies, local links, infrastructure and API.",
);
