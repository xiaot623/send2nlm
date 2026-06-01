#!/usr/bin/env node

const fs = require("fs");
const path = require("path");
const { spawn } = require("child_process");

const root = path.resolve(__dirname, "..");
const devAssets = path.join(root, "dev_assets");
const statePath = path.join(devAssets, "mock_notebooklm.json");
const seedPath = path.join(root, "assets", "mock_notebooklm.seed.json");
const daemonDir = path.join(root, "daemon");

fs.mkdirSync(devAssets, { recursive: true });
if (!fs.existsSync(statePath)) {
  fs.copyFileSync(seedPath, statePath);
  console.log(`[dev:mock] initialized ${path.relative(root, statePath)}`);
}

const args = [
  "run",
  ".",
  "daemon",
  "--dev",
  "--mock-nlm",
  statePath,
  ...process.argv.slice(2),
];

const child = spawn("go", args, {
  cwd: daemonDir,
  stdio: "inherit",
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
  }
  process.exit(code ?? 0);
});
