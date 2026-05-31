import { UploadedNotebook } from '../types';
import { t } from '../i18n';

interface Props {
  notebooks: UploadedNotebook[];
  onSelect: (notebook: UploadedNotebook) => void;
  onUploadNew: () => void;
}

export function UploadedNotebooksView({ notebooks, onSelect, onUploadNew }: Props) {
  return (
    <>
      <div className="section-header shrink-0">
        <div>
          <div className="section-title">Already uploaded</div>
          <div className="label">Choose a notebook to skip uploading this page.</div>
        </div>
      </div>

      <div className="notebook-list">
        {notebooks.map(nb => (
          <button
            key={nb.id}
            type="button"
            className="notebook-item transition-all"
            onClick={() => onSelect(nb)}
          >
            <div className="notebook-title">{nb.emoji || '📒'} {nb.title}</div>
            <div className="label">{nb.source_ids.length} uploaded source{nb.source_ids.length === 1 ? '' : 's'}</div>
          </button>
        ))}
      </div>

      <button className="ghost w-full mt-3 shrink-0" type="button" onClick={onUploadNew}>
        {t('uploadAgainButton') || 'Upload again'}
      </button>
    </>
  );
}
