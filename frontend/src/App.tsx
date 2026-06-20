import { useState, useCallback, useEffect } from 'react';
import UploadZone from './components/UploadZone';
import LayersPanel from './components/LayersPanel';
import ThemeToggle from './components/ThemeToggle';
import AuthPage from './components/AuthPage';

interface LayerInfo {
  index: number;
  download_url: string;
  data_url: string;
}

interface UserInfo {
  id: number;
  username: string;
  email: string;
  plan: string;
  newsletter_opt_in: boolean;
}

type AppPhase = 'upload' | 'layers';

function App() {
  const [theme, setTheme] = useState<'light' | 'dark'>(() => {
    return (localStorage.getItem('stencilforge-theme') as 'light' | 'dark') || 'light';
  });
  const [token, setToken] = useState<string>(() => {
    return localStorage.getItem('stencilforge-token') || '';
  });
  const [user, setUser] = useState<UserInfo | null>(null);
  const [authChecked, setAuthChecked] = useState(false);
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

  // Проверка токена при загрузке
  useEffect(() => {
    if (!token) {
      setAuthChecked(true);
      return;
    }
    fetch('/api/me', {
      headers: { 'Authorization': `Bearer ${token}` },
    })
      .then((res) => res.json())
      .then((data) => {
        if (data.ok && data.user) {
          setUser(data.user);
        } else {
          localStorage.removeItem('stencilforge-token');
          setToken('');
        }
        setAuthChecked(true);
      })
      .catch(() => {
        setAuthChecked(true);
      });
  }, [token]);

  const handleAuth = useCallback((newToken: string, userInfo: UserInfo) => {
    setToken(newToken);
    setUser(userInfo);
    localStorage.setItem('stencilforge-token', newToken);
  }, []);

  const handleLogout = useCallback(() => {
    fetch('/api/logout', { method: 'POST' }).catch(() => { });
    localStorage.removeItem('stencilforge-token');
    setToken('');
    setUser(null);
    setPhase('upload');
    setSessionId('');
    setPreviewUrl('');
    setLayers([]);
    setError('');
  }, []);

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
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
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
  }, [sessionId, numLayers, token]);

  const handleDownloadAll = useCallback(async () => {
    if (!sessionId) return;
    try {
      const res = await fetch('/api/download-all', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({ session_id: sessionId }),
      });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || `Ошибка скачивания: ${res.status}`);
      }
      // Скачиваем zip-файл
      const blob = await res.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'stencilforge-layers.zip';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      window.URL.revokeObjectURL(url);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Неизвестная ошибка при скачивании';
      setError(msg);
    }
  }, [sessionId, token]);

  const handleReset = useCallback(() => {
    setPhase('upload');
    setSessionId('');
    setPreviewUrl('');
    setLayers([]);
    setError('');
  }, []);

  if (!authChecked) {
    return (
      <div className="app-container">
        <header className="app-header">
          <h1>StencilForge</h1>
          <ThemeToggle theme={theme} onToggle={toggleTheme} />
        </header>
        <main className="main-content">
          <div className="card">
            <p>Загрузка...</p>
          </div>
        </main>
      </div>
    );
  }

  // Не авторизован — показываем страницу входа/регистрации
  if (!token || !user) {
    return (
      <div className="app-container">
        <header className="app-header">
          <h1>StencilForge</h1>
          <ThemeToggle theme={theme} onToggle={toggleTheme} />
        </header>
        <main className="main-content">
          <AuthPage onAuth={handleAuth} />
        </main>
      </div>
    );
  }

  return (
    <div className="app-container">
      <header className="app-header">
        <h1>StencilForge</h1>
        <div className="header-right">
          <div className="user-info">
            <span className="user-name">{user.username}</span>
            <span className="user-plan">{user.plan}</span>
          </div>
          <ThemeToggle theme={theme} onToggle={toggleTheme} />
          <button className="btn btn-logout" onClick={handleLogout} title="Выйти">
            Выйти
          </button>
        </div>
      </header>

      <main className="main-content">
        {error && (
          <div className="error-msg">{error}</div>
        )}

        {phase === 'upload' && (
          <div className="card">
            <h2 className="card-title">Загрузка изображения</h2>
            <UploadZone onUploaded={handleUploaded} onError={setError} token={token} />
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
            {layers.length > 0 && (
              <div className="card">
                <h2 className="card-title">Скачать всё</h2>
                <button className="btn btn-primary" onClick={handleDownloadAll}>
                  Скачать все слои (ZIP)
                </button>
              </div>
            )}
          </>
        )}
      </main>
    </div>
  );
}

export default App;