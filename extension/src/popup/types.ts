export interface Notebook {
  id: string;
  title: string;
  emoji: string;
  url?: string;
}

export interface UploadedNotebook extends Notebook {
  source_ids: string[];
  last_used?: string;
}

export interface Source {
  id: string;
  title: string;
}

export interface Job {
  job_id: string;
  status: string;
  tasks: string[];
  task_results?: Record<string, { status: string }>;
  notebook_title?: string;
  notebook_id?: string;
  url?: string;
  error?: string;
  retry_count?: number;
  created_at?: string;
  updated_at?: string;
}

export interface TabInfo {
  id: number;
  title: string;
  url: string;
}

export interface AppState {
  currentPage: number;
  selectedNotebook: Notebook | null;
  sources: Source[];
  newSourceId: string | null;
  currentJobId: string | null;
  lastUrl: string;
  selectedSourceIds?: string[];
  uploadedNotebooks?: UploadedNotebook[];
  tasks?: {
    audio_overview: boolean;
    slide_deck: boolean;
    video_overview: boolean;
  };
}
