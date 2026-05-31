import { useState, useEffect } from 'react';
import { RefreshCw } from 'lucide-react';
import { Notebook } from '../types';
import { listNotebooks, createNotebook } from '../../shared/daemon-client';
import { t } from '../i18n';

interface Props {
  notebooks: Notebook[];
  setNotebooks: (nbs: Notebook[]) => void;
  onSelect: (nb: Notebook) => void;
  setStatus: (msg: string, isError?: boolean) => void;
}

export function NotebooksView({ notebooks, setNotebooks, onSelect, setStatus }: Props) {
  const [title, setTitle] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  const loadNotebooks = async (refresh = false) => {
    setStatus(t('loadingNotebooks') || 'Loading notebooks…');
    setIsLoading(true);
    try {
      const result = await listNotebooks({ refresh });
      setNotebooks(result.notebooks || []);
      if (result.warning) setStatus(result.warning, true);
      else setStatus('');
    } catch (error: any) {
      setStatus(error.message, true);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadNotebooks(false);
  }, []);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim()) {
      setStatus('Notebook title is required.', true);
      return;
    }

    setStatus('Creating notebook…');
    try {
      const notebook = await createNotebook({ title: title.trim() });
      setTitle('');
      setNotebooks([notebook, ...notebooks]);
      setStatus('Notebook created.');
    } catch (error: any) {
      setStatus(error.message, true);
    }
  };

  return (
    <>
      <div className="section-header shrink-0">
        <div className="section-title">{t('selectNotebook') || 'Select Notebook'}</div>
        <button 
          className="icon-button" 
          type="button" 
          aria-label="Refresh" 
          title="Refresh"
          onClick={() => loadNotebooks(true)}
          disabled={isLoading}
        >
          <RefreshCw size={16} className={isLoading ? 'animate-spin' : ''} />
        </button>
      </div>
      <form className="create-form shrink-0" onSubmit={handleCreate}>
        <input 
          placeholder="Notebook title" 
          value={title}
          onChange={e => setTitle(e.target.value)}
        />
        <button type="submit">{t('createNotebook') || 'Create'}</button>
      </form>

      <div className="notebook-list">
        {notebooks.length === 0 ? (
          <div className="detail-card">No notebooks yet.</div>
        ) : (
          notebooks.map(nb => (
            <button 
              key={nb.id} 
              type="button" 
              className="notebook-item transition-all" 
              onClick={() => onSelect(nb)}
            >
              <div className="notebook-title">{nb.title}</div>
            </button>
          ))
        )}
      </div>
    </>
  );
}
