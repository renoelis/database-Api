package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/renoelis/database-api-go/models"
	"github.com/renoelis/database-api-go/services"
	"github.com/renoelis/database-api-go/utils"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ExecuteMongoDB 处理MongoDB执行请求
func ExecuteMongoDB(c *gin.Context) {
	logger := logrus.New()

	// 绑定请求数据
	var request models.MongoDBExecuteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Errorf("解析MongoDB请求失败: %v", err)
		utils.ResponseError(c, 400, 1100, fmt.Sprintf("请求参数错误: %v", err))
		return
	}

	// 验证请求字段
	if err := request.ValidateRequest(); err != nil {
		logger.Errorf("MongoDB请求验证失败: %v", err)
		utils.ResponseError(c, 400, 1101, fmt.Sprintf("请求验证失败: %v", err))
		return
	}

	// 获取MongoDB连接池实例
	mongoPool := services.GetMongoDBInstance()

	// 从连接池获取客户端
	_, db, err := mongoPool.GetClient(
		request.Connection.Host,
		request.Connection.Port,
		request.Connection.Database,
		request.Connection.Username,
		request.Connection.Password,
		request.Connection.AuthSource,
		request.Connection.ConnectTimeoutMS,
	)

	if err != nil {
		logger.Errorf("从连接池获取MongoDB客户端失败: %v", err)
		errorMsg := err.Error()
		// 提供更友好的连接错误消息
		if strings.Contains(errorMsg, "timed out") {
			errorMsg = fmt.Sprintf("连接MongoDB服务器超时，请检查主机地址和端口是否正确: %s:%d", 
				request.Connection.Host, request.Connection.Port)
		} else if strings.Contains(errorMsg, "not authorized") || strings.Contains(errorMsg, "Authentication failed") {
			errorMsg = "MongoDB认证失败，请检查用户名和密码是否正确"
		}
		utils.ResponseError(c, 500, 1102, fmt.Sprintf("数据库连接错误: %s", errorMsg))
		return
	}

	logger.Infof("成功从连接池获取MongoDB客户端: %s:%d/%s", 
		request.Connection.Host, 
		request.Connection.Port, 
		request.Connection.Database)

	// 获取集合
	ctx := context.Background()
	
	// 先检查集合是否存在
	collections, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		logger.Errorf("获取集合列表失败: %v", err)
		utils.ResponseError(c, 500, 1103, fmt.Sprintf("获取集合列表失败: %v", err))
		mongoPool.ReleaseClient(
			request.Connection.Host,
			request.Connection.Port,
			request.Connection.Database,
			request.Connection.Username,
		)
		return
	}
	
	collectionExists := false
	for _, coll := range collections {
		if coll == request.Collection {
			collectionExists = true
			break
		}
	}
	
	if !collectionExists {
		logger.Warnf("集合不存在: %s", request.Collection)
		utils.ResponseError(c, 400, 1104, fmt.Sprintf("集合 '%s' 不存在", request.Collection))
		mongoPool.ReleaseClient(
			request.Connection.Host,
			request.Connection.Port,
			request.Connection.Database,
			request.Connection.Username,
		)
		return
	}

	collection := db.Collection(request.Collection)
	operation := strings.ToLower(request.Operation)

	// 执行操作
	var result interface{}
	var operationErr error

	switch operation {
	case "find":
		result, operationErr = executeFind(ctx, collection, request)
	case "findone":
		result, operationErr = executeFindOne(ctx, collection, request)
	case "insert":
		result, operationErr = executeInsert(ctx, collection, request)
	case "insertmany":
		result, operationErr = executeInsertMany(ctx, collection, request)
	case "update":
		result, operationErr = executeUpdate(ctx, collection, request)
	case "updatemany":
		result, operationErr = executeUpdateMany(ctx, collection, request)
	case "delete":
		result, operationErr = executeDelete(ctx, collection, request)
	case "deletemany":
		result, operationErr = executeDeleteMany(ctx, collection, request)
	case "aggregate":
		result, operationErr = executeAggregate(ctx, collection, request)
	case "count":
		result, operationErr = executeCount(ctx, collection, request)
	default:
		operationErr = fmt.Errorf("不支持的操作: %s", operation)
	}

	// 释放客户端
	mongoPool.ReleaseClient(
		request.Connection.Host,
		request.Connection.Port,
		request.Connection.Database,
		request.Connection.Username,
	)
	logger.Debugf("MongoDB客户端已归还到连接池")

	// 处理操作结果
	if operationErr != nil {
		logger.Errorf("MongoDB操作失败: %v", operationErr)
		utils.ResponseError(c, 400, 1105, fmt.Sprintf("MongoDB操作失败: %v", operationErr))
		return
	}

	// 返回结果
	utils.ResponseSuccess(c, result)
}

