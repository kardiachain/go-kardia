package types


const (
	LevelDB = iota
	MongoDB
)

type DBInfo interface {
	GetInfo() []interface{}
	GetType() int
	Name() string
}

type MongoDBInfo struct {
	URI string
	DatabaseName string
	Drop bool
}

type LDBInfo struct {
	ChainData string
	DbCaches int
	DbHandles int
}

func NewMongoDBInfo(uri, databaseName string, drop bool) *MongoDBInfo {
	return &MongoDBInfo{
		URI: uri,
		DatabaseName: databaseName,
		Drop: drop,
	}
}

func (db *MongoDBInfo) GetInfo() []interface{} {
	result := make([]interface{}, 3)
	result[0] = db.URI
	result[1] = db.DatabaseName
	result[2] = db.Drop
	return result
}

func (db *MongoDBInfo) GetType() int {
	return MongoDB
}

func (db *MongoDBInfo) Name() string {
	return "mongoDB"
}

func NewLDBInfo(chainData string, dbCaches, dbHandles int) *LDBInfo {
	return &LDBInfo{
		ChainData: chainData,
		DbCaches: dbCaches,
		DbHandles: dbHandles,
	}
}

func (db *LDBInfo) GetInfo() []interface{} {
	results := make([]interface{}, 3)
	results[0] = db.ChainData
	results[1] = db.DbCaches
	results[2] = db.DbHandles
	return results
}

func (db *LDBInfo) GetType() int {
	return LevelDB
}

func (db *LDBInfo) Name() string {
	return "levelDB"
}


