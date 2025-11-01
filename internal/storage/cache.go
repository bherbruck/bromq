package storage

import (
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Cache provides thread-safe in-memory caching for frequently accessed data with TTL support
type Cache struct {
	users         sync.Map // map[string]*cachedUser - keyed by username
	aclRules      sync.Map // map[uint]*cachedACLRules - keyed by mqtt_user_id
	metrics       *CacheMetrics
	ttl           time.Duration
	cleanupTicker *time.Ticker
	stopChan      chan struct{}
	stopOnce      sync.Once // Ensures stopChan is only closed once
}

// cachedUser wraps an MQTT user with expiration time
type cachedUser struct {
	user      *MQTTUser
	expiresAt time.Time
}

// cachedACLRules wraps ACL rules with expiration time
type cachedACLRules struct {
	rules     []ACLRule
	expiresAt time.Time
}

// CacheMetrics holds Prometheus metrics for cache operations
type CacheMetrics struct {
	hits       *prometheus.CounterVec
	misses     *prometheus.CounterVec
	size       *prometheus.GaugeVec
	evictions  *prometheus.CounterVec
	expirations *prometheus.CounterVec
}

// newCacheMetrics creates cache metrics with the given registry
func newCacheMetrics(reg prometheus.Registerer) *CacheMetrics {
	factory := promauto.With(reg)
	return &CacheMetrics{
		hits: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "storage_cache_hits_total",
				Help: "Total number of cache hits",
			},
			[]string{"cache_type"},
		),
		misses: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "storage_cache_misses_total",
				Help: "Total number of cache misses",
			},
			[]string{"cache_type"},
		),
		size: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "storage_cache_size",
				Help: "Current number of entries in cache",
			},
			[]string{"cache_type"},
		),
		evictions: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "storage_cache_evictions_total",
				Help: "Total number of cache evictions (manual invalidation)",
			},
			[]string{"cache_type"},
		),
		expirations: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "storage_cache_expirations_total",
				Help: "Total number of cache expirations (TTL expired)",
			},
			[]string{"cache_type"},
		),
	}
}

// NewCache creates a new cache instance with metrics and TTL support
func NewCache() *Cache {
	return NewCacheWithRegistry(prometheus.DefaultRegisterer)
}

// NewCacheWithRegistry creates a new cache instance with a custom Prometheus registry (for testing)
func NewCacheWithRegistry(reg prometheus.Registerer) *Cache {
	return NewCacheWithTTL(loadCacheTTL(), reg)
}

// NewCacheWithTTL creates a new cache instance with a specific TTL and registry (for testing)
func NewCacheWithTTL(ttl time.Duration, reg prometheus.Registerer) *Cache {
	slog.Info("Cache TTL configured", "ttl", ttl)

	c := &Cache{
		ttl:      ttl,
		stopChan: make(chan struct{}),
		metrics:  newCacheMetrics(reg),
	}

	// Start cleanup goroutine
	c.startCleanup()

	return c
}

// loadCacheTTL loads the cache TTL configuration from environment
func loadCacheTTL() time.Duration {
	ttlStr := os.Getenv("CACHE_TTL")
	if ttlStr == "" {
		return 5 * time.Minute // Default: 5 minutes
	}

	ttl, err := time.ParseDuration(ttlStr)
	if err != nil {
		slog.Warn("Invalid CACHE_TTL, using default",
			"value", ttlStr,
			"error", err,
			"default", "5m")
		return 5 * time.Minute
	}

	// Enforce reasonable limits (1 minute to 1 hour)
	if ttl < 1*time.Minute {
		slog.Warn("CACHE_TTL too low, using minimum",
			"value", ttl,
			"minimum", "1m")
		return 1 * time.Minute
	}
	if ttl > 1*time.Hour {
		slog.Warn("CACHE_TTL too high, using maximum",
			"value", ttl,
			"maximum", "1h")
		return 1 * time.Hour
	}

	return ttl
}

// startCleanup starts a background goroutine to clean up expired cache entries
func (c *Cache) startCleanup() {
	// Run cleanup every minute
	c.cleanupTicker = time.NewTicker(1 * time.Minute)

	go func() {
		for {
			select {
			case <-c.cleanupTicker.C:
				c.cleanupExpired()
			case <-c.stopChan:
				c.cleanupTicker.Stop()
				return
			}
		}
	}()
}

// Stop stops the cache cleanup goroutine (safe to call multiple times)
func (c *Cache) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopChan)
	})
}

