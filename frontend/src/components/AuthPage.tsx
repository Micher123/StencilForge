import { useState, useCallback, FormEvent } from 'react';

interface UserInfo {
  id: number;
  username: string;
  email: string;
  plan: string;
  newsletter_opt_in: boolean;
}

interface AuthPageProps {
  onAuth: (token: string, user: UserInfo) => void;
  token?: string;
}

type AuthMode = 'login' | 'register';

function AuthPage({ onAuth, token: existingToken }: AuthPageProps) {
  const [mode, setMode] = useState<AuthMode>('login');
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [passwordConfirm, setPasswordConfirm] = useState('');
  const [newsletter, setNewsletter] = useState(false);
  const [agreedToTerms, setAgreedToTerms] = useState(false);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [passwordStrengthMsg, setPasswordStrengthMsg] = useState('');
  const [gmailWarning, setGmailWarning] = useState(false);

  const switchMode = useCallback((m: AuthMode) => {
    setMode(m);
    setError('');
    setPasswordStrengthMsg('');
    setGmailWarning(false);
  }, []);

  const handleEmailChange = useCallback((value: string) => {
    setEmail(value);
    const lower = value.toLowerCase().trim();
    if (lower.endsWith('@gmail.com') || lower.endsWith('@googlemail.com')) {
      setGmailWarning(true);
    } else {
      setGmailWarning(false);
    }
  }, []);

  const handlePasswordChange = useCallback((value: string) => {
    setPassword(value);
    // Проверка сложности на клиенте (дублирует серверную)
    if (value.length === 0) {
      setPasswordStrengthMsg('');
    } else if (value.length < 8) {
      setPasswordStrengthMsg('Минимум 8 символов');
    } else if (!/[A-Z]/.test(value)) {
      setPasswordStrengthMsg('Нужна заглавная буква');
    } else if (!/[a-z]/.test(value)) {
      setPasswordStrengthMsg('Нужна строчная буква');
    } else if (!/[0-9]/.test(value)) {
      setPasswordStrengthMsg('Нужна цифра');
    } else if (!/[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?~]/.test(value)) {
      setPasswordStrengthMsg('Нужен спецсимвол (!@#$%^&* и т.д.)');
    } else {
      setPasswordStrengthMsg('Пароль надёжный ✓');
    }
  }, []);

  const canRegister = username.trim() !== '' && email.trim() !== '' && password !== '' && password === passwordConfirm && agreedToTerms && passwordStrengthMsg === 'Пароль надёжный ✓';
  const canLogin = email.trim() !== '' && password !== '';

  const handleSubmit = useCallback(async (e: FormEvent) => {
    e.preventDefault();
    setError('');

    if (mode === 'register') {
      if (!canRegister) {
        if (!agreedToTerms) {
          setError('Необходимо согласиться с Пользовательским соглашением и Политикой конфиденциальности');
        } else {
          setError('Пожалуйста, заполните все поля корректно');
        }
        return;
      }
    }

    setLoading(true);

    try {
      const endpoint = mode === 'register' ? '/api/register' : '/api/login';
      const body: Record<string, unknown> = { email: email.trim().toLowerCase(), password };

      if (mode === 'register') {
        body.username = username.trim();
        body.password_confirm = passwordConfirm;
        body.newsletter = newsletter;
      }

      const res = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });

      const data = await res.json();

      if (!res.ok || !data.ok) {
        throw new Error(data.error || 'Ошибка сервера');
      }

      if (data.token && data.user) {
        localStorage.setItem('stencilforge-token', data.token);
        onAuth(data.token, data.user);
      } else if (data.token) {
        // login может не возвращать user (хотя у нас возвращает)
        localStorage.setItem('stencilforge-token', data.token);
        // пробуем получить user через /api/me
        const meRes = await fetch('/api/me', {
          headers: { 'Authorization': `Bearer ${data.token}` },
        });
        const meData = await meRes.json();
        if (meData.ok && meData.user) {
          onAuth(data.token, meData.user);
        }
      }
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Неизвестная ошибка';
      setError(msg);
    } finally {
      setLoading(false);
    }
  }, [mode, canRegister, email, password, username, passwordConfirm, newsletter, agreedToTerms, onAuth]);

  return (
    <div className="auth-page">
      <div className="auth-card">
        <h2 className="auth-title">StencilForge</h2>
        <p className="auth-subtitle">
          {mode === 'login' ? 'Вход в аккаунт' : 'Регистрация'}
        </p>

        {error && <div className="auth-error">{error}</div>}

        <form onSubmit={handleSubmit} className="auth-form">
          {mode === 'register' && (
            <div className="form-group">
              <label htmlFor="username">Имя пользователя</label>
              <input
                id="username"
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="Ваше имя"
                required
                autoComplete="username"
              />
            </div>
          )}

          <div className="form-group">
            <label htmlFor="email">Email</label>
            <input
              id="email"
              type="email"
              value={email}
              onChange={(e) => handleEmailChange(e.target.value)}
              placeholder="example@mail.ru"
              required
              autoComplete="email"
            />
            {gmailWarning && mode === 'register' && (
              <div className="gmail-warning">
                <p>
                  В соответствии с законопроектом{' '}
                  <a
                    href="https://sozd.duma.gov.ru/bill/1110676-8"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="law-link"
                  >
                    № 1110676-8
                  </a>
                  , Правительство РФ наделяется правом устанавливать случаи, когда для регистрации на российских сайтах и в информационных системах обязательно использование адресов электронной почты, созданных в российской национальной доменной зоне (например, .ru, .рф).
                </p>
                <p>
                  Это положение вступает в силу с 1 сентября 2026 года. Обратите внимание, что после этой даты использование зарубежных почтовых сервисов для авторизации может быть ограничено или запрещено в определенных, устанавливаемых Правительством, случаях.
                </p>
              </div>
            )}
          </div>

          <div className="form-group">
            <label htmlFor="password">Пароль</label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => handlePasswordChange(e.target.value)}
              placeholder="••••••••"
              required
              autoComplete={mode === 'register' ? 'new-password' : 'current-password'}
            />
            {mode === 'register' && passwordStrengthMsg && (
              <div className={`password-strength ${passwordStrengthMsg.includes('✓') ? 'strong' : 'weak'}`}>
                {passwordStrengthMsg}
              </div>
            )}
          </div>

          {mode === 'register' && (
            <div className="form-group">
              <label htmlFor="passwordConfirm">Подтверждение пароля</label>
              <input
                id="passwordConfirm"
                type="password"
                value={passwordConfirm}
                onChange={(e) => setPasswordConfirm(e.target.value)}
                placeholder="••••••••"
                required
                autoComplete="new-password"
              />
              {passwordConfirm && password !== passwordConfirm && (
                <div className="password-mismatch">Пароли не совпадают</div>
              )}
            </div>
          )}

          {mode === 'register' && (
            <>
              <div className="form-group checkbox-group">
                <label className="checkbox-label">
                  <input
                    type="checkbox"
                    checked={newsletter}
                    onChange={(e) => setNewsletter(e.target.checked)}
                  />
                  <span>Подписаться на рассылку новостей</span>
                </label>
              </div>

              <div className="form-group checkbox-group">
                <label className="checkbox-label">
                  <input
                    type="checkbox"
                    checked={agreedToTerms}
                    onChange={(e) => setAgreedToTerms(e.target.checked)}
                    required
                  />
                  <span>
                    Я принимаю{' '}
                    <a href="#" className="term-link" onClick={(e) => { e.preventDefault(); alert('Пользовательское соглашение (заглушка)'); }}>
                      Пользовательское соглашение
                    </a>{' '}
                    и{' '}
                    <a href="#" className="term-link" onClick={(e) => { e.preventDefault(); alert('Политика конфиденциальности (заглушка)'); }}>
                      Политику конфиденциальности
                    </a>
                  </span>
                </label>
              </div>
            </>
          )}

          <button
            type="submit"
            className="btn btn-primary auth-submit"
            disabled={loading || (mode === 'register' ? !canRegister : !canLogin)}
          >
            {loading ? 'Загрузка...' : (mode === 'register' ? 'Зарегистрироваться' : 'Войти')}
          </button>
        </form>

        <div className="auth-switch">
          {mode === 'login' ? (
            <p>
              Нет аккаунта?{' '}
              <button className="link-btn" onClick={() => switchMode('register')}>
                Зарегистрироваться
              </button>
            </p>
          ) : (
            <p>
              Уже есть аккаунт?{' '}
              <button className="link-btn" onClick={() => switchMode('login')}>
                Войти
              </button>
            </p>
          )}
        </div>
      </div>
    </div>
  );
}

export default AuthPage;