// executeFind 执行查询操作
func executeFind(ctx context.Context, collection *mongo.Collection, request models.MongoDBExecuteRequest) ([]map[string]interface{}, error) {
	// 创建过滤条件
	filter := bson.M{}
	if request.Filter != nil {
		filter = convertMapToBSON(request.Filter)
	}

	// 创建查询选项
	findOptions := options.Find()
	
	// 设置投影
	if request.Projection != nil {
		findOptions.SetProjection(convertMapToBSON(request.Projection))
	}
	
	// 设置排序
	if request.Sort != nil && len(request.Sort) > 0 {
		sort := bson.D{}
		for _, s := range request.Sort {
			if len(s) == 2 {
				fieldName, ok1 := s[0].(string)
				direction, ok2 := s[1].(float64)
				if ok1 && ok2 {
					sort = append(sort, bson.E{Key: fieldName, Value: int(direction)})
				}
			}
		}
		if len(sort) > 0 {
			findOptions.SetSort(sort)
		}
	}
	
	// 设置跳过数量
	if request.Skip > 0 {
		findOptions.SetSkip(int64(request.Skip))
	}
	
	// 设置限制数量
	if request.Limit > 0 {
		findOptions.SetLimit(int64(request.Limit))
	}

	// 执行查询
	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// 收集结果
	var results []map[string]interface{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}

// executeFindOne 执行单条查询操作
func executeFindOne(ctx context.Context, collection *mongo.Collection, request models.MongoDBExecuteRequest) ([]map[string]interface{}, error) {
	// 创建过滤条件
	filter := bson.M{}
	if request.Filter != nil {
		filter = convertMapToBSON(request.Filter)
	}

	// 创建查询选项
	findOptions := options.FindOne()
	
	// 设置投影
	if request.Projection != nil {
		findOptions.SetProjection(convertMapToBSON(request.Projection))
	}

	// 执行查询
	result := make(map[string]interface{})
	err := collection.FindOne(ctx, filter, findOptions).Decode(&result)
	
	if err == mongo.ErrNoDocuments {
		// 未找到记录，返回空数组
		return []map[string]interface{}{}, nil
	} else if err != nil {
		return nil, err
	}

	return []map[string]interface{}{result}, nil
}

// executeInsert 执行插入操作
func executeInsert(ctx context.Context, collection *mongo.Collection, request models.MongoDBExecuteRequest) (map[string]interface{}, error) {
	if request.Document == nil {
		return nil, fmt.Errorf("缺少document字段")
	}

	// 插入文档
	result, err := collection.InsertOne(ctx, convertMapToBSON(request.Document))
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"insertedId": result.InsertedID,
		"insertedCount": 1,
	}, nil
}

// executeInsertMany 执行批量插入操作
func executeInsertMany(ctx context.Context, collection *mongo.Collection, request models.MongoDBExecuteRequest) (map[string]interface{}, error) {
	if request.Documents == nil || len(request.Documents) == 0 {
		return nil, fmt.Errorf("缺少documents字段")
	}

	// 转换文档列表
	documents := make([]interface{}, len(request.Documents))
	for i, doc := range request.Documents {
		documents[i] = convertMapToBSON(doc)
	}

	// 插入多个文档
	result, err := collection.InsertMany(ctx, documents)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"insertedIds": result.InsertedIDs,
		"insertedCount": len(result.InsertedIDs),
	}, nil
}

