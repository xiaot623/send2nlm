import { createJob, createNotebook, getJob, listNotebooks } from "../shared/daemon-client.js";
import { applyI18n } from "../shared/i18n.js";

const state = {
  currentPage: 0,
  notebooks: [],
  selectedNotebook: null,
  currentTab: null,
  currentJobId: null,
  pollTimer: null,
};

const elements = {
  pages: document.getElementById("pages"),
  backButton: document.getElementById("backButton"),
  refreshButton: document.getElementById("refreshButton"),
  notebookList: document.getElementById("notebookList"),
  createNotebookForm: document.getElementById("createNotebookForm"),
  titleInput: document.getElementById("titleInput"),
  emojiInput: document.getElementById("emojiInput"),
  selectedNotebookTitle: document.getElementById("selectedNotebookTitle"),
  pageUrlPreview: document.getElementById("pageUrlPreview"),
  audioTask: document.getElementById("audioTask"),
  slidesTask: document.getElementById("slidesTask"),
  sendButton: document.getElementById("sendButton"),
  resultNotebook: document.getElementById("resultNotebook"),
  resultUrl: document.getElementById("resultUrl"),
  resultSteps: document.getElementById("resultSteps"),
  viewNotebookLink: document.getElementById("viewNotebookLink"),
  statusBanner: document.getElementById("statusBanner"),
  backBar: document.getElementById("backBar"),
};

function setPage(page) {
  state.currentPage = page;
  elements.pages.style.transform = `translateX(-${page * 33.3333}%)`;
  elements.backBar.classList.toggle("hidden", page === 0);
}

function setStatus(message, isError = false) {
  if (!message) {
    elements.statusBanner.textContent = "";
    elements.statusBanner.className = "status-banner hidden";
    return;
  }
  elements.statusBanner.textContent = message;
  elements.statusBanner.className = `status-banner${isError ? " error" : ""}`;
}

function renderNotebooks() {
  if (state.notebooks.length === 0) {
    elements.notebookList.innerHTML = `<div class="detail-card">No notebooks yet.</div>`;
    return;
  }

  elements.notebookList.innerHTML = "";
  state.notebooks.forEach((notebook) => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "notebook-item";
    button.innerHTML = `
      <div class="notebook-title">${notebook.emoji || "📒"} ${notebook.title}</div>
    `;
    button.addEventListener("click", () => {
      state.selectedNotebook = notebook;
      elements.selectedNotebookTitle.textContent = `${notebook.emoji || "📒"} ${notebook.title}`;
      elements.pageUrlPreview.textContent = state.currentTab?.url || "";
      elements.viewNotebookLink.href = notebook.url || "#";
      elements.viewNotebookLink.classList.toggle("hidden", !notebook.url);
      setPage(1);
    });
    elements.notebookList.appendChild(button);
  });
}

async function loadCurrentTab() {
  const response = await chrome.runtime.sendMessage({ type: "get-current-tab" });
  state.currentTab = response?.tab || null;
}

async function loadNotebooks(refresh = false) {
  setStatus("Loading notebooks…");
  try {
    const result = await listNotebooks({ refresh });
    state.notebooks = result.notebooks || [];
    renderNotebooks();
    setStatus(result.warning || "", Boolean(result.warning));
    if (!result.warning) {
      setStatus("");
    }
  } catch (error) {
    setStatus(error.message, true);
    renderNotebooks();
  }
}

async function handleCreateNotebook(event) {
  event.preventDefault();
  const title = elements.titleInput.value.trim();
  const emoji = elements.emojiInput.value.trim() || "📒";
  if (!title) {
    setStatus("Notebook title is required.", true);
    return;
  }

  setStatus("Creating notebook…");
  try {
    const notebook = await createNotebook({ title, emoji });
    elements.titleInput.value = "";
    state.notebooks.unshift(notebook);
    renderNotebooks();
    setStatus("Notebook created.");
  } catch (error) {
    setStatus(error.message, true);
  }
}

function renderJob(job) {
  const steps = [
    ["PDF", job.pdf_path ? "generated" : job.status === "pending" ? "waiting" : "processing"],
    ["Upload", job.source_id ? "done" : ["uploading", "tasking", "polling", "downloading", "receiving", "done"].includes(job.status) ? "processing" : "waiting"],
  ];

  for (const [taskType, label] of [
    ["audio_overview", "Audio Overview"],
    ["slide_deck", "Slide Deck"],
  ]) {
    if (!job.tasks?.includes(taskType)) continue;
    const task = job.task_results?.[taskType];
    steps.push([label, task?.status || (job.status === "done" ? "done" : "waiting")]);
  }

  elements.resultNotebook.textContent = state.selectedNotebook?.title || job.notebook_title || job.notebook_id;
  elements.resultUrl.textContent = job.url;
  elements.resultSteps.innerHTML = steps
    .map(([label, status]) => `<div class="step">${label}: ${status}</div>`)
    .join("");
}

async function pollJob(jobID) {
  try {
    const job = await getJob(jobID);
    renderJob(job);
    if (job.status === "done" || job.status === "failed") {
      clearInterval(state.pollTimer);
      state.pollTimer = null;
      setStatus(job.error || `Job ${job.status}.`, job.status === "failed");
    }
  } catch (error) {
    clearInterval(state.pollTimer);
    state.pollTimer = null;
    setStatus(error.message, true);
  }
}

async function handleSend() {
  if (!state.selectedNotebook || !state.currentTab?.url) {
    setStatus("Notebook or current tab is missing.", true);
    return;
  }

  const tasks = [];
  if (elements.audioTask.checked) tasks.push("audio_overview");
  if (elements.slidesTask.checked) tasks.push("slide_deck");

  setStatus("Submitting job…");
  try {
    const result = await createJob({
      notebook_id: state.selectedNotebook.id,
      url: state.currentTab.url,
      tasks,
    });
    state.currentJobId = result.job_id;
    setPage(2);
    setStatus("Job accepted.");
    await pollJob(state.currentJobId);
    state.pollTimer = setInterval(() => pollJob(state.currentJobId), 3000);
  } catch (error) {
    setStatus(error.message, true);
  }
}

function bindEvents() {
  elements.backButton.addEventListener("click", () => setPage(Math.max(0, state.currentPage - 1)));
  elements.refreshButton.addEventListener("click", () => loadNotebooks(true));
  elements.createNotebookForm.addEventListener("submit", handleCreateNotebook);
  elements.sendButton.addEventListener("click", handleSend);
}

async function init() {
  applyI18n();
  bindEvents();
  await loadCurrentTab();
  await loadNotebooks(false);
}

init().catch((error) => {
  setStatus(error.message, true);
});
