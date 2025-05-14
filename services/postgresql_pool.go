package services

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

// PostgreSQLConnectionPool PostgreSQL连接池结构
type PostgreSQLConnectionPool struct {
	pools   map[string]*PostgreSQLPool
	mutex   sync.Mutex
	logger  *logrus.Logger
	cleanup bool
}

// PostgreSQLPool 单个连接池
type PostgreSQLPool struct {
	pool     *sql.DB
	lastUsed time.Time
}

// PostgreSQLInstance 包含PostgreSQL实例的全局单例
var PostgreSQLInstance *PostgreSQLConnectionPool
var postgresqlOnce sync.Once

// GetPostgreSQLInstance 获取PostgreSQL连接池的单例实例
func GetPostgreSQLInstance() *PostgreSQLConnectionPool {
	postgresqlOnce.Do(func() {
		logger := logrus.New()
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})

		PostgreSQLInstance = &PostgreSQLConnectionPool{
			pools:   make(map[string]*PostgreSQLPool),
			logger:  logger,
			cleanup: false,
		}

		// 启动清理协程
		go PostgreSQLInstance.cleanupIdlePools()
	})
	return PostgreSQLInstance
}

// GetConnection 获取PostgreSQL连接
func (p *PostgreSQLConnectionPool) GetConnection(host string, port int, database string, user string, password string, sslmode string, connectTimeout int) (*sql.DB, error) {
	poolKey := fmt.Sprintf("%s:%d:%s:%s", host, port, database, user)

	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 检查连接池是否存在
	if pool, exists := p.pools[poolKey]; exists {
		// 更新最后使用时间
		pool.lastUsed = time.Now()
		p.logger.Infof("从连接池获取已有连接: %s", poolKey)
		return pool.pool, nil
	}

	// 设置sslmode默认值
	if sslmode == "" {
		sslmode = "disable"
	}

	// 支持的模式：require, verify-full, verify-ca, disable
	// 检查是否使用了不支持的SSL模式
	if sslmode != "require" && sslmode != "verify-full" && sslmode != "verify-ca" && sslmode != "disable" {
		p.logger.Warnf("不支持的SSL模式: %s, 默认使用 disable", sslmode)
		sslmode = "disable"
	}

	// 设置连接超时默认值
	if connectTimeout <= 0 {
		connectTimeout = 30
	}

	// 创建数据库连接字符串
	connStr := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s connect_timeout=%d",
		host, port, database, user, password, sslmode, connectTimeout)

	// 创建数据库连接
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		p.logger.Errorf("打开PostgreSQL连接失败: %v", err)
		return nil, err
	}

	// 设置连接池参数
	db.SetMaxOpenConns(30)  // 最大连接数
	db.SetMaxIdleConns(10)  // 最大空闲连接数
	db.SetConnMaxLifetime(30 * time.Minute) // 连接最大生命周期
	db.SetConnMaxIdleTime(10 * time.Minute) // 连接最大空闲时间

	// 测试连接
	if err := db.Ping(); err != nil {
		p.logger.Errorf("PostgreSQL连接测试失败: %v", err)
		db.Close()
		return nil, err
	}

	// 将连接池保存到缓存
	p.pools[poolKey] = &PostgreSQLPool{
		pool:     db,
		lastUsed: time.Now(),
	}

	p.logger.Infof("创建新的PostgreSQL连接池: %s", poolKey)
	return db, nil
}

// ReleaseConnection 释放连接回连接池
func (p *PostgreSQLConnectionPool) ReleaseConnection(host string, port int, database string, user string) {
	poolKey := fmt.Sprintf("%s:%d:%s:%s", host, port, database, user)

	p.mutex.Lock()
	defer p.mutex.Unlock()

	if _, exists := p.pools[poolKey]; exists {
		// 更新最后使用时间
		p.pools[poolKey].lastUsed = time.Now()
		p.logger.Debugf("连接已归还到连接池: %s", poolKey)
	}
}

// ClosePool 关闭指定的连接池
func (p *PostgreSQLConnectionPool) ClosePool(host string, port int, database string, user string) error {
	poolKey := fmt.Sprintf("%s:%d:%s:%s", host, port, database, user)

	p.mutex.Lock()
	defer p.mutex.Unlock()

	if pool, exists := p.pools[poolKey]; exists {
		if err := pool.pool.Close(); err != nil {
			p.logger.Errorf("关闭PostgreSQL连接池失败: %v", err)
			return err
		}
		delete(p.pools, poolKey)
		p.logger.Infof("关闭PostgreSQL连接池: %s", poolKey)
	}

	return nil
}

// cleanupIdlePools 定期清理长时间未使用的连接池
func (p *PostgreSQLConnectionPool) cleanupIdlePools() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	p.cleanup = true

	for p.cleanup {
		<-ticker.C

		p.mutex.Lock()
		now := time.Now()
		for key, pool := range p.pools {
			// 关闭超过10分钟未使用的连接池
			if now.Sub(pool.lastUsed) > 10*time.Minute {
				p.logger.Infof("关闭闲置PostgreSQL连接池: %s", key)
				if err := pool.pool.Close(); err != nil {
					p.logger.Errorf("关闭PostgreSQL连接池失败: %v", err)
				}
				delete(p.pools, key)
			}
		}
		p.mutex.Unlock()
	}
}

// StopCleanup 停止清理协程
func (p *PostgreSQLConnectionPool) StopCleanup() {
	p.cleanup = false
}

// Close 关闭所有连接池
func (p *PostgreSQLConnectionPool) Close() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for key, pool := range p.pools {
		if err := pool.pool.Close(); err != nil {
			p.logger.Errorf("关闭PostgreSQL连接池失败: %v", err)
		} else {
			p.logger.Infof("关闭PostgreSQL连接池: %s", key)
		}
	}

	// 清空连接池映射
	p.pools = make(map[string]*PostgreSQLPool)
} 