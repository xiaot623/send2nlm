import { uploadResource } from "../shared/daemon-client";

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
