import { useState, useEffect } from 'react';
import { Job, Notebook } from '../types';
import { getJob } from '../../shared/daemon-client';
import { t } from '../i18n';
import { ExternalLink } from 'lucide-react';

interface Props {
  jobId: string | null;
  notebook: Notebook | null;
  setStatus: (msg: string, isError?: boolean) => void;
}

const TASK_LABELS: Record<string, string> = {
  audio_overview: 'Audio Overview',
  slide_deck: 'Slide Deck',
  video_overview: 'Video Overview',
};

export function ResultView({ jobId, notebook, setStatus }: Props) {
  const [job, setJob] = useState<Job | null>(null);

  useEffect(() => {
    if (!jobId) return;
    
    let timer: any;
    
    const poll = async () => {
      try {
        const result = await getJob(jobId);
        setJob(result);
        if (result.status === 'done' || result.status === 'failed') {
          clearInterval(timer);
          setStatus(result.error || `Job ${result.status}.`, result.status === 'failed');
        }
      } catch (error: any) {
        clearInterval(timer);
        setStatus(error.message, true);
      }
    };

    poll();
    timer = setInterval(poll, 3000);
    return () => clearInterval(timer);
  }, [jobId]);

  const steps = [];
  if (job?.tasks) {
    for (const taskType of ['audio_overview', 'slide_deck', 'video_overview']) {
      if (!job.tasks.includes(taskType)) continue;
      const task = job.task_results?.[taskType];
      steps.push({
        label: TASK_LABELS[taskType],
        status: task?.status || (job.status === 'done' ? 'done' : 'waiting')
      });
    }
  }

  const displayNotebookTitle = notebook?.title || job?.notebook_title || job?.notebook_id || '';
  const notebookUrl = notebook?.url;

  return (
    <>
      <div className="detail-card shadow-sm border-blue-100">
        <div className="label">Notebook</div>
        <div className="value font-semibold">{displayNotebookTitle}</div>
      </div>
      
      {job?.url && (
        <div className="detail-card mt-2">
          <div className="label">Page</div>
          <div className="url-preview text-sm text-gray-600">{job.url}</div>
        </div>
      )}
      
      <div className="result-steps mt-4">
        {steps.map((step, idx) => (
          <div key={idx} className="step shadow-sm flex justify-between items-center bg-white p-3 rounded-lg border border-gray-100">
            <span className="font-medium text-gray-700">{step.label}</span>
            <span className={`text-sm px-2 py-1 rounded-full ${
              step.status === 'done' ? 'bg-green-100 text-green-700' : 
              step.status === 'failed' ? 'bg-red-100 text-red-700' : 
              'bg-blue-100 text-blue-700 animate-pulse'
            }`}>
              {step.status}
            </span>
          </div>
        ))}
      </div>
      
      {notebookUrl && (
        <a 
          className="link-button mt-4 shadow-sm hover:shadow-md transition-all w-full inline-flex items-center justify-center gap-2" 
          href={notebookUrl} 
          target="_blank" 
          rel="noreferrer"
        >
          {t('viewInNotebookLM') || 'View in NotebookLM'}
          <ExternalLink size={16} />
        </a>
      )}
    </>
  );
}
