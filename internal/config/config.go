package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Namespace         string
	PodLabelSelector  string
	SecretName        string
	SecretKey         string
	TypesensePort     int
	CooldownDuration  time.Duration
	SnapshotTimeout   time.Duration
	SnapshotPath      string
	MetricsAddr       string
	HealthProbeAddr   string
	LeaderElection    bool
	LeaderElectionID  string
	CooldownConfigMap string
}

func FromEnv() Config {
	return Config{
		Namespace:         getEnv("NAMESPACE", "default"),
		PodLabelSelector:  getEnv("POD_LABEL_SELECTOR", "app=typesense-global-2-sts"),
		SecretName:        getEnv("SECRET_NAME", "typesense-cluster-bootstrap-key"),
		SecretKey:         getEnv("SECRET_KEY", "typesense-api-key"),
		TypesensePort:     getEnvInt("TYPESENSE_PORT", 8108),
		CooldownDuration:  getEnvDuration("COOLDOWN_DURATION", 3*time.Minute),
		SnapshotTimeout:   getEnvDuration("SNAPSHOT_TIMEOUT", 30*time.Second),
		SnapshotPath:      getEnv("SNAPSHOT_PATH", "/var/lib/typesense/snapshots"),
		MetricsAddr:       getEnv("METRICS_ADDR", ":9090"),
		HealthProbeAddr:   getEnv("HEALTH_PROBE_ADDR", ":8081"),
		LeaderElection:    getEnvBool("LEADER_ELECTION", true),
		LeaderElectionID:  getEnv("LEADER_ELECTION_ID", "typesense-snapshot-controller"),
		CooldownConfigMap: getEnv("COOLDOWN_CONFIGMAP", "typesense-snapshot-cooldown"),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
