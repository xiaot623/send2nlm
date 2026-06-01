#!/usr/bin/env node

const fs = require("fs");
const path = require("path");
const { spawn } = require("child_process");

const root = path.resolve(__dirname, "..");
const daemonDir = path.join(root, "daemon");
const e2eRoot = path.join(root, "e2e");
const seedPath = path.join(root, "assets", "mock_notebooklm.seed.json");
const mockArtifactDir = path.join(root, "assets", "mock-artifacts");
const defaultPort = Number(process.env.SEND2NLM_QA_PORT || 18925);
const tasks = ["audio_overview", "slide_deck", "video_overview"];

const url = parseURLArg(process.argv.slice(2));
if (!url) {
  console.error("Usage: npm run qa -- <webpage-url>");
  console.error("   or: npm run qa -- --url <webpage-url>");
  process.exit(1);
}

const runID = new Date().toISOString().replace(/[:.]/g, "-");
const runDir = path.join(e2eRoot, runID);
const responseDir = path.join(runDir, "responses");
const receiverDir = path.join(runDir, "receiver");
const producerDir = path.join(runDir, "producer");
const devAssets = path.join(runDir, "dev_assets");
const statePath = path.join(devAssets, "mock_notebooklm.json");
const baseURL = `http://127.0.0.1:${defaultPort}`;
const logPath = path.join(runDir, "qa.log");
const daemonLogPath = path.join(runDir, "daemon.log");

fs.mkdirSync(responseDir, { recursive: true });
fs.mkdirSync(receiverDir, { recursive: true });
fs.mkdirSync(producerDir, { recursive: true });

const rootDevAssets = path.join(root, "dev_assets");
if (fs.existsSync(rootDevAssets)) {
  fs.cpSync(rootDevAssets, devAssets, {
    recursive: true,
    filter: (src) => {
      const isSqlite = src.endsWith(".db") || src.endsWith("-shm") || src.endsWith("-wal");
      return !isSqlite;
    },
  });
} else {
  fs.mkdirSync(devAssets, { recursive: true });
}

fs.copyFileSync(seedPath, statePath);
const configSource = seedRuntimeConfig(devAssets);

function parseURLArg(args) {
  if (args.length === 1 && !args[0].startsWith("-")) {
    return args[0];
  }
  if (args.length === 2 && args[0] === "--url") {
    return args[1];
  }
  return "";
}

function seedRuntimeConfig(targetDir) {
  const candidates = [
    process.env.SEND2NLM_QA_CONFIG,
    path.join(root, "dev_assets", "config.json"),
    path.join(process.env.HOME || "", ".send2nlm", "config.json"),
  ].filter(Boolean);

  for (const candidate of candidates) {
    if (!fs.existsSync(candidate)) {
      continue;
    }
    const config = JSON.parse(fs.readFileSync(candidate, "utf8"));
    config.receivers = config.receivers || {};
    config.receivers.download = {
      ...(config.receivers.download || {}),
      enabled: true,
    };
    fs.writeFileSync(path.join(targetDir, "config.json"), `${JSON.stringify(config, null, 2)}\n`);
    return candidate;
  }
  return "";
}

function log(message, data) {
  const line = `[${new Date().toISOString()}] ${message}`;
  console.log(line);
  fs.appendFileSync(logPath, `${line}\n`);
  if (data !== undefined) {
    const text = typeof data === "string" ? data : JSON.stringify(data, null, 2);
    console.log(text);
    fs.appendFileSync(logPath, `${text}\n`);
  }
}

function writeJSON(name, payload) {
  fs.writeFileSync(path.join(responseDir, `${name}.json`), `${JSON.stringify(payload, null, 2)}\n`);
}

async function request(stepName, pathname, options = {}) {
  log(`${stepName}: ${options.method || "GET"} ${pathname}`);
  const response = await fetch(`${baseURL}${pathname}`, {
    headers: {
      "Content-Type": "application/json",
      ...(options.headers || {}),
    },
    ...options,
  });
  let payload = null;
  const text = await response.text();
  if (text) {
    try {
      payload = JSON.parse(text);
    } catch (_error) {
      payload = text;
    }
  }
  writeJSON(stepName, {
    status: response.status,
    ok: response.ok,
    payload,
  });
  if (!response.ok) {
    throw new Error(`${stepName} failed (${response.status}): ${text}`);
  }
  return payload;
}

async function requestWithRetry(stepName, pathname, options = {}) {
  const attempts = options.attempts || 5;
  const retryDelayMs = options.retryDelayMs || 500;
  const requestOptions = { ...options };
  delete requestOptions.attempts;
  delete requestOptions.retryDelayMs;

  let lastError = null;
  for (let attempt = 1; attempt <= attempts; attempt++) {
    try {
      return await request(stepName, pathname, requestOptions);
    } catch (error) {
      lastError = error;
      if (!String(error.message).includes("database is locked") || attempt === attempts) {
        throw error;
      }
      log(`${stepName}: database is locked, retrying (${attempt}/${attempts})`);
      await new Promise((resolve) => setTimeout(resolve, retryDelayMs));
    }
  }
  throw lastError;
}

async function waitForHealth(child) {
  const started = Date.now();
  while (Date.now() - started < 30000) {
    if (child.exitCode !== null) {
      throw new Error(`daemon exited before health check finished with code ${child.exitCode}`);
    }
    try {
      return await request("00-health", "/health");
    } catch (_error) {
      await new Promise((resolve) => setTimeout(resolve, 300));
    }
  }
  throw new Error("daemon did not become healthy within 30s");
}

