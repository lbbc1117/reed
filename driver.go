package reed

import (
	"strings"
	"time"

	"context"
	"fmt"
	"reflect"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// --------- Tool functions

func MapMerge[T comparable, S any](ms ...map[T]S) map[T]S {
	res := map[T]S{}
	for _, m := range ms {
		for k, v := range m {
			_, exists := res[k]
			if !exists {
				res[k] = v
			}
		}
	}
	return res
}

func StructToMap(s any) map[string]any {
	settings := map[string]any{}
	t := reflect.TypeOf(s)
	v := reflect.ValueOf(s)
	for i := 0; i < t.NumField(); i++ {
		field := strings.Split(t.Field(i).Tag.Get("json"), ",")[0]
		value := v.Field(i).Interface()
		if field == "" {
			fieldType := reflect.TypeOf(value)
			if fieldType.Kind() == reflect.Struct {
				newMap := StructToMap(value)
				for k, v := range newMap {
					settings[k] = v
				}
			}
		} else {
			settings[field] = value
		}
	}
	return settings
}

// --------- The MongoService Type

type MongoClient struct {
	Client    *mongo.Client
	DefaultDB *MongoDatabase
}

func (m *MongoClient) Database(name string) *MongoDatabase {
	return &MongoDatabase{
		DB: m.Client.Database(name),
	}
}

func (m *MongoClient) NewQueryOptions() *QueryOptions {
	return &QueryOptions{}
}

func (m *MongoClient) MergeQueryOptions(opts ...*QueryOptions) *QueryOptions {
	qOpts := m.NewQueryOptions()
	for _, co := range opts {
		if co == nil {
			continue
		}
		if co.Filters != nil {
			qOpts.Filters = co.Filters
		}
		if co.Project != nil {
			qOpts.Project = co.Project
		}
		qOpts.WithoutPagination = co.WithoutPagination
	}
	return qOpts
}

func (m *MongoClient) MergeFilters(filters ...primitive.M) primitive.M {
	var mergedMap primitive.M = make(primitive.M)
	for _, f := range filters {
		for k, v := range f {
			mergedMap[k] = v
		}
	}
	return mergedMap
}

// --------- The MongoDatabase Type

type MongoDatabase struct {
	DB *mongo.Database
}

// --------- The MongoCollection Type

type MongoCollection[T IModel] struct {
	Collection mongo.Collection
}

func Collection[T IModel]() *MongoCollection[T] {
	var M T
	return &MongoCollection[T]{
		Collection: *Client.DefaultDB.DB.Collection(M.CollectionName()),
	}
}

func (collection *MongoCollection[T]) parseFilter(filter primitive.M) (primitive.M, error) {
	for key, value := range filter {
		if key == "_id" {
			v, err := primitive.ObjectIDFromHex(value.(string))
			if err != nil {
				return nil, err
			} else {
				filter[key] = v
			}
		}
	}
	return filter, nil
}

func (collection *MongoCollection[T]) genInsertSettings(insertInfo interface{}) primitive.M {
	settings := primitive.M{}
	t := reflect.TypeOf(insertInfo)
	v := reflect.ValueOf(insertInfo)
	if t.Kind() == reflect.Map {
		// map
		for _, key := range v.MapKeys() {
			settings[key.String()] = v.MapIndex(key).Interface()
		}
	} else {
		// struct
		settings = StructToMap(insertInfo)
	}
	settings["sys_updated_at"] = time.Now().String()[0:19]
	delete(settings, "_id")
	return settings
}

// Find ----------------------------------------------------------------------

func (collection *MongoCollection[T]) FindOne(filter primitive.M, project primitive.M) (T, error) {
	var result T
	filter, err := collection.parseFilter(filter)
	if err != nil {
		return result, err
	}
	r := collection.Collection.FindOne(context.Background(), filter, options.FindOne().SetProjection(project))
	err = r.Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, nil
		}
		return result, err
	}
	return result, nil
}

