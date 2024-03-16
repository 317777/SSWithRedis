package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var rdb *redis.Client
var db *gorm.DB
const dsn = "root:123456@tcp(127.0.0.1:3306)/db01?charset=utf8mb4&parseTime=True&loc=Local"
var router *gin.Engine



type student struct {
	Id uint
	Name string
	Age uint
}

type ser interface {
	UnmarshalBinary (data []byte) (err error)
	MarshalBinary () (data []byte, err error)
}

func (stu *student) MarshalBinary() (data []byte, err error) {
	return json.Marshal(stu)
}

func (stu *student) UnmarshalBinary(data []byte) (err error) {
	return json.Unmarshal(data, stu)
}


var _ ser = &student{}


func init() {
    rdb = redis.NewClient(&redis.Options{
        Addr:     "localhost:6379",
        Password: "", // 没有密码，默认值
        DB:       0,  // 默认DB 0
		// PoolSize: 100,//连接池大小
    })
	pong, err := rdb.Ping(context.Background()).Result()
	if err != nil {
		fmt.Println("redis connect failed", err)
		return
	}
	fmt.Println("redis Ping: ", pong)


	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
	})
	if err != nil {
		panic("failed to connect database")
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(100)
	fmt.Println("mysql connect success")

	router = gin.Default()
	router.GET("/student/:id", getStudent)
	router.POST("/student", updateStudent)
	router.DELETE("/student/:id", deleteStudent)
	//对照组，从数据库获取
	router.GET("/dbStudent/:id", getStuFromDb)


	router.Run(":8089")
	fmt.Println("server start at localhost:8089")
}

func main() {
}

func getFromRedis(key string) *student{
	//从redis中获取数据
	prefix := "student:"
	redisKey := prefix + key
	student := &student{}
	err := rdb.Get(context.Background(), redisKey).Scan(student)
	if err == redis.Nil {
		fmt.Println("key不存在,从数据库获取")
		var dberr error
		student, dberr = getFromDb(key)
		if dberr != nil {
			fmt.Println("数据库不存在该id, 缓存nil", dberr)
			rdb.Set(context.Background(), redisKey, nil, 100 * time.Second)
		} else {
			fmt.Print("从数据库获取student: ", student)
			rdb.Set(context.Background(), redisKey, student, 0)
		}
	} else {
		// fmt.Println("redis存在缓存数据")
		// fmt.Println(student)
	}
	return student
}

func getFromDb(key string) (stu *student, err error) {
	//从数据库中获取数据
	if err := db.First(&stu, key).Error; err != nil {
		fmt.Println("数据库查询失败", err)
		return &student{}, err
	}
	return stu, nil
}

func getStudent(c *gin.Context) {
	id := c.Param("id")
	student := getFromRedis(id)
	if student.Id == 0 {
		c.JSON(404, gin.H{"error": "学生未找到"})
	}else{
		c.JSON(200, student)
	}
}

func getStuFromDb(c *gin.Context){
	id := c.Param("id")
	student, _ := getFromDb(id)
	if student.Id == 0 {
		c.JSON(404, gin.H{"error": "学生未找到"})
	}else{
		c.JSON(200, student)
	}
}


func updateStudent(c *gin.Context) {
	student := &student{}
	err := c.BindJSON(student)
	if err != nil {
		c.JSON(400, gin.H{"error": "参数错误"})
		return
	}
	//更新数据库
	if err := db.Save(student).Error; err != nil {
		c.JSON(500, gin.H{"error": "更新数据库失败"})
		return
	}
	//删除缓存
	prefix := "student:"
	redisKey := prefix + fmt.Sprint(student.Id)
	rdb.Del(context.Background(), redisKey)
	// //更新缓存
	// prefix := "student:"
	// redisKey := prefix + fmt.Sprint(student.Id)
	// rdb.Set(context.Background(), redisKey, student, 0)
	// c.JSON(200, student)
}


func deleteStudent(c *gin.Context){
	id := c.Param("id")
	//删除数据库
	if err := db.Delete(&student{}, id).Error; err != nil {
		c.JSON(500, gin.H{"error": "删除数据库失败"})
		return
	}
	//删除缓存
	prefix := "student:"
	redisKey := prefix + id
	rdb.Del(context.Background(), redisKey)
	c.JSON(200, gin.H{"message": "删除成功"})
}
