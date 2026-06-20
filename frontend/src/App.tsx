import { useState, useCallback, useEffect } from 'react';
import UploadZone from './components/UploadZone';
import LayersPanel from './components/LayersPanel';
import ThemeToggle from './components/ThemeToggle';

interface LayerInfo {
  index: number;
  download_url: string;
  data_url: string;
}

type AppPhase = 'upload' | 'layers';

function App() {
  const [theme, setTheme] = useState<'light' | 'dark'>(() => {
    return (localStorage.getItem('stencilforge-theme') as 'light' | 'dark') || 'light';
  });
  const [phase, setPhase] = useState<AppPhase>('upload');
  const [sessionId, setSessionId] = useState<string>('');
  const [previewUrl, setPreviewUrl] = useState<string>('');
  const [numLayers, setNumLayers] = useState<number>(4);
  const [layers, setLayers] = useState<LayerInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('stencilforge-theme', theme);
  }, [theme]);

  const toggleTheme = useCallback(() => {
    setTheme((prev) => (prev === 'light' ? 'dark' : 'light'));
  }, []);

  const handleUploaded = useCallback((sid: string, dataUrl: string) => {
    setSessionId(sid);
    setPreviewUrl(dataUrl);
    setPhase('layers');
    setLayers([]);
    setError('');
  }, []);

  const handleGenerate = useCallback(async () => {
    if (!sessionId) return;
    setLoading(true);
    setError('');
    setLayers([]);

    try {
      const res = await fetch('/api/layers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          session_id: sessionId,
          num_layers: numLayers,
          auto_layers: false,
        }),
      });

      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || `Ошибка сервера: ${res.status}`);
      }

      const data = await res.json();
      if (data.layers && Array.isArray(data.layers)) {
        setLayers(data.layers);
      } else {
        throw new Error('Некорректный ответ от сервера');
      }
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Неизвестная ошибка';
      setError(msg);
    } finally {
      setLoading(false);
    }
  }, [sessionId, numLayers]);

  const handleReset = useCallback(() => {
    setPhase('upload');
    setSessionId('');
    setPreviewUrl('');
    setLayers([]);
    setError('');
  }, []);

  return (
    <div className="app-container">
      <header className="app-header">
        <h1>StencilForge</h1>
        <ThemeToggle theme={theme} onToggle={toggleTheme} />
      </header>

      <main className="main-content">
        {error && (
          <div className="error-msg">{error}</div>
        )}

        {phase === 'upload' && (
          <div className="card">
            <h2 className="card-title">Загрузка изображения</h2>
            <UploadZone onUploaded={handleUploaded} onError={setError} />
          </div>
        )}

        {phase === 'layers' && (
          <>
            <div className="card">
              <h2 className="card-title">Исходное изображение</h2>
              {previewUrl && (
                <img src={previewUrl} alt="Исходное" className="uploaded-image" />
              )}
            </div>

            <div className="card">
              <h2 className="card-title">Параметры трафаретов</h2>
              <div className="controls">
                <div className="control-group">
                  <label htmlFor="numLayers">Количество слоёв (2–16)</label>
                  <input
                    id="numLayers"
                    type="number"
                    min={2}
                    max={16}
                    value={numLayers}
                    onChange={(e) => setNumLayers(Math.max(2, Math.min(16, parseInt(e.target.value) || 4)))}
                  />
                </div>
                <button
                  className="btn btn-primary"
                  onClick={handleGenerate}
                  disabled={loading}
                >
                  {loading ? 'Обработка...' : 'Создать слои'}
                </button>
                <button className="btn btn-secondary" onClick={handleReset}>
                  Загрузить другое изображение
                </button>
              </div>
            </div>

            <LayersPanel layers={layers} loading={loading} />
          </>
        )}
      </main>
    </div>
  );
}

export default App;