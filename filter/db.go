package filter

import (
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"
)

// DbConnector is DB layer client
type DbConnector struct {
	Pool *redis.Pool
}

// NewDbConnector return db connector
func NewDbConnector(redisURI string) *DbConnector {
	return &DbConnector{
		Pool: newRedisPool(redisURI),
	}
}

// GetMetricDbKey returns string redis key for metric
func GetMetricDbKey(metric string) string {
	return fmt.Sprintf("moira-metric:%s", metric)
}

// GetMetricRetentionDbKey returns string redis key for metric retention
func GetMetricRetentionDbKey(metric string) string {
	return fmt.Sprintf("moira-metric-retention:%s", metric)
}

func (connector *DbConnector) getPatterns() ([]string, error) {
	c := connector.Pool.Get()
	defer c.Close()
	return redis.Strings(c.Do("SMEMBERS", "moira-pattern-list"))
}

func (connector *DbConnector) saveMetrics(buffer []*MatchedMetric) error {

	c := connector.Pool.Get()
	defer c.Close()

	for _, m := range buffer {

		metricKey := GetMetricDbKey(m.Metric)
		metricRetentionKey := GetMetricRetentionDbKey(m.Metric)

		metricValue := fmt.Sprintf("%v %v", m.Value, m.Timestamp)

		c.Send("ZADD", metricKey, m.Timestamp, metricValue)
		c.Send("SET", metricRetentionKey, m.Retention)

		for _, pattern := range m.Patterns {
			event, err := makeEvent(pattern, m.Metric)
			if err != nil {
				continue
			}
			c.Send("PUBLISH", "metric-event", event)
		}
	}
	return c.Flush()
}

func newRedisPool(redisURI string, dbID ...int) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     10,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", redisURI)
			if err != nil {
				return nil, err
			}
			if len(dbID) > 0 {
				c.Do("SELECT", dbID[0])
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}
