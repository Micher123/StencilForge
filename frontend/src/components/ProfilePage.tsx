import React, { useEffect, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { extractErrorMessage } from '../utils/errors';

interface User {
  id: number;
  username: string;
  email: string;
  plan: string;
  max_layers: number;
  created_at: string;
}

interface PlanDuration {
  id: string;
  name: string;
  price_rub: number;
}

interface Plan {
  id: string;
  name: string;
  max_layers: number;
  durations: PlanDuration[];
}

const ProfilePage: React.FC<{ token: string }> = ({ token }) => {
  const navigate = useNavigate();
  const [user, setUser] = useState<User | null>(null);
  const [plans, setPlans] = useState<Plan[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [selectedDurations, setSelectedDurations] = useState<Record<string, string>>({});

  // Проверка статуса платежа после редиректа
  const queryParams = new URLSearchParams(window.location.search);
  const paymentID = queryParams.get('payment_id');

  useEffect(() => {
    if (paymentID) {
      checkPayment(paymentID);
    }
  }, [paymentID]);

  const checkPayment = async (pid: string) => {
    try {
      const res = await fetch(`/api/check-payment?id=${encodeURIComponent(pid)}`, {
        headers: { 'Authorization': `Bearer ${token}` },
      });
      const data = await res.json();
      if (data.status === 'succeeded') {
        setError('Оплата прошла! Тариф обновлён.');
        window.history.replaceState({}, '', '/profile');
        fetchUser();
      } else if (data.status === 'pending') {
        setError('Оплата ещё обрабатывается. Обновите страницу через минуту.');
      }
    } catch {
      setError('Не удалось проверить статус платежа.');
    }
  };

  const fetchUser = useCallback(async () => {
    try {
      const res = await fetch('/api/me', {
        headers: { 'Authorization': `Bearer ${token}` },
      });
      if (!res.ok) {
        navigate('/auth');
        return;
      }
      const data = await res.json();
      setUser(data.user || data);
    } catch {
      setError('Ошибка загрузки профиля.');
    }
  }, [token, navigate]);

  const fetchPlans = useCallback(async () => {
    try {
      const res = await fetch('/api/plans');
      const data = await res.json();
      if (data.ok) {
        setPlans(data.plans);
        // Установить длительность по умолчанию "1m"
        const defaults: Record<string, string> = {};
        data.plans.forEach((p: Plan) => {
          if (p.durations && p.durations.length > 0) {
            defaults[p.id] = p.durations[0].id;
          }
        });
        setSelectedDurations(defaults);
      }
    } catch {
      // игнорируем
    }
  }, []);

  useEffect(() => {
    const load = async () => {
      await fetchUser();
      await fetchPlans();
      setLoading(false);
    };
    load();
  }, [fetchUser, fetchPlans]);

  const currentPlanName = plans.find(p => p.id === user?.plan)?.name || user?.plan || 'free';

  const handleUpgrade = async (planId: string) => {
    setError('');
    const duration = selectedDurations[planId] || '1m';
    try {
        const res = await fetch('/api/create-payment', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({ plan: planId, duration }),
      });
      if (!res.ok) {
        const msg = await extractErrorMessage(res);
        throw new Error(msg);
      }
      const data = await res.json();
      if (data.ok && data.payment_url) {
        window.location.href = data.payment_url;
      } else {
        throw new Error(data.error || 'Не удалось создать платёж. Попробуйте позже.');
      }
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Сетевая ошибка. Проверьте соединение.';
      setError(msg);
    }
  };

  if (loading) {
    return <div className="page-center"><div className="spinner"></div></div>;
  }

  if (!user) {
    return <div className="page-center">Пожалуйста, <a onClick={() => navigate('/auth')}>войдите</a></div>;
  }

  return (
    <div className="profile-page">
      <h1>Профиль</h1>

      {error && (
        <div className={`msg ${error.includes('прошла') ? 'msg-success' : 'msg-error'}`}>
          {error}
        </div>
      )}

      <div className="profile-card">
        <div className="profile-row">
          <span className="profile-label">Имя:</span>
          <span>{user.username}</span>
        </div>
        <div className="profile-row">
          <span className="profile-label">Email:</span>
          <span>{user.email}</span>
        </div>
        <div className="profile-row">
          <span className="profile-label">Тариф:</span>
          <span className={`plan-badge plan-${user.plan}`}>{currentPlanName}</span>
        </div>
        <div className="profile-row">
          <span className="profile-label">Макс. слоёв:</span>
          <span>{user.max_layers}</span>
        </div>
        <div className="profile-row">
          <span className="profile-label">Дата регистрации:</span>
          <span>{new Date(user.created_at).toLocaleDateString('ru-RU')}</span>
        </div>
      </div>

      <h2>Сменить тариф</h2>
      <div className="plans-grid">
        {plans.map(plan => (
          <div key={plan.id} className={`plan-card ${user.plan === plan.id ? 'plan-current' : ''}`}>
            <div className="plan-name">{plan.name}</div>
            <div className="plan-layers">до {plan.max_layers} слоёв</div>

            {plan.id !== 'free' && plan.durations && plan.durations.length > 0 && (
              <div className="duration-tabs">
                {plan.durations.map(d => (
                  <button
                    key={d.id}
                    className={`duration-tab ${selectedDurations[plan.id] === d.id ? 'duration-active' : ''}`}
                    onClick={() => setSelectedDurations(prev => ({ ...prev, [plan.id]: d.id }))}
                  >
                    {d.name}
                  </button>
                ))}
              </div>
            )}

            <div className="plan-price">
              {plan.id === 'free'
                ? 'Бесплатно'
                : `${(() => {
                    const dur = plan.durations?.find(d => d.id === selectedDurations[plan.id]);
                    return dur ? dur.price_rub : 0;
                  })()} ₽`
              }
            </div>

            {plan.id === 'free' ? (
              <div className="plan-btn-disabled">Текущий free</div>
            ) : user.plan === plan.id ? (
              <div className="plan-btn-disabled">Уже подключен</div>
            ) : (
              <button className="btn" onClick={() => handleUpgrade(plan.id)}>Купить</button>
            )}
          </div>
        ))}
      </div>

      <div className="profile-nav">
        <button className="btn btn-secondary" onClick={() => navigate('/')}>← На главную</button>
      </div>
    </div>
  );
};

export default ProfilePage;