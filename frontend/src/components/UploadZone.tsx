import { useState, useRef, useCallback } from 'react';

interface Props {
  onUploaded: (sessionId: string, dataUrl: string) => void;
  onError: (msg: string) => void;
  token: string;
}

function UploadZone({ onUploaded, onError, token }: Props) {
  const [dragOver, setDragOver] = useState(false);
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleFile = useCallback(
    async (file: File) => {
      const allowedExts = ['.png', '.jpg', '.jpeg', '.bmp', '.tiff', '.tif'];
      const ext = '.' + file.name.split('.').pop()?.toLowerCase();
      if (!allowedExts.includes(ext)) {
        onError(`Неподдерживаемый формат: ${ext}. Поддерживаются: PNG, JPG, BMP, TIFF.`);
        return;
      }

      if (file.size > 50 * 1024 * 1024) {
        onError('Файл слишком большой. Максимальный размер: 50MB.');
        return;
      }

      setUploading(true);
      onError('');

      try {
        // Читаем dataURL для превью
        const dataUrl = await new Promise<string>((resolve, reject) => {
          const reader = new FileReader();
          reader.onload = () => resolve(reader.result as string);
          reader.onerror = () => reject(new Error('Не удалось прочитать файл'));
          reader.readAsDataURL(file);
        });

        // Загружаем на сервер
        const formData = new FormData();
        formData.append('image', file);

        const res = await fetch('/api/upload', {
          method: 'POST',
          headers: {
            'Authorization': `Bearer ${token}`,
          },
          body: formData,
        });

        if (!res.ok) {
          const text = await res.text();
          throw new Error(text || `Ошибка загрузки: ${res.status}`);
        }

        const data = await res.json();
        onUploaded(data.session_id, dataUrl);
      } catch (e: unknown) {
        const msg = e instanceof Error ? e.message : 'Неизвестная ошибка при загрузке';
        onError(msg);
      } finally {
        setUploading(false);
      }
    },
    [onUploaded, onError]
  );

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      setDragOver(false);
      const file = e.dataTransfer.files?.[0];
      if (file) handleFile(file);
    },
    [handleFile]
  );

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
  }, []);

  const handleChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (file) handleFile(file);
    },
    [handleFile]
  );

  return (
    <div
      className={`upload-zone ${dragOver ? 'drag-over' : ''}`}
      onDrop={handleDrop}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onClick={() => fileInputRef.current?.click()}
    >
      <input
        ref={fileInputRef}
        type="file"
        accept="image/png,image/jpeg,image/bmp,image/tiff"
        onChange={handleChange}
      />
      <div className="icon">{uploading ? '⏳' : '📁'}</div>
      <p>{uploading ? 'Загрузка...' : 'Перетащите изображение сюда или кликните для выбора'}</p>
      <p className="hint">PNG, JPG, BMP, TIFF — до 50MB</p>
    </div>
  );
}

export default UploadZone;