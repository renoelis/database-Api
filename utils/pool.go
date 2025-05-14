package utils

import (
	"context"
	"database-api-public-go/config"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// PostgreSQLPoolManager PostgreSQL连接池管理器
type PostgreSQLPoolManager struct {
	pools     sync.Map // 连接池映射，键为连接字符串，值为连接池和最后使用时间
	mu        sync.Mutex
	closeChan chan struct{} // 用于关闭清理协程的通道
}

// MongoDBPoolManager MongoDB连接池管理器
type MongoDBPoolManager struct {
	clients   sync.Map // 客户端映射，键为URI，值为客户端和最后使用时间
	mu        sync.Mutex
	closeChan chan struct{} // 用于关闭清理协程的通道
}

// PoolInfo 连接池信息
type PoolInfo struct {
	Pool     *pgxpool.Pool
	LastUsed time.Time
}

// ClientInfo 客户端信息
type ClientInfo struct {
	Client   *mongo.Client
	LastUsed time.Time
}

// PostgreSQLPool 全局PostgreSQL连接池管理器实例
var PostgreSQLPool = initPostgreSQLPool()

// MongoDBPool 全局MongoDB连接池管理器实例
var MongoDBPool = initMongoDBPool()

// 初始化PostgreSQL连接池管理器
func initPostgreSQLPool() *PostgreSQLPoolManager {
	pm := &PostgreSQLPoolManager{
		closeChan: make(chan struct{}),
	}
	// 启动清理协程
	go pm.cleanupIdlePools()
	return pm
}

// 初始化MongoDB连接池管理器
func initMongoDBPool() *MongoDBPoolManager {
	mm := &MongoDBPoolManager{
		closeChan: make(chan struct{}),
	}
	// 启动清理协程
	go mm.cleanupIdleClients()
	return mm
}

// PostgreSQLConnectionKey PostgreSQL连接键
type PostgreSQLConnectionKey struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string
	SSLMode  string
}

// MongoDBConnectionKey MongoDB连接键
type MongoDBConnectionKey struct {
	Host         string
	Port         int
	Database     string
	Username     string
	Password     string
	AuthSource   string
}

// GetConnection 从PostgreSQL连接池获取连接
func (pm *PostgreSQLPoolManager) GetConnection(host string, port int, database, user, password, sslmode string) (*pgxpool.Pool, error) {
	// 创建连接键
	key := PostgreSQLConnectionKey{
		Host:     host,
		Port:     port,
		Database: database,
		User:     user,
		Password: password,
		SSLMode:  sslmode,
	}

	// 生成连接字符串
	connStr := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		key.Host, key.Port, key.Database, key.User, key.Password, key.SSLMode)

	// 记录连接开始时间
	startTime := time.Now()

	// 尝试从池中获取现有连接
	if poolInfo, ok := pm.pools.Load(connStr); ok {
		pi := poolInfo.(*PoolInfo)
		// 更新最后使用时间
		pi.LastUsed = time.Now()
		if config.IsDebugMode() {
			log.Printf("数据库连接：复用现有PostgreSQL连接池 耗时=%.3fms 数据库=%s",
				float64(time.Since(startTime).Microseconds())/1000.0, database)
		}
		return pi.Pool, nil
	}

	log.Printf("数据库连接：创建新的PostgreSQL连接池 数据库=%s", database)

	// 获取锁以创建新连接
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 再次检查连接是否已存在（双重检查锁定模式）
	if poolInfo, ok := pm.pools.Load(connStr); ok {
		pi := poolInfo.(*PoolInfo)
		// 更新最后使用时间
		pi.LastUsed = time.Now()
		if config.IsDebugMode() {
			log.Printf("数据库连接：复用现有PostgreSQL连接池（锁内检查） 耗时=%.3fms 数据库=%s",
				float64(time.Since(startTime).Microseconds())/1000.0, database)
		}
		return pi.Pool, nil
	}

	// 创建新的连接池
	poolCreationStart := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	log.Printf("数据库连接：正在解析连接配置 数据库=%s", database)
	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		log.Printf("数据库连接：解析连接字符串失败 耗时=%.3fms 错误=%v", 
			float64(time.Since(poolCreationStart).Microseconds())/1000.0, err)
		return nil, fmt.Errorf("解析连接字符串失败: %v", err)
	}

	// 设置连接池选项
	poolConfig.MaxConns = 20    // 最大连接数，根据需求设置为20
	poolConfig.MinConns = 1     // 最小连接数，保持至少一个活跃连接
	poolConfig.MaxConnLifetime = 30 * time.Minute
	poolConfig.MaxConnIdleTime = 5 * time.Minute  // 单个连接最大空闲时间
	poolConfig.ConnConfig.ConnectTimeout = 30 * time.Second // 连接超时时间
	
	log.Printf("数据库连接：正在创建连接池 数据库=%s", database)
	poolStart := time.Now()
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Printf("数据库连接：创建连接池失败 耗时=%.3fms 错误=%v", 
			float64(time.Since(poolStart).Microseconds())/1000.0, err)
		return nil, fmt.Errorf("创建连接池失败: %v", err)
	}

	// 将连接池和使用时间存储在map中
	poolInfo := &PoolInfo{
		Pool:     pool,
		LastUsed: time.Now(),
	}
	pm.pools.Store(connStr, poolInfo)
	
	totalTime := time.Since(startTime)
	log.Printf("数据库连接：成功创建PostgreSQL连接池 总耗时=%.3fms 解析配置=%.3fms 创建连接池=%.3fms 数据库=%s",
		float64(totalTime.Microseconds())/1000.0,
		float64(poolStart.Sub(poolCreationStart).Microseconds())/1000.0,
		float64(time.Since(poolStart).Microseconds())/1000.0,
		database)

	return pool, nil
}

