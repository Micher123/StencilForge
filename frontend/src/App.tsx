import React, { useState, useEffect, useCallback } from 'react';
import { extractErrorMessage } from './utils/errors';
import { BrowserRouter, Routes, Route, Link, useNavigate } from 'react-router-dom';
import ThemeToggle from './components/ThemeToggle';
import UploadZone from './components/UploadZone';
import LayersPanel from './components/LayersPanel';
import AuthPage from './components/AuthPage';
import ProfilePage from './components/ProfilePage';
import WelcomePage from './components/WelcomePage';
import TermsPage from './components/TermsPage';
import PrivacyPage from './components/PrivacyPage';
import './styles/global.css';

interface UserInfo {
  id: number;
  username: string;
  email: string;
  plan: string;
  max_layers?: number;
  newsletter_opt_in?: boolean;
}

// ---------- Shell ----------
const AppShell: React.FC = () => {
  const [token, setToken] = useState<string>(() => localStorage.getItem('stencilforge_token') || '');
  const [user, setUser] = useState<UserInfo | null>(null);
  const [theme, setTheme] = useState<'light' | 'dark'>(() =>
    (localStorage.getItem('stencilforge_theme') as 'light' | 'dark') || 'dark'
  );
  const [maxLayers, setMaxLayers] = useState(3);
  const navigate = useNavigate();

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('stencilforge_theme', theme);
  }, [theme]);

  const toggleTheme = () => setTheme(t => t === 'dark' ? 'light' : 'dark');

  const fetchUser = useCallback(async () => {
    if (!token) return;
    try {
      const res = await fetch('/api/me', {
        headers: { 'Authorization': `Bearer ${token}` },
      });
      if (!res.ok) {
        const msg = await extractErrorMessage(res);
        throw new Error(msg);
      }
      const data = await res.json();
      const u: UserInfo = data.user || data;
      setUser(u);
      setMaxLayers(u.max_layers || 3);
    } catch {
      localStorage.removeItem('stencilforge_token');
      setToken('');
      setUser(null);
      setMaxLayers(3);
    }
  }, [token]);

  useEffect(() => {
    fetchUser();
  }, [fetchUser]);

  const handleAuth = (t: string, _u: UserInfo) => {
    localStorage.setItem('stencilforge_token', t);
    setToken(t);
    navigate('/');
  };

  const handleLogout = () => {
    localStorage.removeItem('stencilforge_token');
    setToken('');
    setUser(null);
    setMaxLayers(3);
    navigate('/');
  };

  return (
    <div className="app">
      <header className="app-header">
        <div className="header-left">
          <Link to="/" className="logo">🖼 StencilForge</Link>
        </div>
        <div className="header-right">
          <ThemeToggle theme={theme} onToggle={toggleTheme} />
          {token && user && (
            <>
              <Link to="/profile" className="nav-link">
                {user.username}{' '}
                <span className={`plan-badge plan-${user.plan}`}>{user.plan}</span>
              </Link>
              <button className="btn btn-sm btn-outline" onClick={handleLogout}>
                Выйти
              </button>
            </>
          )}
          {!token && (
            <Link to="/auth" className="btn btn-sm">
              Войти
            </Link>
          )}
        </div>
      </header>

      <main className="app-main">
        <Routes>
          <Route
            path="/"
            element={
              token && user ? (
                <MainPage token={token} user={user} maxLayers={maxLayers} />
              ) : (
                <WelcomePage />
              )
            }
          />
          <Route path="/auth" element={<AuthPage onAuth={handleAuth} />} />
          <Route path="/profile" element={<ProfilePage token={token} />} />
          <Route path="/terms" element={<TermsPage />} />
          <Route path="/privacy" element={<PrivacyPage />} />
        </Routes>
      </main>

      <footer className="app-footer">
        <span>StencilForge © 2026</span>
        <span className="footer-inn">ИНН 235214614964</span>
      </footer>
    </div>
  );
};

// ---------- Main page ----------
const MainPage: React.FC<{
  token: string;
  user: UserInfo | null;
  maxLayers: number;
}> = ({ token, user, maxLayers }) => {
  const [sessionID, setSessionID] = useState('');
  const [numLayers, setNumLayers] = useState(Math.min(3, maxLayers));
  const [processing, setProcessing] = useState(false);
  const [error, setError] = useState('');
  const [layers, setLayers] = useState<
    { index: number; download_url: string; data_url: string }[]
  >([]);

  useEffect(() => {
    if (numLayers > maxLayers) setNumLayers(maxLayers);
  }, [maxLayers, numLayers]);

  const handleUploaded = useCallback((sid: string, _dataUrl: string) => {
    setSessionID(sid);
    setLayers([]);
    setError('');
  }, []);

  const handleUploadError = useCallback((msg: string) => {
    setError(msg);
  }, []);

  const handleGenerate = async () => {
    if (!sessionID) {
      setError('Сначала загрузите изображение');
      return;
    }
    if (!token) {
      setError('Войдите в аккаунт для обработки');
      return;
    }

    setProcessing(true);
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
          session_id: sessionID,
          num_layers: numLayers,
          auto_layers: false,
        }),
      });
      if (!res.ok) {
        const msg = await extractErrorMessage(res);
        throw new Error(msg);
      }
      const data = await res.json();
      setLayers(data.layers || []);
    } catch (e: any) {
      setError(e.message || 'Ошибка генерации слоёв');
    } finally {
      setProcessing(false);
    }
  };

  return (
    <div className="main">
      <UploadZone onUploaded={handleUploaded} onError={handleUploadError} token={token} />

      {token && (
        <div className="layers-control">
          <label>
            Количество слоёв:{' '}
            <input
              type="number"
              min={1}
              max={maxLayers}
              value={numLayers}
              onChange={e =>
                setNumLayers(
                  Math.min(maxLayers, Math.max(1, Number(e.target.value) || 1)),
                )
              }
              className="num-input"
              disabled={!sessionID}
            />
          </label>
          <span className="limit-hint">
            (макс. {maxLayers} для вашего тарифа)
          </span>
          <button
            className="btn btn-primary"
            onClick={handleGenerate}
            disabled={!sessionID || processing}
            title={!sessionID ? 'Сначала загрузите изображение' : ''}
          >
            {processing ? 'Обработка...' : 'Создать трафареты'}
          </button>
        </div>
      )}

      {error && <div className="msg msg-error">{error}</div>}

      {processing && (
        <div className="page-center">
          <div className="spinner"></div>
          <p>Кластеризация и построение слоёв...</p>
        </div>
      )}

      {layers.length > 0 && (
        <LayersPanel layers={layers} loading={false} />
      )}
    </div>
  );
};

// ---------- Root ----------
const App: React.FC = () => {
  return (
    <BrowserRouter>
      <AppShell />
    </BrowserRouter>
  );
};

export default App;