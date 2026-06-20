import React, { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';

interface PlanInfo {
  id: string;
  name: string;
  max_layers: number;
  durations: { id: string; name: string; price_rub: number }[];
}

const defaultPlans: PlanInfo[] = [
  {
    id: 'free', name: 'Free', max_layers: 3,
    durations: [],
  },
  {
    id: 'pro', name: 'Pro', max_layers: 10,
    durations: [
      { id: '1m', name: '1 месяц', price_rub: 299 },
      { id: '3m', name: '3 месяца', price_rub: 799 },
      { id: '12m', name: '12 месяцев', price_rub: 2999 },
    ],
  },
  {
    id: 'ultima', name: 'Ultima', max_layers: 16,
    durations: [
      { id: '1m', name: '1 месяц', price_rub: 499 },
      { id: '3m', name: '3 месяца', price_rub: 1099 },
      { id: '12m', name: '12 месяцев', price_rub: 3999 },
    ],
  },
];

// Хук для отслеживания видимости элемента
function useInView(threshold = 0.15) {
  const ref = useRef<HTMLDivElement>(null);
  const [inView, setInView] = useState(false);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const observer = new IntersectionObserver(
      ([entry]) => {
        setInView(entry.isIntersecting);
      },
      { threshold }
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [threshold]);

  return { ref, inView };
}

// Оборачивает блок с анимацией fade-in/out
const AnimateBlock: React.FC<{ children: React.ReactNode; className?: string }> = ({
  children,
  className = '',
}) => {
  const { ref, inView } = useInView(0.12);
  return (
    <div ref={ref} className={`animate-block ${inView ? 'visible' : ''} ${className}`}>
      {children}
    </div>
  );
};

// ---------- Компонент ----------
const WelcomePage: React.FC = () => {
  const [plans, setPlans] = useState<PlanInfo[]>(defaultPlans);

  useEffect(() => {
    fetch('/api/plans')
      .then(res => res.json())
      .then(data => {
        if (data.ok && data.plans) setPlans(data.plans);
      })
      .catch(() => {
        // Оставляем планы по умолчанию
      });
  }, []);

  return (
    <div className="welcome-page">
      {/* Hero */}
      <AnimateBlock className="welcome-hero">
        <h1>StencilForge</h1>
        <p className="welcome-hero-sub">
          Превратите любое изображение в набор трафаретных слоёв для печати, вырезания и нанесения краски
        </p>
        <Link to="/auth" className="btn btn-lg">
          Начать бесплатно
        </Link>
      </AnimateBlock>

      {/* Как это работает */}
      <AnimateBlock>
        <section className="welcome-section">
          <h2>Как это работает</h2>
          <div className="steps-grid">
            <div className="step-card">
              <div className="step-num">1</div>
              <h3>Загрузите изображение</h3>
              <p>PNG, JPG, BMP или TIFF — мы поддерживаем все популярные форматы до 50MB.</p>
            </div>
            <div className="step-card">
              <div className="step-num">2</div>
              <h3>Выберите количество слоёв</h3>
              <p>Укажите, на сколько трафаретных слоёв нужно разбить изображение.</p>
            </div>
            <div className="step-card">
              <div className="step-num">3</div>
              <h3>Получите трафареты</h3>
              <p>Алгоритм K-Means кластеризации разделит изображение на слои, готовые к печати.</p>
            </div>
            <div className="step-card">
              <div className="step-num">4</div>
              <h3>Печатайте и творите</h3>
              <p>Распечатайте каждый слой, вырежьте, приложите к поверхности и нанесите краску.</p>
            </div>
          </div>
        </section>
      </AnimateBlock>

      {/* Примеры работ */}
      <AnimateBlock>
        <section className="welcome-section">
          <h2>Примеры работ</h2>

          {/* Линия 1: картинка → трафареты */}
          <h3 className="demo-line-title">Разложение на трафаретные слои</h3>
          <div className="demo-line">
            <div className="demo-item">
              <img src="/examples/layer0.svg" alt="Исходное изображение" className="demo-img" />
              <span className="demo-label">Исходник</span>
            </div>
            <span className="demo-arrow">→</span>
            <div className="demo-item">
              <img src="/examples/layer1.svg" alt="Слой 1" className="demo-img" />
              <span className="demo-label">Слой 1</span>
            </div>
            <span className="demo-arrow">→</span>
            <div className="demo-item">
              <img src="/examples/layer2.svg" alt="Слой 2" className="demo-img" />
              <span className="demo-label">Слой 2</span>
            </div>
            <span className="demo-arrow">→</span>
            <div className="demo-item">
              <img src="/examples/layer3.svg" alt="Слой 3" className="demo-img" />
              <span className="demo-label">Слой 3</span>
            </div>
          </div>

          {/* Линия 2: трафареты → картинка */}
          <h3 className="demo-line-title">Наложение трафаретов</h3>
          <div className="demo-line">
            <div className="demo-item">
              <img src="/examples/layer3.svg" alt="Слой 3" className="demo-img" />
              <span className="demo-label">Слой 3</span>
            </div>
            <span className="demo-arrow">→</span>
            <div className="demo-item">
              <img src="/examples/layer2.svg" alt="Слой 2" className="demo-img" />
              <span className="demo-label">Слой 2</span>
            </div>
            <span className="demo-arrow">→</span>
            <div className="demo-item">
              <img src="/examples/layer1.svg" alt="Слой 1" className="demo-img" />
              <span className="demo-label">Слой 1</span>
            </div>
            <span className="demo-arrow">→</span>
            <div className="demo-item">
              <img src="/examples/layer0.svg" alt="Исходник" className="demo-img" />
              <span className="demo-label">Исходник</span>
            </div>
          </div>

          <p className="examples-hint">
            Каждый слой — это отдельный трафарет. Накладывая их последовательно, вы воссоздаёте исходное изображение.
          </p>
        </section>
      </AnimateBlock>

      {/* Тарифы */}
      <AnimateBlock>
        <section className="welcome-section">
          <h2>Тарифы</h2>
          <div className="plans-grid welcome-plans">
            {plans.map(plan => {
              const isFree = plan.id === 'free';
              const minPrice = isFree ? 0 : Math.min(...plan.durations.map(d => d.price_rub));
              return (
                <div key={plan.id} className="plan-card welcome-plan-card">
                  <div className="plan-name">{plan.name}</div>
                  <div className="plan-price">
                    {isFree ? 'Бесплатно' : `от ${minPrice} ₽`}
                  </div>
                  <div className="plan-price-sub">в месяц</div>
                  <ul className="plan-features">
                    <li className="feature-available">
                      <span className="feature-icon feature-yes">✓</span>
                      До {plan.max_layers} слоёв
                    </li>
                    <li className="feature-available">
                      <span className="feature-icon feature-yes">✓</span>
                      PNG / SVG слои
                    </li>
                    <li className="feature-available">
                      <span className="feature-icon feature-yes">✓</span>
                      Скачивание всех слоёв
                    </li>
                    <li className={plan.id !== 'free' ? 'feature-available' : 'feature-unavailable'}>
                      <span className={`feature-icon ${plan.id !== 'free' ? 'feature-yes' : 'feature-no'}`}>
                        {plan.id !== 'free' ? '✓' : '✗'}
                      </span>
                      Приоритетная обработка
                    </li>
                    <li className={plan.id === 'ultima' ? 'feature-available' : 'feature-unavailable'}>
                      <span className={`feature-icon ${plan.id === 'ultima' ? 'feature-yes' : 'feature-no'}`}>
                        {plan.id === 'ultima' ? '✓' : '✗'}
                      </span>
                      Максимальное качество
                    </li>
                  </ul>
                  {plan.durations.length > 0 && (
                    <div className="plan-durations">
                      {plan.durations.map(d => (
                        <span key={d.id} className="plan-dur-item">
                          {d.name}: {d.price_rub} ₽
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
          <div className="welcome-cta">
            <Link to="/auth" className="btn btn-lg">
              Зарегистрироваться и начать
            </Link>
          </div>
        </section>
      </AnimateBlock>

      {/* Технологии */}
      <AnimateBlock>
        <section className="welcome-section welcome-tech">
          <h2>Технологии</h2>
          <p className="tech-desc">
            StencilForge использует алгоритм K-Means кластеризации для разделения изображения
            на цветовые группы. Каждая группа становится отдельным трафаретным слоем.
            После кластеризации применяется сглаживание и SVG-трассировка для получения
            чётких контуров, пригодных для печати и вырезания.
          </p>
          <div className="tech-tags">
            <span className="tech-tag">K-Means</span>
            <span className="tech-tag">Octree Quantization</span>
            <span className="tech-tag">SVG Tracing</span>
            <span className="tech-tag">Go + React</span>
          </div>
        </section>
      </AnimateBlock>
    </div>
  );
};

export default WelcomePage;