// cleanupExpired removes expired entries from the cache
func (c *Cache) cleanupExpired() {
	now := time.Now()
	userExpired := 0
	aclExpired := 0

	// Clean up expired MQTT users
	c.users.Range(func(key, value interface{}) bool {
		cached := value.(*cachedUser)
		if now.After(cached.expiresAt) {
			c.users.Delete(key)
			userExpired++
		}
		return true
	})

	// Clean up expired ACL rules
	c.aclRules.Range(func(key, value interface{}) bool {
		cached := value.(*cachedACLRules)
		if now.After(cached.expiresAt) {
			c.aclRules.Delete(key)
			aclExpired++
		}
		return true
	})

	// Update metrics
	if userExpired > 0 {
		c.metrics.expirations.WithLabelValues("mqtt_user").Add(float64(userExpired))
		c.updateUserCacheSize()
		slog.Debug("Cache cleanup removed expired MQTT users", "count", userExpired)
	}
	if aclExpired > 0 {
		c.metrics.expirations.WithLabelValues("acl_rules").Add(float64(aclExpired))
		c.updateACLCacheSize()
		slog.Debug("Cache cleanup removed expired ACL rules", "count", aclExpired)
	}
}

// GetMQTTUser retrieves a cached MQTT user by username
func (c *Cache) GetMQTTUser(username string) (*MQTTUser, bool) {
	val, ok := c.users.Load(username)
	if !ok {
		c.metrics.misses.WithLabelValues("mqtt_user").Inc()
		return nil, false
	}

	cached := val.(*cachedUser)

	// Check if expired
	if time.Now().After(cached.expiresAt) {
		c.users.Delete(username)
		c.metrics.expirations.WithLabelValues("mqtt_user").Inc()
		c.metrics.misses.WithLabelValues("mqtt_user").Inc()
		c.updateUserCacheSize()
		return nil, false
	}

	c.metrics.hits.WithLabelValues("mqtt_user").Inc()
	return cached.user, true
}

// SetMQTTUser caches an MQTT user with TTL
func (c *Cache) SetMQTTUser(username string, user *MQTTUser) {
	cached := &cachedUser{
		user:      user,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.users.Store(username, cached)
	c.updateUserCacheSize()
}

// DeleteMQTTUser removes an MQTT user from cache
func (c *Cache) DeleteMQTTUser(username string) {
	c.users.Delete(username)
	c.metrics.evictions.WithLabelValues("mqtt_user").Inc()
	c.updateUserCacheSize()
}

// GetACLRules retrieves cached ACL rules for a user
func (c *Cache) GetACLRules(mqttUserID uint) ([]ACLRule, bool) {
	val, ok := c.aclRules.Load(mqttUserID)
	if !ok {
		c.metrics.misses.WithLabelValues("acl_rules").Inc()
		return nil, false
	}

	cached := val.(*cachedACLRules)

	// Check if expired
	if time.Now().After(cached.expiresAt) {
		c.aclRules.Delete(mqttUserID)
		c.metrics.expirations.WithLabelValues("acl_rules").Inc()
		c.metrics.misses.WithLabelValues("acl_rules").Inc()
		c.updateACLCacheSize()
		return nil, false
	}

	c.metrics.hits.WithLabelValues("acl_rules").Inc()
	return cached.rules, true
}

// SetACLRules caches ACL rules for a user with TTL
func (c *Cache) SetACLRules(mqttUserID uint, rules []ACLRule) {
	cached := &cachedACLRules{
		rules:     rules,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.aclRules.Store(mqttUserID, cached)
	c.updateACLCacheSize()
}

// DeleteACLRules removes cached ACL rules for a user
func (c *Cache) DeleteACLRules(mqttUserID uint) {
	c.aclRules.Delete(mqttUserID)
	c.metrics.evictions.WithLabelValues("acl_rules").Inc()
	c.updateACLCacheSize()
}

// InvalidateAllACLRules clears all cached ACL rules (used when any ACL rule changes)
func (c *Cache) InvalidateAllACLRules() {
	c.aclRules = sync.Map{}
	c.metrics.size.WithLabelValues("acl_rules").Set(0)
}

// updateUserCacheSize updates the user cache size metric
func (c *Cache) updateUserCacheSize() {
	count := 0
	c.users.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	c.metrics.size.WithLabelValues("mqtt_user").Set(float64(count))
}

// updateACLCacheSize updates the ACL cache size metric
func (c *Cache) updateACLCacheSize() {
	count := 0
	c.aclRules.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	c.metrics.size.WithLabelValues("acl_rules").Set(float64(count))
}

// WarmMQTTUsers pre-loads all MQTT users into cache
func (c *Cache) WarmMQTTUsers(users []MQTTUser) {
	for i := range users {
		c.SetMQTTUser(users[i].Username, &users[i])
	}
}

// WarmACLRules pre-loads all ACL rules into cache (grouped by user)
func (c *Cache) WarmACLRules(rules []ACLRule) {
	// Group rules by mqtt_user_id
	rulesByUser := make(map[uint][]ACLRule)
	for _, rule := range rules {
		rulesByUser[rule.MQTTUserID] = append(rulesByUser[rule.MQTTUserID], rule)
	}

	// Store grouped rules in cache
	for userID, userRules := range rulesByUser {
		c.SetACLRules(userID, userRules)
	}
}