// GetClient 从MongoDB连接池获取客户端
func (mm *MongoDBPoolManager) GetClient(host string, port int, database, username, password, authSource string) (*mongo.Client, error) {
	// 记录连接开始时间
	startTime := time.Now()
	
	// 生成连接URI
	var uri string
	if username != "" && password != "" {
		uri = fmt.Sprintf("mongodb://%s:%s@%s:%d/?authSource=%s",
			username, password, host, port, authSource)
	} else {
		uri = fmt.Sprintf("mongodb://%s:%d", host, port)
	}

	// 尝试从池中获取现有连接
	if clientInfo, ok := mm.clients.Load(uri); ok {
		ci := clientInfo.(*ClientInfo)
		// 更新最后使用时间
		ci.LastUsed = time.Now()
		if config.IsDebugMode() {
			log.Printf("数据库连接：复用现有MongoDB客户端 耗时=%.3fms 数据库=%s",
				float64(time.Since(startTime).Microseconds())/1000.0, database)
		}
		return ci.Client, nil
	}

	log.Printf("数据库连接：创建新的MongoDB客户端 数据库=%s", database)

	// 获取锁以创建新连接
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// 再次检查连接是否已存在（双重检查锁定模式）
	if clientInfo, ok := mm.clients.Load(uri); ok {
		ci := clientInfo.(*ClientInfo)
		// 更新最后使用时间
		ci.LastUsed = time.Now()
		if config.IsDebugMode() {
			log.Printf("数据库连接：复用现有MongoDB客户端（锁内检查） 耗时=%.3fms 数据库=%s",
				float64(time.Since(startTime).Microseconds())/1000.0, database)
		}
		return ci.Client, nil
	}

	// 创建新的客户端
	clientCreationStart := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 设置客户端选项
	log.Printf("数据库连接：正在配置MongoDB客户端选项 数据库=%s", database)
	clientOptions := options.Client().ApplyURI(uri)
	clientOptions.SetMaxPoolSize(20)   // 最大连接池大小，根据需求设置为20
	clientOptions.SetMinPoolSize(1)    // 最小连接池大小，保持至少一个活跃连接
	clientOptions.SetMaxConnIdleTime(5 * time.Minute) // 单个连接最大空闲时间
	clientOptions.SetConnectTimeout(30 * time.Second) // 连接超时时间

	connectStart := time.Now()
	log.Printf("数据库连接：正在连接MongoDB 数据库=%s", database)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Printf("数据库连接：创建MongoDB客户端失败 耗时=%.3fms 错误=%v", 
			float64(time.Since(connectStart).Microseconds())/1000.0, err)
		return nil, fmt.Errorf("创建MongoDB客户端失败: %v", err)
	}

	// 将客户端和使用时间存储在map中
	clientInfo := &ClientInfo{
		Client:   client,
		LastUsed: time.Now(),
	}
	mm.clients.Store(uri, clientInfo)
	
	totalTime := time.Since(startTime)
	log.Printf("数据库连接：成功创建MongoDB客户端 总耗时=%.3fms 配置客户端=%.3fms 连接=%.3fms 数据库=%s",
		float64(totalTime.Microseconds())/1000.0,
		float64(connectStart.Sub(clientCreationStart).Microseconds())/1000.0,
		float64(time.Since(connectStart).Microseconds())/1000.0,
		database)

	return client, nil
}

