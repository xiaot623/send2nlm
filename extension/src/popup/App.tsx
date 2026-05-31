import { useState, useEffect } from 'react';
import { NotebooksView } from './components/NotebooksView';
import { UploadedNotebooksView } from './components/UploadedNotebooksView';
import { UploadingView } from './components/UploadingView';
import { SendView } from './components/SendView';
import { ResultView } from './components/ResultView';
import { TasksView } from './components/TasksView';
import { Notebook, Source, TabInfo, AppState, UploadedNotebook } from './types';
import { t } from './i18n';
import { listSources, listUploadedNotebooks } from '../shared/daemon-client';

export default function App() {
  const [isInitialized, setIsInitialized] = useState(false);
  const [currentPage, setCurrentPage] = useState<number>(0);
  const [notebooks, setNotebooks] = useState<Notebook[]>([]);
  const [uploadedNotebooks, setUploadedNotebooks] = useState<UploadedNotebook[]>([]);
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
          setUploadedNotebooks(saved.uploadedNotebooks || []);
          if (saved.selectedSourceIds) setSelectedSourceIds(new Set(saved.selectedSourceIds));
          if (saved.tasks) setTasks(saved.tasks);

          if (saved.currentPage === 1) {
            handleNotebookSelection(saved.selectedNotebook, tab);
          } else if (saved.currentPage === 2) {
            setCurrentPage(2);
          } else if (saved.currentPage === 3) {
            setCurrentPage(3);
          } else if (saved.currentPage === 4) {
            setCurrentPage(4);
          } else if (saved.currentPage === 5) {
            setCurrentPage(5);
          }
        } else {
          await chrome.storage.local.remove("appState");
          if (currentUrl.startsWith("http")) {
            const uploaded = await listUploadedNotebooks(currentUrl);
            const uploadedList = uploaded.notebooks || [];
            setUploadedNotebooks(uploadedList);
            if (uploadedList.length > 0) {
              setCurrentPage(5);
            }
          }
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
        uploadedNotebooks,
        lastUrl: currentTab?.url || "",
        selectedSourceIds: Array.from(selectedSourceIds),
        tasks,
      }
    });
  }, [currentPage, selectedNotebook, sources, newSourceId, currentJobId, uploadedNotebooks, currentTab, selectedSourceIds, tasks]);

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

  const handleUploadedNotebookSelection = async (notebook: UploadedNotebook) => {
    setSelectedNotebook(notebook);
    setNewSourceId(null);
    setStatus("");

    try {
      const result = await listSources(notebook.id);
      const nextSources = result.sources || [];
      const availableSourceIds = new Set(nextSources.map((source: Source) => source.id));
      const selectedIds = (notebook.source_ids || []).filter(id => availableSourceIds.has(id));
      setSources(nextSources);
      setSelectedSourceIds(new Set(selectedIds));
      if (result.warning) setStatus(result.warning, true);
      setCurrentPage(2);
    } catch (err: any) {
      setStatus(err.message, true);
    }
  };

  const handleBack = () => {
    if (currentPage === 2) setCurrentPage(0);
    else if (currentPage === 3) setCurrentPage(2);
    else if (currentPage === 4) setCurrentPage(0);
    else if (currentPage === 5) setCurrentPage(0);
    else setCurrentPage(0);
  };

  return (
    <div className="app">
      <div id="backBar" className={`back-bar ${currentPage === 0 || currentPage === 1 ? 'hidden' : ''}`}>
        <button id="backButton" className="ghost" type="button" onClick={handleBack}>
          {t('backButton')}
        </button>
      </div>
      
      {currentPage === 0 && (
        <div style={{ position: 'absolute', top: '12px', right: '12px', zIndex: 10 }}>
          <button className="ghost" onClick={() => setCurrentPage(4)} style={{ padding: '4px 8px', fontSize: '18px' }} title={t('tasksTitle') || 'Tasks'}>
            📋
          </button>
        </div>
      )}

      <div id="statusBanner" className={`status-banner ${statusMsg ? (statusMsg.isError ? 'error' : '') : 'hidden'}`}>
        {statusMsg?.text}
      </div>

      <div className="pages-shell relative overflow-hidden" style={{ width: '360px', height: '480px' }}>
        <div 
          className="pages flex h-full transition-transform duration-300"
          style={{ 
            width: '2160px', // 6 pages * 360px
            transform: `translateX(-${currentPage * 360}px)`,
            transitionTimingFunction: 'cubic-bezier(0.4, 0, 0.2, 1)'
          }}
        >
          <div className="page shrink-0 w-[360px] h-full overflow-y-auto" id="page-notebooks">
            <NotebooksView
              notebooks={notebooks}
              setNotebooks={setNotebooks}
              onSelect={(nb) => handleNotebookSelection(nb, currentTab)}
              setStatus={setStatus}
            />
          </div>

          <div className="page shrink-0 w-[360px] h-full overflow-y-auto" id="page-uploading">
            <UploadingView
              onRetry={() => handleNotebookSelection(selectedNotebook, currentTab)}
              hasError={!!(statusMsg && statusMsg.isError)}
            />
          </div>

          <div className="page shrink-0 w-[360px] h-full overflow-y-auto" id="page-send">
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

          <div className="page shrink-0 w-[360px] h-full overflow-y-auto" id="page-result">
            <ResultView
              jobId={currentJobId}
              notebook={selectedNotebook}
              setStatus={setStatus}
            />
          </div>

          <div className="page shrink-0 w-[360px] h-full overflow-y-auto" id="page-tasks">
            <TasksView setStatus={setStatus} />
          </div>

          <div className="page shrink-0 w-[360px] h-full overflow-y-auto" id="page-uploaded-notebooks">
            <UploadedNotebooksView
              notebooks={uploadedNotebooks}
              onSelect={handleUploadedNotebookSelection}
              onUploadNew={() => setCurrentPage(0)}
            />
          </div>
        </div>
      </div>
    </div>
  );
}
