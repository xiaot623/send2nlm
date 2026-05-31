export function t(key) {
  return chrome.i18n.getMessage(key) || key;
}

export function applyI18n(root = document) {
  root.querySelectorAll("[data-i18n]").forEach((node) => {
    const key = node.dataset.i18n;
    node.textContent = t(key);
  });
}
