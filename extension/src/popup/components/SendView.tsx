import { useState } from 'react';
import { Notebook, Source, TabInfo } from '../types';
import { createJob } from '../../shared/daemon-client';
import { t } from '../i18n';

interface Props {
  notebook: Notebook | null;
  currentTab: TabInfo | null;
  sources: Source[];
  newSourceId: string | null;
  setStatus: (msg: string, isError?: boolean) => void;
  onJobAccepted: (jobId: string) => void;
}

export function SendView({ notebook, currentTab, sources, newSourceId, setStatus, onJobAccepted }: Props) {
  const [selectedSourceIds, setSelectedSourceIds] = useState<Set<string>>(new Set(newSourceId ? [newSourceId] : []));
  const [tasks, setTasks] = useState({
    audio_overview: false,
    slide_deck: false,
    video_overview: false,
  });

  const toggleSource = (id: string) => {
    const next = new Set(selectedSourceIds);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setSelectedSourceIds(next);
  };

  const toggleTask = (key: keyof typeof tasks) => {
    setTasks(prev => ({ ...prev, [key]: !prev[key] }));
  };

  const handleSend = async () => {
    if (!notebook) {
      setStatus('Notebook is missing.', true);
      return;
    }
    if (selectedSourceIds.size === 0) {
      setStatus('Please select at least one source.', true);
      return;
    }

    const taskList = Object.entries(tasks)
      .filter(([_, isSelected]) => isSelected)
      .map(([key]) => key);

    setStatus('Submitting job…');
    try {
      const result = await createJob({
        notebook_id: notebook.id,
        url: currentTab?.url || '',
        tasks: taskList,
        source_ids: Array.from(selectedSourceIds),
      });
      setStatus('Job accepted.');
      onJobAccepted(result.job_id);
    } catch (error: any) {
      setStatus(error.message, true);
    }
  };

  return (
    <>
      <div className="detail-card">
        <div className="label">Notebook</div>
        <div className="value font-semibold">
          {notebook ? `${notebook.emoji || '📒'} ${notebook.title}` : ''}
        </div>
      </div>
      <div className="detail-card mt-2">
        <div className="label">Page</div>
        <div className="url-preview">{currentTab?.url || 'No page context'}</div>
      </div>
      
      <div className="sources-section">
        <div className="section-title">Sources (<span className="text-blue-600">{sources.length}</span>)</div>
        <div className="source-list mt-2">
          {sources.length === 0 ? (
            <div className="detail-card">No sources available.</div>
          ) : (
            sources.map(source => {
              const isSelected = selectedSourceIds.has(source.id);
              const isNew = source.id === newSourceId;
              return (
                <label 
                  key={source.id} 
                  className={`source-item transition-all ${isSelected ? 'selected' : ''}`}
                >
                  <input 
                    type="checkbox" 
                    checked={isSelected} 
                    onChange={() => toggleSource(source.id)} 
                  />
                  <div className="source-title" title={source.title}>{source.title}</div>
                  {isNew && <span className="new-badge">[新]</span>}
                </label>
              );
            })
          )}
        </div>
      </div>

      <div className="options">
        <label className="flex items-center gap-2 cursor-pointer hover:text-blue-600 transition-colors">
          <input 
            type="checkbox" 
            checked={tasks.audio_overview} 
            onChange={() => toggleTask('audio_overview')} 
          /> 
          <span>{t('audioOverview') || 'Audio Overview'}</span>
        </label>
        <label className="flex items-center gap-2 cursor-pointer hover:text-blue-600 transition-colors">
          <input 
            type="checkbox" 
            checked={tasks.slide_deck} 
            onChange={() => toggleTask('slide_deck')} 
          /> 
          <span>{t('slideDeck') || 'Slide Deck'}</span>
        </label>
        <label className="flex items-center gap-2 cursor-pointer hover:text-blue-600 transition-colors">
          <input 
            type="checkbox" 
            checked={tasks.video_overview} 
            onChange={() => toggleTask('video_overview')} 
          /> 
          <span>{t('videoOverview') || 'Video Overview'}</span>
        </label>
      </div>
      
      <button className="primary w-full shadow-sm hover:shadow-md transition-all active:scale-[0.98]" type="button" onClick={handleSend}>
        {t('sendButton') || 'Send'}
      </button>
    </>
  );
}
