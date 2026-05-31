import { uploadResource, listJobs } from "../shared/daemon-client";

const uploadTasks = new Map<string, Promise<any>>();

chrome.runtime.onMessage.addListener((message: any, _sender: chrome.runtime.MessageSender, sendResponse: (response?: any) => void) => {
  if (message && message.type === "get-current-tab") {
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs: chrome.tabs.Tab[]) => {
      const tab = tabs ? tabs[0] : null;
      sendResponse({
        ok: Boolean(tab),
        tab: tab ? {
          id: tab.id,
          title: tab.title || "",
          url: tab.url || "",
        } : null
      });
    });
    return true; // Keep the message channel open for sendResponse
  }

  if (message && message.type === "start-upload") {
    const notebookId = message.notebookId;
    const url = message.url;
    const key = notebookId + "|" + url;

    if (!uploadTasks.has(key)) {
      const taskPromise = uploadResource(notebookId, url)
        .then((result) => {
          return { status: "done", result: result };
        })
        .catch((err: any) => {
          return { status: "error", error: err.message || String(err) };
        });
      uploadTasks.set(key, taskPromise);

      setTimeout(() => {
        uploadTasks.delete(key);
      }, 5 * 60 * 1000); // 5 minutes
    }

    // Call sendResponse when the upload task finishes
    uploadTasks.get(key)!.then(sendResponse);
    return true; // Keep the message channel open for sendResponse
  }

  return false;
});

// Poll for pending jobs and update badge
async function checkPendingJobs() {
  try {
    const result = await listJobs();
    if (result && result.jobs) {
      const pendingJobs = result.jobs.filter((j: any) => j.status !== 'done' && j.status !== 'completed' && j.status !== 'failed');
      const count = pendingJobs.length;
      if (count > 0) {
        chrome.action.setBadgeText({ text: count.toString() });
        chrome.action.setBadgeBackgroundColor({ color: '#4caf50' });
      } else {
        chrome.action.setBadgeText({ text: '' });
      }
    }
  } catch (e) {
    // Daemon might be down, ignore
    chrome.action.setBadgeText({ text: '' });
  }
}

// Check every 5 seconds
setInterval(checkPendingJobs, 5000);
checkPendingJobs();
