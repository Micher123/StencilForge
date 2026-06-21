package cluster

import (
	"os"
	"strings"
)

// Config содержит настройки кластера
type Config struct {
	// Mode: "main" (главный с БД + авторизацией) или "worker" (только обработка)
	Mode string
	// NodeID — уникальный идентификатор этой ноды
	NodeID string
	// MainURL — URL главной ноды (нужен worker-ам для проксирования auth)
	MainURL string
	// Peers — список URL других нод (через запятую), только для main
	Peers []string
	// ClusterToken — общий секрет для межнодового взаимодействия
	ClusterToken string
	// AdvertiseURL — URL этой ноды, который видят другие (для регистрации)
	AdvertiseURL string
}

// LoadConfig загружает конфигурацию кластера из переменных окружения
// Приоритет: STENCILFORGE_* > CLUSTER_* (без префикса)
func LoadConfig() *Config {
	// Mode (STENCILFORGE_CLUSTER_MODE или CLUSTER_MODE)
	mode := firstNonEmpty(
		os.Getenv("STENCILFORGE_CLUSTER_MODE"),
		os.Getenv("CLUSTER_MODE"),
		"main",
	)

	// NodeID (STENCILFORGE_NODE_ID или CLUSTER_NODE_ID)
	nodeID := firstNonEmpty(
		os.Getenv("STENCILFORGE_NODE_ID"),
		os.Getenv("CLUSTER_NODE_ID"),
	)
	if nodeID == "" {
		nodeID, _ = os.Hostname()
	}

	// MainURL (STENCILFORGE_MAIN_URL или CLUSTER_MAIN_URL)
	mainURL := firstNonEmpty(
		os.Getenv("STENCILFORGE_MAIN_URL"),
		os.Getenv("CLUSTER_MAIN_URL"),
		"http://localhost:8080",
	)

	// Peers (STENCILFORGE_CLUSTER_PEERS или CLUSTER_PEERS)
	var peers []string
	peersEnv := firstNonEmpty(
		os.Getenv("STENCILFORGE_CLUSTER_PEERS"),
		os.Getenv("CLUSTER_PEERS"),
	)
	if peersEnv != "" {
		for _, p := range strings.Split(peersEnv, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				peers = append(peers, p)
			}
		}
	}

	// ClusterToken (STENCILFORGE_CLUSTER_SECRET или CLUSTER_TOKEN)
	clusterToken := firstNonEmpty(
		os.Getenv("STENCILFORGE_CLUSTER_SECRET"),
		os.Getenv("CLUSTER_TOKEN"),
		"stencilforge-cluster-secret",
	)

	// AdvertiseURL (STENCILFORGE_ADVERTISE_URL или CLUSTER_ADVERTISE_URL)
	advertiseURL := firstNonEmpty(
		os.Getenv("STENCILFORGE_ADVERTISE_URL"),
		os.Getenv("CLUSTER_ADVERTISE_URL"),
	)
	if advertiseURL == "" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		advertiseURL = "http://localhost:" + port
	}

	return &Config{
		Mode:         mode,
		NodeID:       nodeID,
		MainURL:      mainURL,
		Peers:        peers,
		ClusterToken: clusterToken,
		AdvertiseURL: advertiseURL,
	}
}

// IsMain возвращает true если нода — главная
func (c *Config) IsMain() bool {
	return c.Mode == "main"
}

// IsWorker возвращает true если нода — обработчик
func (c *Config) IsWorker() bool {
	return c.Mode == "worker"
}

// firstNonEmpty возвращает первый непустой аргумент
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