func (collection *MongoCollection[T]) Find(filter primitive.M, project primitive.M) ([]T, error) {
	var result []T
	filter, err := collection.parseFilter(filter)
	if err != nil {
		return result, err
	}
	cursor, err := collection.Collection.Find(context.Background(), filter, options.Find().SetProjection(project))
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.Background(), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Create ----------------------------------------------------------------------

func (collection *MongoCollection[T]) InsertOne(document T) (*mongo.InsertOneResult, error) {
	doc := collection.genInsertSettings(document)
	return collection.Collection.InsertOne(context.Background(), doc)
}

// Update ----------------------------------------------------------------------

func (collection *MongoCollection[T]) UpdateOne(filter primitive.M, insertInfo interface{}, extraSetting primitive.M) (*mongo.UpdateResult, error) {
	filter, err := collection.parseFilter(filter)
	if err != nil {
		return nil, err
	}
	settings := collection.genInsertSettings(insertInfo)
	update := primitive.M{
		"$set": settings,
	}
	if extraSetting != nil {
		update = MapMerge(update, extraSetting)
	}
	r, err := collection.Collection.UpdateOne(context.Background(), filter, update)
	return r, err
}

func (collection *MongoCollection[T]) UpsertOne(filter primitive.M, insertInfo interface{}, extraSetting primitive.M) (*mongo.UpdateResult, error) {
	filter, err := collection.parseFilter(filter)
	if err != nil {
		return nil, err
	}
	settings := collection.genInsertSettings(insertInfo)
	options := options.Update().SetUpsert(true)
	update := primitive.M{
		"$set": settings,
	}
	if extraSetting != nil {
		update = MapMerge(update, extraSetting)
	}
	r, err := collection.Collection.UpdateOne(context.Background(), filter, update, options)
	return r, err
}

func (collection *MongoCollection[T]) UpsertMany(writeItems *[]UpsertSetting) (*mongo.BulkWriteResult, error) {
	var writeModels []mongo.WriteModel
	for _, item := range *writeItems {
		updateModel := mongo.NewUpdateOneModel().SetFilter(item.Filter).SetUpdate(item.Update).SetUpsert(true)
		writeModels = append(writeModels, updateModel)
	}
	res, err := collection.Collection.BulkWrite(context.Background(), writeModels)
	return res, err
}

func (collection *MongoCollection[T]) FindOneAndUpdate(filter primitive.M, settings interface{}, extraSetting primitive.M) (T, error) {
	var result T
	filter, err := collection.parseFilter(filter)
	if err != nil {
		return result, err
	}
	_settings := collection.genInsertSettings(settings)
	update := primitive.M{
		"$set": _settings,
	}
	if extraSetting != nil {
		update = MapMerge(update, extraSetting)
	}
	r := collection.Collection.FindOneAndUpdate(context.Background(), filter, update)
	r.Decode(&result)
	return result, nil
}

// Delete ----------------------------------------------------------------------

func (collection *MongoCollection[T]) Delete(oid string) (*mongo.DeleteResult, error) {
	filter, err := collection.parseFilter(bson.M{"_id": oid})
	if err != nil {
		return nil, err
	}
	r, err := collection.Collection.DeleteOne(context.Background(), filter)
	return r, err
}

func (collection *MongoCollection[T]) DeleteMany(oidlist []string) (*mongo.DeleteResult, error) {
	var ids []primitive.ObjectID
	for _, oid := range oidlist {
		_id, err := primitive.ObjectIDFromHex(oid)
		if err != nil {
			return nil, err
		}
		ids = append(ids, _id)
	}
	filter := bson.M{"_id": bson.M{"$in": ids}}
	r, err := collection.Collection.DeleteMany(context.Background(), filter)
	return r, err
}

// Aggregation ----------------------------------------------------------------------

func (collection *MongoCollection[T]) Aggregate(pipeline []bson.M, extraOptions ...*QueryOptions) ([]bson.M, error) {
	cursor, err := collection.Collection.Aggregate(context.Background(), pipeline)
	if err != nil {
		return nil, err
	}
	result := []bson.M{}
	if err = cursor.All(context.Background(), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// --------- QueryOptions

type QueryOptions struct {
	Filters           primitive.M
	Project           map[string]int
	WithoutPagination bool
}

// --------- Bulk upsert settings

type UpsertSetting struct {
	Filter primitive.M
	Update primitive.M
}

// --------- Initialize singleton

func NewMongoClient(url string, dbname string) *MongoClient {
	options := options.Client().ApplyURI(url)
	client, err := mongo.Connect(context.Background(), options)
	if err != nil {
		panic(err)
	}
	db := &MongoDatabase{DB: client.Database(dbname)}
	fmt.Println("Mongo Client Initialized")
	return &MongoClient{Client: client, DefaultDB: db}
}

var Client *MongoClient
