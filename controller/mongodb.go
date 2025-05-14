package controller

import (
	"context"
	"database-api-public-go/utils"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDBConnectionInfo MongoDB连接信息
type MongoDBConnectionInfo struct {
	Host            string `json:"host" binding:"required"`
	Port            int    `json:"port" binding:"required"`
	Database        string `json:"database" binding:"required"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	AuthSource      string `json:"auth_source"`
	ConnectTimeoutMS int   `json:"connect_timeout_ms"`
}

// MongoDBExecuteRequest MongoDB执行请求
type MongoDBExecuteRequest struct {
	Connection MongoDBConnectionInfo `json:"connection" binding:"required"`
	Collection string                `json:"collection" binding:"required"`
	Operation  string                `json:"operation" binding:"required"`
	Filter     map[string]interface{} `json:"filter"`
	Update     map[string]interface{} `json:"update"`
	Document   map[string]interface{} `json:"document"`
	Documents  []map[string]interface{} `json:"documents"`
	Projection map[string]interface{} `json:"projection"`
	Sort       []interface{}          `json:"sort"`
	Limit      int64                  `json:"limit"`
	Skip       int64                  `json:"skip"`
	Pipeline   []map[string]interface{} `json:"pipeline"`
}

// MongoDBHandler 处理MongoDB请求
func MongoDBHandler(c *gin.Context) {
	var request MongoDBExecuteRequest

	// 绑定请求数据
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequestResponse(c, fmt.Sprintf("无效的请求数据: %v", err))
		return
	}

	// 验证操作类型
	operation := strings.ToLower(request.Operation)
	if !isValidOperation(operation) {
		utils.BadRequestResponse(c, "不支持的操作类型。支持的操作类型有: find, findone, insert, insertmany, update, updatemany, delete, deletemany, aggregate, count")
		return
	}

	// 验证操作参数
	if err := validateOperationParams(request); err != nil {
		utils.ErrorResponse(c, 1100, err.Error())
		return
	}

	// 设置默认值
	if request.Connection.AuthSource == "" {
		request.Connection.AuthSource = "admin"
	}
	if request.Connection.ConnectTimeoutMS == 0 {
		request.Connection.ConnectTimeoutMS = 30000
	}

	// 获取MongoDB客户端
	client, err := utils.MongoDBPool.GetClient(
		request.Connection.Host,
		request.Connection.Port,
		request.Connection.Database,
		request.Connection.Username,
		request.Connection.Password,
		request.Connection.AuthSource,
	)
	if err != nil {
		log.Printf("MongoDB连接错误: %v", err)
		utils.ErrorResponse(c, 1001, fmt.Sprintf("数据库连接错误: %v", err))
		return
	}

	// 获取数据库和集合
	db := client.Database(request.Connection.Database)
	
	// 检查集合是否存在
	collections, err := db.ListCollectionNames(context.Background(), bson.D{})
	if err != nil {
		utils.ErrorResponse(c, 1002, fmt.Sprintf("列出集合失败: %v", err))
		return
	}
	
	collectionExists := false
	for _, coll := range collections {
		if coll == request.Collection {
			collectionExists = true
			break
		}
	}
	
	if !collectionExists && operation != "insert" && operation != "insertmany" {
		utils.ErrorResponse(c, 1120, fmt.Sprintf("集合 '%s' 不存在", request.Collection))
		return
	}
	
	collection := db.Collection(request.Collection)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 根据操作类型执行相应的MongoDB操作
	switch operation {
	case "find":
		handleFind(ctx, c, collection, request)
	case "findone":
		handleFindOne(ctx, c, collection, request)
	case "insert":
		handleInsert(ctx, c, collection, request)
	case "insertmany":
		handleInsertMany(ctx, c, collection, request)
	case "update":
		handleUpdate(ctx, c, collection, request, false)
	case "updatemany":
		handleUpdate(ctx, c, collection, request, true)
	case "delete":
		handleDelete(ctx, c, collection, request, false)
	case "deletemany":
		handleDelete(ctx, c, collection, request, true)
	case "aggregate":
		handleAggregate(ctx, c, collection, request)
	case "count":
		handleCount(ctx, c, collection, request)
	}
}

// 验证操作类型
func isValidOperation(operation string) bool {
	validOperations := map[string]bool{
		"find": true, "findone": true, "insert": true, "insertmany": true,
		"update": true, "updatemany": true, "delete": true, "deletemany": true,
		"aggregate": true, "count": true,
	}
	return validOperations[operation]
}

// 验证操作参数
func validateOperationParams(request MongoDBExecuteRequest) error {
	operation := strings.ToLower(request.Operation)

	// 验证插入操作参数
	if operation == "insert" && request.Document == nil {
		return fmt.Errorf("插入操作缺少document字段，请提供要插入的文档")
	}
	if operation == "insertmany" && (request.Documents == nil || len(request.Documents) == 0) {
		return fmt.Errorf("批量插入操作缺少documents字段，请提供要插入的文档列表")
	}

	// 验证更新操作参数
	if (operation == "update" || operation == "updatemany") {
		if request.Filter == nil {
			return fmt.Errorf("更新操作缺少filter字段，请提供查询条件")
		}
		if request.Update == nil {
			return fmt.Errorf("更新操作缺少update字段，请提供更新内容")
		}

		// 检查更新操作是否包含操作符
		hasUpdateOperator := false
		for key := range request.Update {
			if strings.HasPrefix(key, "$") {
				hasUpdateOperator = true
				break
			}
		}
		if !hasUpdateOperator {
			return fmt.Errorf("更新操作的update字段格式不正确，应包含至少一个更新操作符如 $set, $unset 等")
		}
	}

	// 验证删除操作参数
	if (operation == "delete" || operation == "deletemany") && request.Filter == nil {
		return fmt.Errorf("删除操作缺少filter字段，请提供查询条件")
	}

	// 验证聚合操作参数
	if operation == "aggregate" && (request.Pipeline == nil || len(request.Pipeline) == 0) {
		return fmt.Errorf("聚合操作缺少pipeline字段，请提供聚合管道")
	}

	return nil
}

// 处理查询操作
func handleFind(ctx context.Context, c *gin.Context, collection *mongo.Collection, request MongoDBExecuteRequest) {
	filter := convertMapToBSON(request.Filter)
	findOptions := options.Find()

	if request.Projection != nil {
		findOptions.SetProjection(convertMapToBSON(request.Projection))
	}
	if request.Sort != nil && len(request.Sort) > 0 {
		findOptions.SetSort(convertToSort(request.Sort))
	}
	if request.Skip > 0 {
		findOptions.SetSkip(request.Skip)
	}
	if request.Limit > 0 {
		findOptions.SetLimit(request.Limit)
	}

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		utils.ErrorResponse(c, 1200, fmt.Sprintf("查询操作失败: %v", err))
		return
	}
	defer cursor.Close(ctx)

	var results []map[string]interface{}
	if err := cursor.All(ctx, &results); err != nil {
		utils.ErrorResponse(c, 1201, fmt.Sprintf("处理查询结果失败: %v", err))
		return
	}

	for i := range results {
		results[i] = convertBSONToMap(results[i])
	}

	utils.SuccessResponse(c, results)
}

// 处理单条查询操作
func handleFindOne(ctx context.Context, c *gin.Context, collection *mongo.Collection, request MongoDBExecuteRequest) {
	filter := convertMapToBSON(request.Filter)
	findOptions := options.FindOne()

	if request.Projection != nil {
		findOptions.SetProjection(convertMapToBSON(request.Projection))
	}

	var result map[string]interface{}
	err := collection.FindOne(ctx, filter, findOptions).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.SuccessResponse(c, []interface{}{})
		} else {
			utils.ErrorResponse(c, 1202, fmt.Sprintf("查询单条记录失败: %v", err))
		}
		return
	}

	utils.SuccessResponse(c, []interface{}{convertBSONToMap(result)})
}

// 处理插入操作
func handleInsert(ctx context.Context, c *gin.Context, collection *mongo.Collection, request MongoDBExecuteRequest) {
	document := convertMapToBSON(request.Document)
	
	result, err := collection.InsertOne(ctx, document)
	if err != nil {
		utils.ErrorResponse(c, 1203, fmt.Sprintf("插入文档失败: %v", err))
		return
	}

	utils.SuccessResponse(c, map[string]interface{}{
		"inserted_id": result.InsertedID,
	})
}

// 处理批量插入操作
func handleInsertMany(ctx context.Context, c *gin.Context, collection *mongo.Collection, request MongoDBExecuteRequest) {
	var documents []interface{}
	for _, doc := range request.Documents {
		documents = append(documents, convertMapToBSON(doc))
	}
	
	result, err := collection.InsertMany(ctx, documents)
	if err != nil {
		utils.ErrorResponse(c, 1204, fmt.Sprintf("批量插入文档失败: %v", err))
		return
	}

	utils.SuccessResponse(c, map[string]interface{}{
		"inserted_count": len(result.InsertedIDs),
		"inserted_ids":   result.InsertedIDs,
	})
}

// 处理更新操作
func handleUpdate(ctx context.Context, c *gin.Context, collection *mongo.Collection, request MongoDBExecuteRequest, many bool) {
	filter := convertMapToBSON(request.Filter)
	update := convertMapToBSON(request.Update)

	if many {
		result, err := collection.UpdateMany(ctx, filter, update)
		if err != nil {
			utils.ErrorResponse(c, 1205, fmt.Sprintf("批量更新文档失败: %v", err))
			return
		}
		utils.SuccessResponse(c, map[string]interface{}{
			"matched_count":  result.MatchedCount,
			"modified_count": result.ModifiedCount,
		})
	} else {
		result, err := collection.UpdateOne(ctx, filter, update)
		if err != nil {
			utils.ErrorResponse(c, 1206, fmt.Sprintf("更新文档失败: %v", err))
			return
		}
		utils.SuccessResponse(c, map[string]interface{}{
			"matched_count":  result.MatchedCount,
			"modified_count": result.ModifiedCount,
		})
	}
}

// 处理删除操作
func handleDelete(ctx context.Context, c *gin.Context, collection *mongo.Collection, request MongoDBExecuteRequest, many bool) {
	filter := convertMapToBSON(request.Filter)

	if many {
		result, err := collection.DeleteMany(ctx, filter)
		if err != nil {
			utils.ErrorResponse(c, 1207, fmt.Sprintf("批量删除文档失败: %v", err))
			return
		}
		utils.SuccessResponse(c, map[string]interface{}{
			"deleted_count": result.DeletedCount,
		})
	} else {
		result, err := collection.DeleteOne(ctx, filter)
		if err != nil {
			utils.ErrorResponse(c, 1208, fmt.Sprintf("删除文档失败: %v", err))
			return
		}
		utils.SuccessResponse(c, map[string]interface{}{
			"deleted_count": result.DeletedCount,
		})
	}
}

// 处理聚合操作
func handleAggregate(ctx context.Context, c *gin.Context, collection *mongo.Collection, request MongoDBExecuteRequest) {
	var pipeline []bson.D
	for _, stage := range request.Pipeline {
		pipeline = append(pipeline, convertMapToBSONDoc(stage))
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		utils.ErrorResponse(c, 1209, fmt.Sprintf("聚合操作失败: %v", err))
		return
	}
	defer cursor.Close(ctx)

	var results []map[string]interface{}
	if err := cursor.All(ctx, &results); err != nil {
		utils.ErrorResponse(c, 1210, fmt.Sprintf("处理聚合结果失败: %v", err))
		return
	}

	for i := range results {
		results[i] = convertBSONToMap(results[i])
	}

	utils.SuccessResponse(c, results)
}

// 处理计数操作
func handleCount(ctx context.Context, c *gin.Context, collection *mongo.Collection, request MongoDBExecuteRequest) {
	filter := convertMapToBSON(request.Filter)
	
	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		utils.ErrorResponse(c, 1211, fmt.Sprintf("计数操作失败: %v", err))
		return
	}

	utils.SuccessResponse(c, map[string]interface{}{
		"count": count,
	})
}

// 将map转换为BSON
func convertMapToBSON(m map[string]interface{}) bson.M {
	if m == nil {
		return bson.M{}
	}
	return bson.M(m)
}

// 将map转换为BSON文档
func convertMapToBSONDoc(m map[string]interface{}) bson.D {
	var doc bson.D
	for k, v := range m {
		// 如果值是map，递归转换
		if subMap, ok := v.(map[string]interface{}); ok {
			doc = append(doc, bson.E{Key: k, Value: convertMapToBSON(subMap)})
		} else {
			doc = append(doc, bson.E{Key: k, Value: v})
		}
	}
	return doc
}

// 将BSON转换为map
func convertBSONToMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	for k, v := range m {
		// 转换MongoDB的_id
		if k == "_id" {
			// 将ObjectID转换为纯十六进制字符串
			idStr := fmt.Sprintf("%v", v)
			// 检查是否是ObjectID形式
			if strings.HasPrefix(idStr, "ObjectID(") && strings.HasSuffix(idStr, ")") {
				// 提取引号内的十六进制字符串
				hexString := strings.TrimPrefix(strings.TrimSuffix(idStr, ")"), "ObjectID(\"")
				hexString = strings.Trim(hexString, "\"")
				result[k] = hexString
			} else {
				// 不是ObjectID格式，直接使用原值
				result[k] = idStr
			}
			continue
		}
		
		// 处理嵌套的map
		if subMap, ok := v.(map[string]interface{}); ok {
			result[k] = convertBSONToMap(subMap)
			continue
		}
		
		// 处理数组
		if arr, ok := v.([]interface{}); ok {
			convertedArr := make([]interface{}, len(arr))
			for i, item := range arr {
				if subMap, ok := item.(map[string]interface{}); ok {
					convertedArr[i] = convertBSONToMap(subMap)
				} else {
					convertedArr[i] = item
				}
			}
			result[k] = convertedArr
			continue
		}
		
		// 其他类型直接保留
		result[k] = v
	}
	
	return result
}

// 转换排序条件
func convertToSort(sort []interface{}) bson.D {
	var doc bson.D
	
	// 处理字符串形式的排序字段 ["field1", "-field2"]
	for _, field := range sort {
		if strField, ok := field.(string); ok {
			if strings.HasPrefix(strField, "-") {
				doc = append(doc, bson.E{Key: strings.TrimPrefix(strField, "-"), Value: -1})
			} else {
				doc = append(doc, bson.E{Key: strField, Value: 1})
			}
		} else if mapField, ok := field.(map[string]interface{}); ok {
			// 处理map形式的排序字段 [{"field": 1}, {"field2": -1}]
			for k, v := range mapField {
				if val, ok := v.(int); ok {
					doc = append(doc, bson.E{Key: k, Value: val})
				} else if val, ok := v.(float64); ok {
					doc = append(doc, bson.E{Key: k, Value: int(val)})
				}
			}
		}
	}
	
	return doc
} 