// 清理长时间未使用的PostgreSQL连接池
func (pm *PostgreSQLPoolManager) cleanupIdlePools() {
	ticker := time.NewTicker(5 * time.Minute) // 每5分钟检查一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			var keysToDelete []string

			// 找出超过10分钟未使用的连接池
			pm.pools.Range(func(key, value interface{}) bool {
				connStr := key.(string)
				poolInfo := value.(*PoolInfo)
				if now.Sub(poolInfo.LastUsed) > 10*time.Minute {
					keysToDelete = append(keysToDelete, connStr)
				}
				return true
			})

			// 关闭并删除这些连接池
			if len(keysToDelete) > 0 {
				pm.mu.Lock()
				for _, connStr := range keysToDelete {
					if value, ok := pm.pools.Load(connStr); ok {
						poolInfo := value.(*PoolInfo)
						log.Printf("关闭空闲PostgreSQL连接池: %s", connStr)
						poolInfo.Pool.Close()
						pm.pools.Delete(connStr)
					}
				}
				pm.mu.Unlock()
			}
		case <-pm.closeChan:
			return
		}
	}
}

// 清理长时间未使用的MongoDB客户端
func (mm *MongoDBPoolManager) cleanupIdleClients() {
	ticker := time.NewTicker(5 * time.Minute) // 每5分钟检查一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			var keysToDelete []string

			// 找出超过10分钟未使用的客户端
			mm.clients.Range(func(key, value interface{}) bool {
				uri := key.(string)
				clientInfo := value.(*ClientInfo)
				if now.Sub(clientInfo.LastUsed) > 10*time.Minute {
					keysToDelete = append(keysToDelete, uri)
				}
				return true
			})

			// 关闭并删除这些客户端
			if len(keysToDelete) > 0 {
				mm.mu.Lock()
				for _, uri := range keysToDelete {
					if value, ok := mm.clients.Load(uri); ok {
						clientInfo := value.(*ClientInfo)
						log.Printf("关闭空闲MongoDB客户端: %s", uri)
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						clientInfo.Client.Disconnect(ctx)
						cancel()
						mm.clients.Delete(uri)
					}
				}
				mm.mu.Unlock()
			}
		case <-mm.closeChan:
			return
		}
	}
}

// CloseAll 关闭所有PostgreSQL连接池
func (pm *PostgreSQLPoolManager) CloseAll() {
	// 通知清理协程退出
	close(pm.closeChan)

	pm.pools.Range(func(key, value interface{}) bool {
		poolInfo := value.(*PoolInfo)
		poolInfo.Pool.Close()
		return true
	})
}

// CloseAll 关闭所有MongoDB客户端
func (mm *MongoDBPoolManager) CloseAll() {
	// 通知清理协程退出
	close(mm.closeChan)

	mm.clients.Range(func(key, value interface{}) bool {
		clientInfo := value.(*ClientInfo)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		clientInfo.Client.Disconnect(ctx)
		cancel()
		return true
	})
}

// WarmupPostgreSQLPool 预热PostgreSQL连接池，可在应用启动时调用
func (pm *PostgreSQLPoolManager) WarmupPostgreSQLPool(host string, port int, database, user, password, sslmode string) bool {
	startTime := time.Now()
	log.Printf("预热连接池：开始预热PostgreSQL连接池 数据库=%s", database)
	
	pool, err := pm.GetConnection(host, port, database, user, password, sslmode)
	if err != nil {
		log.Printf("预热连接池：预热PostgreSQL连接池失败 数据库=%s 错误=%v", database, err)
		return false
	}
	
	// 后台异步进行一次简单查询，确保连接可用
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		// 执行简单查询以验证连接
		if _, err := pool.Exec(ctx, "SELECT 1"); err != nil {
			log.Printf("预热连接池：PostgreSQL连接测试查询失败 数据库=%s 错误=%v", database, err)
		}
	}()
	
	log.Printf("预热连接池：PostgreSQL连接池预热成功 耗时=%.3fms 数据库=%s", 
		float64(time.Since(startTime).Microseconds())/1000.0, database)
	return true
}

// WarmupMongoDBClient 预热MongoDB客户端，可在应用启动时调用
func (mm *MongoDBPoolManager) WarmupMongoDBClient(host string, port int, database, username, password, authSource string) bool {
	startTime := time.Now()
	log.Printf("预热连接池：开始预热MongoDB客户端 数据库=%s", database)
	
	client, err := mm.GetClient(host, port, database, username, password, authSource)
	if err != nil {
		log.Printf("预热连接池：预热MongoDB客户端失败 数据库=%s 错误=%v", database, err)
		return false
	}
	
	// 后台异步进行一次简单查询，确保连接可用
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		// 执行简单查询以验证连接
		if err := client.Database(database).RunCommand(ctx, map[string]interface{}{"ping": 1}).Err(); err != nil {
			log.Printf("预热连接池：MongoDB连接测试查询失败 数据库=%s 错误=%v", database, err)
		}
	}()
	
	log.Printf("预热连接池：MongoDB客户端预热成功 耗时=%.3fms 数据库=%s", 
		float64(time.Since(startTime).Microseconds())/1000.0, database)
	return true
} 