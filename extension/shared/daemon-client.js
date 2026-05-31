const DEFAULT_PORT = 18923;
const PORT_KEY = "daemon_port";

async function readSavedPort() {
  const result = await chrome.storage.local.get(PORT_KEY);
  return result?.[PORT_KEY] || null;
}

async function savePort(port) {
  await chrome.storage.local.set({ [PORT_KEY]: port });
}

async function request(port, path, options = {}) {
  const response = await fetch(`http://127.0.0.1:${port}${path}`, {
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
    throw new Error(payload?.error || `Daemon request failed (${response.status})`);
  }

  await savePort(port);
  return payload;
}

export async function resolvePort() {
  const ports = [DEFAULT_PORT];
  const savedPort = await readSavedPort();
  if (savedPort && savedPort !== DEFAULT_PORT) {
    ports.push(savedPort);
  }

  for (const port of ports) {
    try {
      await request(port, "/health");
      return port;
    } catch (_error) {
      // Try the next port.
    }
  }

  throw new Error("Send2NLM daemon is not reachable on localhost.");
}

export async function listNotebooks({ refresh = false } = {}) {
  const port = await resolvePort();
  return request(port, `/notebooks${refresh ? "?refresh=true" : ""}`);
}

export async function createNotebook(body) {
  const port = await resolvePort();
  return request(port, "/notebooks", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export async function createJob(body) {
  const port = await resolvePort();
  return request(port, "/jobs", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export async function getJob(jobID) {
  const port = await resolvePort();
  return request(port, `/jobs/${jobID}`);
}
