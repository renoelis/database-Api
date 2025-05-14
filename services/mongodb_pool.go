package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// MongoDBConnectionPool MongoDB连接池结构
type MongoDBConnectionPool struct {
	clients map[string]*MongoDBClient
	mutex   sync.Mutex
	logger  *logrus.Logger
	cleanup bool
}

// MongoDBClient 单个客户端连接
type MongoDBClient struct {
	client   *mongo.Client
	lastUsed time.Time
}

// MongoDBInstance 包含MongoDB实例的全局单例
var MongoDBInstance *MongoDBConnectionPool
var mongodbOnce sync.Once

// GetMongoDBInstance 获取MongoDB连接池的单例实例
func GetMongoDBInstance() *MongoDBConnectionPool {
	mongodbOnce.Do(func() {
		logger := logrus.New()
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})

		MongoDBInstance = &MongoDBConnectionPool{
			clients: make(map[string]*MongoDBClient),
			logger:  logger,
			cleanup: false,
		}

		// 启动清理协程
		go MongoDBInstance.cleanupIdleClients()
	})
	return MongoDBInstance
}

// GetClient 获取MongoDB客户端
func (p *MongoDBConnectionPool) GetClient(
	host string,
	port int,
	database string,
	username string,
	password string,
	authSource string,
	connectTimeoutMS int,
) (*mongo.Client, *mongo.Database, error) {
	clientKey := fmt.Sprintf("%s:%d:%s", host, port, database)
	if username != "" {
		clientKey += ":" + username
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 检查客户端是否存在
	if client, exists := p.clients[clientKey]; exists {
		// 测试连接是否有效
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		
		err := client.client.Ping(ctx, readpref.Primary())
		if err == nil {
			// 更新最后使用时间
			client.lastUsed = time.Now()
			p.logger.Infof("从连接池获取已有连接: %s", clientKey)
			return client.client, client.client.Database(database), nil
		}
		
		// 连接无效，删除并创建新连接
		p.logger.Warnf("MongoDB连接已断开，将创建新连接: %s", clientKey)
		client.client.Disconnect(ctx)
		delete(p.clients, clientKey)
	}

	// 设置默认值
	if authSource == "" {
		authSource = "admin"
	}
	if connectTimeoutMS <= 0 {
		connectTimeoutMS = 30000
	}

	// 构建连接字符串
	connectionString := fmt.Sprintf("mongodb://%s:%d", host, port)

	// 创建客户端选项
	clientOptions := options.Client().ApplyURI(connectionString)
	
	// 设置连接超时
	clientOptions.SetConnectTimeout(time.Duration(connectTimeoutMS) * time.Millisecond)
	
	// 设置最大连接池大小
	clientOptions.SetMaxPoolSize(30)
	clientOptions.SetMinPoolSize(1)
	
	// 设置空闲超时
	clientOptions.SetMaxConnIdleTime(10 * time.Second)
	
	// 设置认证信息
	if username != "" && password != "" {
		credential := options.Credential{
			Username:   username,
			Password:   password,
			AuthSource: authSource,
		}
		clientOptions.SetAuth(credential)
	}

	// 创建客户端
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(connectTimeoutMS)*time.Millisecond)
	defer cancel()
	
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		p.logger.Errorf("连接MongoDB失败: %v", err)
		return nil, nil, err
	}

	// 测试连接
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		p.logger.Errorf("MongoDB连接测试失败: %v", err)
		client.Disconnect(ctx)
		return nil, nil, err
	}

	// 将客户端添加到缓存
	p.clients[clientKey] = &MongoDBClient{
		client:   client,
		lastUsed: time.Now(),
	}

	p.logger.Infof("创建新的MongoDB连接: %s", clientKey)
	return client, client.Database(database), nil
}

// CloseClient 关闭指定的客户端
func (p *MongoDBConnectionPool) CloseClient(host string, port int, database string, username string) error {
	clientKey := fmt.Sprintf("%s:%d:%s", host, port, database)
	if username != "" {
		clientKey += ":" + username
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	if client, exists := p.clients[clientKey]; exists {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		if err := client.client.Disconnect(ctx); err != nil {
			p.logger.Errorf("关闭MongoDB连接失败: %v", err)
			return err
		}
		
		delete(p.clients, clientKey)
		p.logger.Infof("关闭MongoDB连接: %s", clientKey)
	}

	return nil
}

// ReleaseClient 更新客户端的最后使用时间
func (p *MongoDBConnectionPool) ReleaseClient(host string, port int, database string, username string) {
	clientKey := fmt.Sprintf("%s:%d:%s", host, port, database)
	if username != "" {
		clientKey += ":" + username
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	if client, exists := p.clients[clientKey]; exists {
		client.lastUsed = time.Now()
		p.logger.Debugf("MongoDB客户端已归还到连接池: %s", clientKey)
	}
}

// cleanupIdleClients 定期清理长时间未使用的客户端
func (p *MongoDBConnectionPool) cleanupIdleClients() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	p.cleanup = true

	for p.cleanup {
		<-ticker.C

		p.mutex.Lock()
		now := time.Now()
		for key, client := range p.clients {
			// 关闭超过10分钟未使用的客户端
			if now.Sub(client.lastUsed) > 10*time.Minute {
				p.logger.Infof("关闭闲置MongoDB连接: %s", key)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := client.client.Disconnect(ctx); err != nil {
					p.logger.Errorf("关闭MongoDB连接失败: %v", err)
				}
				cancel()
				delete(p.clients, key)
			}
		}
		p.mutex.Unlock()
	}
}

// StopCleanup 停止清理协程
func (p *MongoDBConnectionPool) StopCleanup() {
	p.cleanup = false
}

// Close 关闭所有客户端
func (p *MongoDBConnectionPool) Close() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for key, client := range p.clients {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := client.client.Disconnect(ctx); err != nil {
			p.logger.Errorf("关闭MongoDB连接失败: %v", err)
		} else {
			p.logger.Infof("关闭MongoDB连接: %s", key)
		}
		cancel()
	}

	// 清空客户端映射
	p.clients = make(map[string]*MongoDBClient)
} 