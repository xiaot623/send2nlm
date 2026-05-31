import { useState, useEffect } from 'react';
import { Job } from '../types';
import { t } from '../i18n';
import { listJobs, clearJobs } from '../../shared/daemon-client';

interface Props {
  setStatus: (msg: string, isError?: boolean) => void;
}

export function TasksView({ setStatus }: Props) {
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchJobs = async () => {
    try {
      const data = await listJobs();
      if (data && data.jobs) {
        setJobs(data.jobs);
      }
    } catch (err: any) {
      setStatus(err.message, true);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchJobs();
    const interval = setInterval(fetchJobs, 3000);
    return () => clearInterval(interval);
  }, []);

  const handleClear = async () => {
    try {
      setLoading(true);
      await clearJobs();
      await fetchJobs();
    } catch (err: any) {
      setStatus(err.message, true);
    }
  };

  const getStatusLabel = (status: string) => {
    switch (status) {
      case 'completed':
      case 'done':
        return <span className="status-badge success">{t('statusCompleted') || 'Completed'}</span>;
      case 'failed':
        return <span className="status-badge error">{t('statusFailed') || 'Failed'}</span>;
      case 'tasking':
        return <span className="status-badge pending">{t('statusTasking') || 'Creating Tasks'}</span>;
      case 'polling':
        return <span className="status-badge pending">{t('statusPolling') || 'Generating'}</span>;
      case 'downloading':
        return <span className="status-badge pending">{t('statusDownloading') || 'Downloading'}</span>;
      case 'receiving':
        return <span className="status-badge pending">{t('statusReceiving') || 'Receiving'}</span>;
      case 'pending':
        return <span className="status-badge pending">{t('statusPending') || 'Pending'}</span>;
      default:
        return <span className="status-badge pending">{status}</span>;
    }
  };

  return (
    <div className="tasks-view" style={{ padding: '16px', display: 'flex', flexDirection: 'column', height: '100%', boxSizing: 'border-box' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
        <h2 style={{ margin: 0, fontSize: '18px', fontWeight: 600 }}>{t('tasksTitle') || 'Tasks'}</h2>
        <button className="secondary" onClick={handleClear} disabled={loading} style={{ padding: '4px 8px', fontSize: '12px' }}>
          {t('clearTasks') || 'Clear Completed'}
        </button>
      </div>

      <div className="tasks-list" style={{ flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: '12px' }}>
        {loading && jobs.length === 0 ? (
          <div style={{ textAlign: 'center', color: 'var(--text-secondary)', padding: '24px 0' }}>{t('loading') || 'Loading...'}</div>
        ) : jobs.length === 0 ? (
          <div style={{ textAlign: 'center', color: 'var(--text-secondary)', padding: '24px 0' }}>{t('noTasks') || 'No tasks found.'}</div>
        ) : (
          jobs.map((job) => (
            <div key={job.job_id} className="task-item" style={{ 
              padding: '12px', 
              borderRadius: '8px', 
              border: '1px solid var(--border-color)',
              background: 'var(--bg-secondary)',
              display: 'flex',
              flexDirection: 'column',
              gap: '8px'
            }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                <div style={{ fontWeight: 500, fontSize: '14px', wordBreak: 'break-all', paddingRight: '8px' }}>
                  {job.notebook_title || job.url || job.job_id}
                </div>
                {getStatusLabel(job.status)}
              </div>
              
              {job.error && (
                <div style={{ fontSize: '12px', color: 'var(--error-color)', background: 'rgba(239, 68, 68, 0.1)', padding: '6px', borderRadius: '4px' }}>
                  {job.error}
                </div>
              )}
              
              <div style={{ fontSize: '11px', color: 'var(--text-secondary)', display: 'flex', justifyContent: 'space-between' }}>
                <span>{job.created_at ? new Date(job.created_at).toLocaleString() : ''}</span>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
