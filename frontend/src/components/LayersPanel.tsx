import { useState, useCallback, useEffect } from 'react';

interface LayerInfo {
  index: number;
  download_url: string;
  data_url: string;
}

interface Props {
  layers: LayerInfo[];
  loading: boolean;
}

function LayersPanel({ layers, loading }: Props) {
  const [zoomedLayer, setZoomedLayer] = useState<LayerInfo | null>(null);

  const handleDownloadLayer = (layer: LayerInfo) => {
    window.open(layer.download_url, '_blank');
  };

  const handleDownloadAll = () => {
    layers.forEach((layer, i) => {
      setTimeout(() => {
        window.open(layer.download_url, '_blank');
      }, i * 300);
    });
  };

  const handleZoom = useCallback((layer: LayerInfo) => {
    setZoomedLayer(layer);
  }, []);

  const handleCloseZoom = useCallback(() => {
    setZoomedLayer(null);
  }, []);

  // Закрытие по Escape
  useEffect(() => {
    if (!zoomedLayer) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setZoomedLayer(null);
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [zoomedLayer]);

  if (loading) {
    return (
      <div className="card">
        <h2 className="card-title">Слои трафарета</h2>
        <div className="status-msg">⏳ Обработка изображения... Пожалуйста, подождите.</div>
      </div>
    );
  }

  if (layers.length === 0) {
    return (
      <div className="card">
        <h2 className="card-title">Слои трафарета</h2>
        <div className="status-msg">Нажмите «Создать слои», чтобы сгенерировать трафареты.</div>
      </div>
    );
  }

  return (
    <div className="card">
      <h2 className="card-title">Слои трафарета</h2>
      <div className="layers-grid">
        {layers.map((layer) => (
          <div key={layer.index} className="layer-card">
            <img
              src={layer.data_url}
              alt={`Слой ${layer.index + 1}`}
              className="layer-thumb"
              onClick={() => handleZoom(layer)}
              title="Нажмите для увеличения"
            />
            <div className="layer-label">Слой {layer.index + 1}</div>
          </div>
        ))}
      </div>

      <div className="download-all-row">
        <button className="btn btn-primary" onClick={handleDownloadAll}>
          Скачать трафареты
        </button>
      </div>

      {/* Модальное окно увеличения */}
      {zoomedLayer && (
        <div className="zoom-overlay" onClick={handleCloseZoom}>
          <div className="zoom-container" onClick={(e) => e.stopPropagation()}>
            <div className="zoom-content-row">
              {(() => {
                const idx = layers.findIndex((l) => l.index === zoomedLayer.index);
                const hasPrev = idx > 0;
                const hasNext = idx < layers.length - 1;
                return (
                  <>
                    {hasPrev && (
                      <button
                        className="zoom-nav zoom-nav-left"
                        onClick={(e) => {
                          e.stopPropagation();
                          setZoomedLayer(layers[idx - 1]);
                        }}
                        title="Предыдущий слой"
                      >
                        ‹
                      </button>
                    )}
                    <img
                      src={zoomedLayer.data_url}
                      alt={`Слой ${zoomedLayer.index + 1}`}
                      className="zoom-image"
                    />
                    {hasNext && (
                      <button
                        className="zoom-nav zoom-nav-right"
                        onClick={(e) => {
                          e.stopPropagation();
                          setZoomedLayer(layers[idx + 1]);
                        }}
                        title="Следующий слой"
                      >
                        ›
                      </button>
                    )}
                  </>
                );
              })()}
            </div>
            <div className="zoom-actions">
              <button
                className="btn btn-primary"
                onClick={() => handleDownloadLayer(zoomedLayer)}
              >
                Скачать слой
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default LayersPanel;
