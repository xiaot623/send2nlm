interface Props {
  onRetry: () => void;
  hasError: boolean;
}

export function UploadingView({ onRetry, hasError }: Props) {
  return (
    <div className="uploading-state">
      {!hasError && <div className="spinner"></div>}
      <div className="upload-status-text">
        {hasError ? 'Upload failed' : 'Converting and uploading page...'}
      </div>
      {hasError && (
        <button className="primary" type="button" onClick={onRetry}>
          Retry
        </button>
      )}
    </div>
  );
}