async function pollJob(jobID) {
  const started = Date.now();
  let lastStatus = "";
  let attempt = 0;
  while (Date.now() - started < 120000) {
    attempt++;
    const job = await requestWithRetry(`06-job-${String(attempt).padStart(3, "0")}`, `/jobs/${jobID}`);
    if (job.status !== lastStatus) {
      lastStatus = job.status;
      log(`job ${jobID} status: ${job.status}`);
    }
    if (job.status === "done") {
      return job;
    }
    if (job.status === "failed") {
      throw new Error(`job failed: ${job.error || "unknown error"}`);
    }
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }
  throw new Error(`job ${jobID} did not finish within 120s`);
}

function copyIfExists(src, dstDir) {
  if (!src || !fs.existsSync(src)) {
    return null;
  }
  fs.mkdirSync(dstDir, { recursive: true });
  const dst = path.join(dstDir, path.basename(src));
  fs.copyFileSync(src, dst);
  return dst;
}

async function main() {
  log(`QA run directory: ${runDir}`);
  log(`Target URL: ${url}`);
  if (configSource) {
    log(`Runtime config copied from: ${configSource}`);
  } else {
    log("Runtime config source not found; daemon will create default config");
  }

  const child = spawn("go", [
    "run",
    ".",
    "daemon",
    "--dev",
    "--config-dir",
    devAssets,
    "--port",
    String(defaultPort),
    "--mock-nlm",
    statePath,
    "--mock-nlm-artifacts",
    mockArtifactDir,
  ], {
    cwd: daemonDir,
    detached: true,
    stdio: ["ignore", "pipe", "pipe"],
    env: {
      ...process.env,
      SEND2NLM_DOWNLOAD_DIR: receiverDir,
      SEND2NLM_PRODUCER_OUTPUT_DIR: producerDir,
    },
  });

  child.stdout.on("data", (chunk) => fs.appendFileSync(daemonLogPath, chunk));
  child.stderr.on("data", (chunk) => fs.appendFileSync(daemonLogPath, chunk));

  try {
    await waitForHealth(child);

    const listed = await request("01-list-notebooks", "/notebooks?refresh=true");
    const firstNotebook = listed.notebooks?.[0];
    log(`listed notebooks: ${listed.notebooks?.length ?? 0}`);

    const created = await request("02-create-notebook", "/notebooks", {
      method: "POST",
      body: JSON.stringify({
        title: `QA ${new Date().toISOString()}`,
        emoji: "T",
      }),
    });
    const notebookID = created.id || firstNotebook?.id;
    log(`selected notebook: ${notebookID}`);

    const refreshed = await request("03-refresh-notebooks", "/notebooks?refresh=true");
    log(`refreshed notebooks: ${refreshed.notebooks?.length ?? 0}`);

    const upload = await request("04-send-webpage", `/notebooks/${encodeURIComponent(notebookID)}/upload`, {
      method: "POST",
      body: JSON.stringify({ url }),
    });
    log(`uploaded source: ${upload.source_id}`);

    const jobAccepted = await request("05-generate-resources", "/jobs", {
      method: "POST",
      body: JSON.stringify({
        notebook_id: notebookID,
        url,
        source_ids: [upload.source_id],
        tasks,
      }),
    });
    log(`job accepted: ${jobAccepted.job_id}`);

    const finalJob = await pollJob(jobAccepted.job_id);
    writeJSON("07-final-job", finalJob);
    failOnTelegramReceiverError();

    const copiedArtifacts = [];
    for (const result of Object.values(finalJob.task_results || {})) {
      const copied = copyIfExists(result.asset_path, path.join(runDir, "artifacts"));
      if (copied) copiedArtifacts.push(copied);
    }

    const receiverFiles = fs.existsSync(receiverDir) ? fs.readdirSync(receiverDir).map((name) => path.join(receiverDir, name)) : [];
    writeJSON("08-summary", {
      url,
      notebook_id: notebookID,
      source_id: upload.source_id,
      job_id: finalJob.job_id,
      status: finalJob.status,
      mock_state: statePath,
      copied_artifacts: copiedArtifacts,
      receiver_files: receiverFiles,
      producer_output: producerDir,
      responses: responseDir,
      logs: {
        qa: logPath,
        daemon: daemonLogPath,
      },
    });

    log("receiver files", receiverFiles);
    log("QA flow completed successfully");
  } finally {
    if (child.exitCode === null && child.signalCode === null) {
      try {
        process.kill(-child.pid, "SIGTERM");
      } catch (_error) {
        child.kill("SIGTERM");
      }
      await Promise.race([
        new Promise((resolve) => child.once("exit", resolve)),
        new Promise((resolve) => setTimeout(resolve, 3000)),
      ]);
    }
  }
}

function failOnTelegramReceiverError() {
  if (!fs.existsSync(daemonLogPath)) {
    return;
  }
  const daemonLog = fs.readFileSync(daemonLogPath, "utf8");
  const failed = daemonLog
    .split("\n")
    .filter((line) => line.includes("[receiver] telegram delivery failed"));
  if (failed.length > 0) {
    throw new Error(`telegram receiver failed; see ${daemonLogPath}\n${failed.join("\n")}`);
  }
}

main().catch((error) => {
  log(`QA flow failed: ${error.stack || error.message}`);
  process.exit(1);
});
