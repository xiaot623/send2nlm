chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message?.type !== "get-current-tab") {
    return false;
  }

  chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
    const tab = tabs?.[0];
    sendResponse({
      ok: Boolean(tab),
      tab: tab
        ? {
            id: tab.id,
            title: tab.title || "",
            url: tab.url || "",
          }
        : null,
    });
  });

  return true;
});