// executeUpdate 执行更新操作
func executeUpdate(ctx context.Context, collection *mongo.Collection, request models.MongoDBExecuteRequest) (map[string]interface{}, error) {
	if request.Filter == nil {
		return nil, fmt.Errorf("缺少filter字段")
	}
	if request.Update == nil {
		return nil, fmt.Errorf("缺少update字段")
	}

	// 创建过滤条件和更新文档
	filter := convertMapToBSON(request.Filter)
	update := convertMapToBSON(request.Update)

	// 执行更新
	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"matchedCount": result.MatchedCount,
		"modifiedCount": result.ModifiedCount,
		"upsertedId": result.UpsertedID,
	}, nil
}

// executeUpdateMany 执行批量更新操作
func executeUpdateMany(ctx context.Context, collection *mongo.Collection, request models.MongoDBExecuteRequest) (map[string]interface{}, error) {
	if request.Filter == nil {
		return nil, fmt.Errorf("缺少filter字段")
	}
	if request.Update == nil {
		return nil, fmt.Errorf("缺少update字段")
	}

	// 创建过滤条件和更新文档
	filter := convertMapToBSON(request.Filter)
	update := convertMapToBSON(request.Update)

	// 执行批量更新
	result, err := collection.UpdateMany(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"matchedCount": result.MatchedCount,
		"modifiedCount": result.ModifiedCount,
		"upsertedId": result.UpsertedID,
	}, nil
}

// executeDelete 执行删除操作
func executeDelete(ctx context.Context, collection *mongo.Collection, request models.MongoDBExecuteRequest) (map[string]interface{}, error) {
	if request.Filter == nil {
		return nil, fmt.Errorf("缺少filter字段")
	}

	// 创建过滤条件
	filter := convertMapToBSON(request.Filter)

	// 执行删除
	result, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"deletedCount": result.DeletedCount,
	}, nil
}

// executeDeleteMany 执行批量删除操作
func executeDeleteMany(ctx context.Context, collection *mongo.Collection, request models.MongoDBExecuteRequest) (map[string]interface{}, error) {
	if request.Filter == nil {
		return nil, fmt.Errorf("缺少filter字段")
	}

	// 创建过滤条件
	filter := convertMapToBSON(request.Filter)

	// 执行批量删除
	result, err := collection.DeleteMany(ctx, filter)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"deletedCount": result.DeletedCount,
	}, nil
}

// executeAggregate 执行聚合操作
func executeAggregate(ctx context.Context, collection *mongo.Collection, request models.MongoDBExecuteRequest) ([]map[string]interface{}, error) {
	if request.Pipeline == nil || len(request.Pipeline) == 0 {
		return nil, fmt.Errorf("缺少pipeline字段")
	}

	// 转换聚合管道
	pipeline := make([]bson.M, len(request.Pipeline))
	for i, stage := range request.Pipeline {
		pipeline[i] = convertMapToBSON(stage)
	}

	// 执行聚合
	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// 收集结果
	var results []map[string]interface{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}

// executeCount 执行计数操作
func executeCount(ctx context.Context, collection *mongo.Collection, request models.MongoDBExecuteRequest) (map[string]interface{}, error) {
	// 创建过滤条件
	filter := bson.M{}
	if request.Filter != nil {
		filter = convertMapToBSON(request.Filter)
	}

	// 执行计数
	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"count": count,
	}, nil
}

// convertMapToBSON 将map转换为BSON文档
func convertMapToBSON(m map[string]interface{}) bson.M {
	result := bson.M{}
	
	for k, v := range m {
		switch val := v.(type) {
		case map[string]interface{}:
			result[k] = convertMapToBSON(val)
		case []interface{}:
			result[k] = convertArrayToBSON(val)
		default:
			result[k] = val
		}
	}
	
	return result
}

// convertArrayToBSON 将数组转换为BSON数组
func convertArrayToBSON(arr []interface{}) []interface{} {
	result := make([]interface{}, len(arr))
	
	for i, v := range arr {
		switch val := v.(type) {
		case map[string]interface{}:
			result[i] = convertMapToBSON(val)
		case []interface{}:
			result[i] = convertArrayToBSON(val)
		default:
			result[i] = val
		}
	}
	
	return result
} 