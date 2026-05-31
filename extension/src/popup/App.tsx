import { useState, useEffect } from 'react';
import { NotebooksView } from './components/NotebooksView';
import { UploadingView } from './components/UploadingView';
import { SendView } from './components/SendView';
import { ResultView } from './components/ResultView';
import { Notebook, Source, TabInfo, AppState } from './types';
import { t } from './i18n';
import { listSources } from '../shared/daemon-client';

export default function App() {
  const [isInitialized, setIsInitialized] = useState(false);
  const [currentPage, setCurrentPage] = useState<number>(0);
  const [notebooks, setNotebooks] = useState<Notebook[]>([]);
  const [selectedNotebook, setSelectedNotebook] = useState<Notebook | null>(null);
  const [sources, setSources] = useState<Source[]>([]);
  const [newSourceId, setNewSourceId] = useState<string | null>(null);
  const [currentJobId, setCurrentJobId] = useState<string | null>(null);
  const [currentTab, setCurrentTab] = useState<TabInfo | null>(null);

  const [selectedSourceIds, setSelectedSourceIds] = useState<Set<string>>(new Set());
  const [tasks, setTasks] = useState({
    audio_overview: false,
    slide_deck: false,
    video_overview: false,
  });

  const [statusMsg, setStatusMsg] = useState<{ text: string; isError: boolean } | null>(null);

  useEffect(() => {
    // apply i18n to static HTML attributes if any, but we will mostly use `t()` directly.
    document.querySelectorAll("[data-i18n]").forEach((node: any) => {
      node.textContent = t(node.dataset.i18n);
    });

    const init = async () => {
      try {
        const tabResponse = await chrome.runtime.sendMessage({ type: "get-current-tab" });
        const tab = tabResponse?.tab || null;
        setCurrentTab(tab);

        const data = await chrome.storage.local.get("appState");
        const saved = data.appState as AppState;
        const currentUrl = tab?.url || "";

        if (saved && saved.lastUrl === currentUrl && saved.currentPage > 0) {
          setSelectedNotebook(saved.selectedNotebook);
          setSources(saved.sources || []);
          setNewSourceId(saved.newSourceId);
          setCurrentJobId(saved.currentJobId);
          if (saved.selectedSourceIds) setSelectedSourceIds(new Set(saved.selectedSourceIds));
          if (saved.tasks) setTasks(saved.tasks);

          if (saved.currentPage === 1) {
            handleNotebookSelection(saved.selectedNotebook, tab);
          } else if (saved.currentPage === 2) {
            setCurrentPage(2);
          } else if (saved.currentPage === 3) {
            setCurrentPage(3);
          }
        } else {
          await chrome.storage.local.remove("appState");
        }
      } catch (err: any) {
        setStatus(err.message, true);
      } finally {
        setIsInitialized(true);
      }
    };
    init();
  }, []);

  useEffect(() => {
    if (!isInitialized) return;
    
    chrome.storage.local.set({
      appState: {
        currentPage,
        selectedNotebook,
        sources,
        newSourceId,
        currentJobId,
        lastUrl: currentTab?.url || "",
        selectedSourceIds: Array.from(selectedSourceIds),
        tasks,
      }
    });
  }, [currentPage, selectedNotebook, sources, newSourceId, currentJobId, currentTab, selectedSourceIds, tasks]);

  const setStatus = (text: string, isError = false) => {
    if (!text) setStatusMsg(null);
    else setStatusMsg({ text, isError });
  };

  const handleNotebookSelection = async (notebook: Notebook | null, tab: TabInfo | null = currentTab) => {
    if (!notebook) return;
    setSelectedNotebook(notebook);
    setCurrentPage(1); // Uploading
    setStatus("");

    try {
      let result;
      if (tab?.url && tab.url.startsWith("http")) {
        const response = await chrome.runtime.sendMessage({
          type: "start-upload",
          notebookId: notebook.id,
          url: tab.url
        });
        if (response.status === "error") {
          throw new Error(response.error);
        }
        result = response.result;
        setNewSourceId(result.source_id);
        setSelectedSourceIds(new Set([result.source_id]));
      } else {
        result = await listSources(notebook.id);
        setNewSourceId(null);
        setSelectedSourceIds(new Set());
      }
      setSources(result.sources || []);
      if (result.warning) setStatus(result.warning, true);
      setCurrentPage(2);
    } catch (err: any) {
      setStatus(err.message, true);
      // Stay on page 1 but show retry button
    }
  };

  const handleBack = () => {
    if (currentPage === 2) setCurrentPage(0);
    else if (currentPage === 3) setCurrentPage(2);
    else setCurrentPage(0);
  };

  return (
    <div className="app">
      <div id="backBar" className={`back-bar ${currentPage === 0 || currentPage === 1 ? 'hidden' : ''}`}>
        <button id="backButton" className="ghost" type="button" onClick={handleBack}>
          {t('backButton')}
        </button>
      </div>

      <div id="statusBanner" className={`status-banner ${statusMsg ? (statusMsg.isError ? 'error' : '') : 'hidden'}`}>
        {statusMsg?.text}
      </div>

      <div className="pages-shell">
        {currentPage === 0 && (
          <div className="page" id="page-notebooks" style={{ height: '480px' }}>
            <NotebooksView
              notebooks={notebooks}
              setNotebooks={setNotebooks}
              onSelect={(nb) => handleNotebookSelection(nb, currentTab)}
              setStatus={setStatus}
            />
          </div>
        )}

        {currentPage === 1 && (
          <div className="page" id="page-uploading">
            <UploadingView
              onRetry={() => handleNotebookSelection(selectedNotebook, currentTab)}
              hasError={!!(statusMsg && statusMsg.isError)}
            />
          </div>
        )}

        {currentPage === 2 && (
          <div className="page" id="page-send">
            <SendView
              notebook={selectedNotebook}
              currentTab={currentTab}
              sources={sources}
              newSourceId={newSourceId}
              selectedSourceIds={selectedSourceIds}
              setSelectedSourceIds={setSelectedSourceIds}
              tasks={tasks}
              setTasks={setTasks}
              setStatus={setStatus}
              onJobAccepted={(jobId) => {
                setCurrentJobId(jobId);
                setCurrentPage(3);
              }}
            />
          </div>
        )}

        {currentPage === 3 && (
          <div className="page" id="page-result">
            <ResultView
              jobId={currentJobId}
              notebook={selectedNotebook}
              setStatus={setStatus}
            />
          </div>
        )}
      </div>
    </div>
  );
}
