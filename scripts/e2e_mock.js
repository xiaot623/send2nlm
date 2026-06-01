#!/usr/bin/env node

const fs = require("fs");
const path = require("path");
const { spawn } = require("child_process");

const root = path.resolve(__dirname, "..");
const daemonDir = path.join(root, "daemon");
const devAssets = path.join(root, "dev_assets");
const statePath = path.join(devAssets, "mock_notebooklm.json");
const seedPath = path.join(root, "assets", "mock_notebooklm.seed.json");
const port = Number(process.env.SEND2NLM_E2E_PORT || 18924);
const baseURL = `http://127.0.0.1:${port}`;

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

async function request(pathname, options = {}) {
  const response = await fetch(`${baseURL}${pathname}`, {
    headers: {
      "Content-Type": "application/json",
      ...(options.headers || {}),
    },
    ...options,
  });
  let payload = null;
  try {
    payload = await response.json();
  } catch (_error) {
    payload = null;
  }
  if (!response.ok) {
    throw new Error(`${options.method || "GET"} ${pathname} failed (${response.status}): ${JSON.stringify(payload)}`);
  }
  return payload;
}

async function waitForHealth(child) {
  const started = Date.now();
  while (Date.now() - started < 30000) {
    if (child.exitCode !== null) {
      throw new Error(`daemon exited early with code ${child.exitCode}`);
    }
    try {
      return await request("/health");
    } catch (_error) {
      await new Promise((resolve) => setTimeout(resolve, 250));
    }
  }
  throw new Error("daemon did not become healthy");
}

async function waitForJobDone(jobID) {
  const started = Date.now();
  let lastJob = null;
  while (Date.now() - started < 10000) {
    lastJob = await request(`/jobs/${jobID}`);
    if (lastJob.status === "done") {
      return lastJob;
    }
    if (lastJob.status === "failed") {
      throw new Error(`job failed: ${lastJob.error}`);
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error(`job did not finish, last state: ${JSON.stringify(lastJob)}`);
}

async function main() {
  fs.mkdirSync(devAssets, { recursive: true });
  fs.copyFileSync(seedPath, statePath);

  const child = spawn("go", ["run", ".", "daemon", "--dev", "--port", String(port), "--mock-nlm", statePath], {
    cwd: daemonDir,
    detached: true,
    stdio: ["ignore", "pipe", "pipe"],
  });

  let output = "";
  child.stdout.on("data", (chunk) => {
    output += chunk.toString();
  });
  child.stderr.on("data", (chunk) => {
    output += chunk.toString();
  });

  try {
    await waitForHealth(child);

    const listed = await request("/notebooks?refresh=true");
    assert(Array.isArray(listed.notebooks), "notebooks response should contain an array");
    assert(listed.notebooks.length > 0, "seed should contain at least one notebook");
    const seedNotebook = listed.notebooks[0];

    const created = await request("/notebooks", {
      method: "POST",
      body: JSON.stringify({ title: "Mock E2E Notebook", emoji: "T" }),
    });
    assert(created.id, "created notebook should have an id");
    const stateAfterCreate = JSON.parse(fs.readFileSync(statePath, "utf8"));
    assert(stateAfterCreate.notebooks.some((notebook) => notebook.id === created.id), "created notebook should be persisted to mock state");

    const sources = await request(`/notebooks/${seedNotebook.id}/sources`);
    assert(Array.isArray(sources.sources), "sources response should contain an array");
    assert(sources.sources.length > 0, "seed notebook should contain at least one source");
    const sourceID = sources.sources[0].id;

    const accepted = await request("/jobs", {
      method: "POST",
      body: JSON.stringify({
        notebook_id: seedNotebook.id,
        url: "https://example.com/mock-e2e",
        source_ids: [sourceID],
        tasks: ["audio_overview", "slide_deck", "video_overview"],
      }),
    });
    assert(accepted.job_id, "job should be accepted with an id");

    const job = await waitForJobDone(accepted.job_id);
    for (const taskType of ["audio_overview", "slide_deck", "video_overview"]) {
      const result = job.task_results[taskType];
      assert(result, `${taskType} result should exist`);
      assert(result.task_id, `${taskType} should have a task id`);
      assert(result.status === "done", `${taskType} should be done`);
      assert(result.asset_path && fs.existsSync(result.asset_path), `${taskType} artifact should exist`);
      assert(result.asset_path.startsWith(path.join(devAssets, "tmp")), `${taskType} artifact should be under dev_assets/tmp`);
    }

    console.log("[e2e:mock] passed");
  } catch (error) {
    console.error("[e2e:mock] failed");
    console.error(error.stack || error.message);
    if (output.trim()) {
      console.error("\n--- daemon output ---");
      console.error(output.trim());
    }
    process.exitCode = 1;
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

main().catch((error) => {
  console.error(error.stack || error.message);
  process.exit(1);
});
